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
	Name   string                 `json:"name"`
	Secret *istioUserVolumeSecret `json:"secret,omitempty"`
}

type istioUserVolumeSecret struct {
	SecretName string `json:"secretName"`
}

type istioUserVolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readonly"`
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

func podAnnotationErrorMessageSuffix() string {
	return fmt.Sprintf(
		"see https://github.com/kartverket/ztoperator/blob/%s/README.md#-mounting-oauth-credentials-in-the-istio-sidecar on how to do it correctly",
		config.Get().GitRef,
	)
}

func collectIstioVolumesAndMountsFromPod(
	podAnnotations map[string]string,
) ([]istioUserVolume, []istioUserVolumeMount, error) {
	var volumes []istioUserVolume
	if err := json.Unmarshal([]byte(podAnnotations[istioUserVolumeAnnotation]), &volumes); err != nil {
		return nil, nil, fmt.Errorf(
			"the required annotation '%s' is either missing or its content is not properly formatted, %s",
			istioUserVolumeAnnotation,
			podAnnotationErrorMessageSuffix(),
		)
	}

	var volumeMounts []istioUserVolumeMount
	if err := json.Unmarshal([]byte(podAnnotations[istioUserVolumeMountAnnotation]), &volumeMounts); err != nil {
		return nil, nil, fmt.Errorf(
			"the required annotation '%s' is either missing or its content is not properly formatted, %s",
			istioUserVolumeMountAnnotation,
			podAnnotationErrorMessageSuffix(),
		)
	}

	return volumes, volumeMounts, nil
}

func validateSecretVolumeMount(
	mounts []istioUserVolumeMount,
	userVolumes []istioUserVolume,
	expectedSecretName string,
) bool {
	if len(mounts) == 0 {
		return false
	}

	hasMountedCorrect := false

	for _, mount := range mounts {
		for _, volume := range userVolumes {
			if volume.Name != mount.Name {
				continue
			}
			if volume.Secret != nil && volume.Secret.SecretName == expectedSecretName {
				hasMountedCorrect = true
			}
		}
	}

	return hasMountedCorrect
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
			podAnnotationErrorMessageSuffix(),
		)
	}

	var envoySecretVolumeMounts []istioUserVolumeMount
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
		if volumeMount.MountPath == configpatch.IstioCredentialsDirectory {
			envoySecretVolumeMounts = append(envoySecretVolumeMounts, volumeMount)
		}
	}

	if len(envoySecretVolumeMounts) == 0 || !validateSecretVolumeMount(
		envoySecretVolumeMounts,
		userVolumes,
		scope.AutoLoginConfig.EnvoySecretName,
	) {
		return fmt.Errorf(
			"secret with name '%s' used by OAuth-EnvoyFilter is not mounted in istio-proxy, %s",
			scope.AutoLoginConfig.EnvoySecretName,
			podAnnotationErrorMessageSuffix(),
		)
	}

	return nil
}
