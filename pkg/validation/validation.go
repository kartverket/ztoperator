package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	"github.com/kartverket/ztoperator/pkg/utilities"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	istioUserVolumeAnnotation      = "sidecar.istio.io/userVolume"
	istioUserVolumeMountAnnotation = "sidecar.istio.io/userVolumeMount"

	matchOneTemplate = "{*}"
	matchAnyTemplate = "{**}"

	pathsValidation authPolicyValidatorType = iota
	podAnnotationsValidation
)

var (
	// Valid pchar from https://datatracker.ietf.org/doc/html/rfc3986#appendix-A
	// pchar = unreserved / pct-encoded / sub-delims / ":" / "@".
	validLiteral = regexp.MustCompile("^[a-zA-Z0-9-._~%!$&'()+,;:@=]+$")
)

type authPolicyValidatorType int

type AuthPolicyValidator struct {
	Type     authPolicyValidatorType
	Validate func(ctx context.Context, k8sClient client.Client, scope *state.Scope) error
}

type istioUserVolume struct {
	Name      string                    `json:"name"`
	Secret    *istioUserVolumeSecret    `json:"secret,omitempty"`
	ConfigMap *istioUserVolumeConfigMap `json:"configMap,omitempty"`
}

type istioUserVolumeSecret struct {
	SecretName string `json:"secretName"`
}

type istioUserVolumeConfigMap struct {
	Name string `json:"name"`
}

type istioUserVolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readonly"`
}

type volumeSourceKind int

const (
	volumeSourceSecret volumeSourceKind = iota
	volumeSourceConfigMap
)

func (s *volumeSourceKind) String() string {
	if *s == volumeSourceSecret {
		return "secret"
	}
	return "configMap"
}

func GetValidators() []AuthPolicyValidator {
	return []AuthPolicyValidator{
		{
			Type: pathsValidation,
			Validate: func(_ context.Context, _ client.Client, scope *state.Scope) error {
				return validatePaths(scope.AuthPolicy.GetPaths())
			},
		},
		{
			Type:     podAnnotationsValidation,
			Validate: validatePodAnnotations,
		},
	}
}

func errorMessageSuffix() string {
	return fmt.Sprintf(
		"see https://github.com/kartverket/ztoperator/blob/%s/README.md on how to do it correctly",
		config.Get().GitRef,
	)
}

func collectIstioVolumesAndMountsFromPod(
	podAnnotations map[string]string,
) ([]istioUserVolume, []istioUserVolumeMount, error) {
	var volumes []istioUserVolume
	if err := json.Unmarshal([]byte(podAnnotations[istioUserVolumeAnnotation]), &volumes); err != nil {
		return nil, nil, fmt.Errorf(
			"protected pods are missing or has incorrect pod annotation '%s', %s",
			istioUserVolumeAnnotation,
			errorMessageSuffix(),
		)
	}

	var volumeMounts []istioUserVolumeMount
	if err := json.Unmarshal([]byte(podAnnotations[istioUserVolumeMountAnnotation]), &volumeMounts); err != nil {
		return nil, nil, fmt.Errorf(
			"protected pods are missing or has incorrect pod annotation '%s', %s",
			istioUserVolumeMountAnnotation,
			errorMessageSuffix(),
		)
	}

	return volumes, volumeMounts, nil
}

func validateMountedVolumes(
	mounts []istioUserVolumeMount,
	userVolumes []istioUserVolume,
	expectedName string,
	sourceKind volumeSourceKind,
) (bool, error) {
	if len(mounts) == 0 {
		return false, nil
	}

	hasMountedCorrect := false

	for _, mount := range mounts {
		if !mount.ReadOnly {
			return false, fmt.Errorf(
				"volume mount with name %s mounting %s cannot have readonly=%t, %s",
				mount.Name,
				sourceKind.String(),
				mount.ReadOnly,
				errorMessageSuffix(),
			)
		}

		for _, volume := range userVolumes {
			if volume.Name != mount.Name {
				continue
			}

			if volume.Secret != nil && volume.ConfigMap != nil {
				return false, fmt.Errorf(
					"volume with name %s cannot create a volume from both a secret AND a configmap, %s",
					volume.Name,
					errorMessageSuffix(),
				)
			}
			correct, err := hasMountedCorrectVolume(sourceKind, volume, expectedName, mount)
			if err != nil {
				return false, err
			}
			hasMountedCorrect = *correct
		}
	}

	return hasMountedCorrect, nil
}

func hasMountedCorrectVolume(
	sourceKind volumeSourceKind,
	volume istioUserVolume,
	expectedName string,
	volumeMount istioUserVolumeMount,
) (*bool, error) {
	switch sourceKind {
	case volumeSourceSecret:
		if volume.Secret != nil {
			if volume.Secret.SecretName == expectedName {
				return utilities.Ptr(true), nil
			}
		} else {
			return nil, fmt.Errorf("volume with name %s must create a volume from a secret, %s", volumeMount.Name, errorMessageSuffix())
		}
	case volumeSourceConfigMap:
		if volume.ConfigMap != nil {
			if volume.ConfigMap.Name == expectedName {
				return utilities.Ptr(true), nil
			}
		} else {
			return nil, fmt.Errorf("volume with name %s must create a volume from a configMap, %s", volumeMount.Name, errorMessageSuffix())
		}
	}
	return nil, fmt.Errorf("encountered unknown volumeSourceKind: %s for volume %s", sourceKind.String(), volume.Name)
}

func (t authPolicyValidatorType) String() string {
	switch t {
	case pathsValidation:
		return "Path validation"
	case podAnnotationsValidation:
		return "Pod annotation"
	default:
		panic(fmt.Sprintf("unknown authPolicyValidatorType %d", t))
	}
}

func validatePaths(paths []string) error {
	for _, path := range paths {
		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("invalid path: %s; must start with '/'", path)
		}
		if strings.Contains(path, "{") {
			if err := validateNewPathSyntax(paths); err != nil {
				return err
			}
			continue
		}
		if strings.Count(path, "*") > 1 ||
			(strings.Contains(path, "*") && (path != "*" && !strings.HasPrefix(path, "*") && !strings.HasSuffix(path, "*"))) {
			return fmt.Errorf("invalid path: %s; '*' must appear only once, be at the start, end, or be '*'", path)
		}
	}
	return nil
}

func validateNewPathSyntax(paths []string) error {
	for _, path := range paths {
		containsPathTemplate := strings.Contains(path, matchOneTemplate) || strings.Contains(path, matchAnyTemplate)
		foundMatchAnyTemplate := false
		// Strip leading and trailing slashes if they exist
		path = strings.Trim(path, "/")
		globs := strings.Split(path, "/")
		for _, glob := range globs {
			// If glob is a supported path template, skip the check
			// If glob is {**}, it must be the last operator in the template
			switch {
			case glob == matchOneTemplate && !foundMatchAnyTemplate:
				continue
			case glob == matchAnyTemplate && !foundMatchAnyTemplate:
				foundMatchAnyTemplate = true
				continue
			case (glob == matchAnyTemplate || glob == matchOneTemplate) && foundMatchAnyTemplate:
				return fmt.Errorf("invalid or unsupported path %s. "+
					"{**} is not the last operator", path)
			}

			// If glob is not a supported path template and contains `{`, or `}` it is invalid.
			// Path is invalid if it contains `{` or `}` beyond a supported path template.
			if strings.ContainsAny(glob, "{}") {
				return fmt.Errorf("invalid or unsupported path %s. "+
					"Contains '{' or '}' beyond a supported path template", path)
			}

			// Validate glob is valid string literal
			// Meets Envoy's valid pchar requirements from https://datatracker.ietf.org/doc/html/rfc3986#appendix-A
			if containsPathTemplate && !validLiteral.MatchString(glob) {
				return fmt.Errorf("invalid or unsupported path %s. "+
					"Contains segment %s with invalid string literal", path, glob)
			}
		}
	}
	return nil
}

func validatePodAnnotations(ctx context.Context, k8sClient client.Client, scope *state.Scope) error {
	if !scope.AutoLoginConfig.Enabled {
		return nil
	}

	pods, getPodsErr := utilities.GetProtectedPods(ctx, k8sClient, scope.AuthPolicy)
	if getPodsErr != nil {
		return fmt.Errorf(
			"error when getting pods matching the configured labelSelector %s: %w",
			scope.AuthPolicy.Spec.Selector.MatchLabels,
			getPodsErr,
		)
	}
	if len(*pods) == 0 {
		return fmt.Errorf(
			"no pods found having the labels %s, %s",
			scope.AuthPolicy.Spec.Selector.MatchLabels,
			errorMessageSuffix(),
		)
	}

	var envoySecretVolumeMounts []istioUserVolumeMount
	var configMapVolumeMounts []istioUserVolumeMount
	var userVolumes []istioUserVolume

	youngestPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Time{},
		},
	}

	for _, pod := range *pods {
		if pod.CreationTimestamp.After(youngestPod.CreationTimestamp.Time) {
			youngestPod = pod
		}
	}

	volumes, volumeMounts, collectIstioMountsErr := collectIstioVolumesAndMountsFromPod(youngestPod.Annotations)
	if collectIstioMountsErr != nil {
		return collectIstioMountsErr
	}
	userVolumes = append(userVolumes, volumes...)
	for _, volumeMount := range volumeMounts {
		switch volumeMount.MountPath {
		case configpatch.IstioCredentialsDirectory:
			envoySecretVolumeMounts = append(envoySecretVolumeMounts, volumeMount)
		case configpatch.LuaScriptDirectory:
			configMapVolumeMounts = append(configMapVolumeMounts, volumeMount)
		}
	}

	hasMountedCorrectSecret, err := validateMountedVolumes(
		envoySecretVolumeMounts,
		userVolumes,
		scope.AutoLoginConfig.EnvoySecretName,
		volumeSourceSecret,
	)
	if err != nil {
		return err
	}

	if len(envoySecretVolumeMounts) == 0 || !hasMountedCorrectSecret {
		return fmt.Errorf(
			"secret used by envoyfilter is either not mounted in istio-proxy or is mounted incorrectly, %s",
			errorMessageSuffix(),
		)
	}

	if len(configMapVolumeMounts) == 0 {
		// If no configMap mounts are present, fall back to injecting the Lua script inline.
		scope.AutoLoginConfig.LuaScriptConfig.InjectLuaScriptAsInlineCode = true
		return nil
	}
	hasMountedCorrectConfigMap, err := validateMountedVolumes(
		configMapVolumeMounts,
		userVolumes,
		scope.AutoLoginConfig.LuaScriptConfig.LuaScriptConfigMapName,
		volumeSourceConfigMap,
	)
	if err != nil {
		return err
	}

	if !hasMountedCorrectConfigMap {
		return fmt.Errorf("configmap with lua script not correctly mounted in istio-proxy, %s", errorMessageSuffix())
	}

	return nil
}
