package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	"github.com/kartverket/ztoperator/pkg/utilities"
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

var AuthPolicyValidators = []AuthPolicyValidator{
	{
		Type: pathsValidation,
		Validate: func(ctx context.Context, k8sClient client.Client, scope state.Scope) error {
			return validatePaths(scope.AuthPolicy.GetPaths())
		},
	},
	{
		Type: podAnnotationsValidation,
		Validate: func(ctx context.Context, k8sClient client.Client, scope state.Scope) error {
			return validatePodAnnotations(ctx, k8sClient, scope)
		},
	},
}

type authPolicyValidatorType int

type AuthPolicyValidator struct {
	Type     authPolicyValidatorType
	Validate func(ctx context.Context, k8sClient client.Client, scope state.Scope) error
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

func validatePodAnnotations(ctx context.Context, k8sClient client.Client, scope state.Scope) error {
	pods, err := utilities.GetProtectedPods(ctx, k8sClient, scope.AuthPolicy)
	if err != nil {
		return fmt.Errorf("error when getting pods matching the configured labelSelector %s: %w", scope.AuthPolicy.Spec.Selector.MatchLabels, err)
	}
	if len(*pods) == 0 {
		return fmt.Errorf("no pods found having the labels %s", scope.AuthPolicy.Spec.Selector.MatchLabels)
	}

	var envoySecretVolumeMounts []istioUserVolumeMount
	var configMapVolumeMounts []istioUserVolumeMount
	var userVolumes []istioUserVolume

	if scope.AutoLoginConfig.Enabled {
		for _, pod := range *pods {
			var volumes []istioUserVolume
			userVolumeParseErr := json.Unmarshal([]byte(pod.Annotations[istioUserVolumeAnnotation]), &volumes)
			if userVolumeParseErr != nil {
				return fmt.Errorf("protected pods are missing or has incorrect pod annotation '%s'", istioUserVolumeAnnotation)
			}

			var volumeMounts []istioUserVolumeMount
			userVolumeMountParseErr := json.Unmarshal([]byte(pod.Annotations[istioUserVolumeMountAnnotation]), &volumeMounts)
			if userVolumeMountParseErr != nil {
				return fmt.Errorf("protected pods are missing or has incorrect pod annotation '%s'", istioUserVolumeMountAnnotation)
			}

			// iterer over volumemounts og hent ut de som mounter til /etc/istio/config og de som mounter til /etc/envoy/lua
			// av de som mounter til /etc/istio/config så må de finnes en userVolume med samme name og må referere til riktig secret
			// av de som mounter til /etc/envoy/lua så må de finnes en userVolume med samme name og må referere til riktig configMap

			for _, volumeMount := range volumeMounts {
				if volumeMount.MountPath == configpatch.IstioCredentialsDirectory {
					envoySecretVolumeMounts = append(envoySecretVolumeMounts, volumeMount)
				} else if volumeMount.MountPath == configpatch.LuaScriptDirectory {
					configMapVolumeMounts = append(configMapVolumeMounts, volumeMount)
				}
			}
			for _, volume := range volumes {
				userVolumes = append(userVolumes, volume)
			}
		}
	}
	hasMountedCorrectSecret := false
	for _, secretMount := range envoySecretVolumeMounts {
		if !secretMount.ReadOnly {
			return fmt.Errorf("volume mount with name %s mounting secret used by envoy filter cannot have readonly=%t", secretMount.Name, secretMount.ReadOnly)
		}
		for _, userVolume := range userVolumes {
			if userVolume.Name == secretMount.Name {
				if userVolume.Secret != nil && userVolume.ConfigMap != nil {
					return fmt.Errorf("volume with name %s cannot create a volume from both a secret AND a configmap", userVolume.Name)
				}
				if userVolume.Secret != nil {
					if userVolume.Secret.SecretName == scope.AutoLoginConfig.EnvoySecretName {
						hasMountedCorrectSecret = true
					}
				} else {
					return fmt.Errorf("volume with name %s must create a volume from a secret", secretMount.Name)
				}
			}
		}
	}
	hasMountedCorrectConfigMap := false
	for _, configMapMount := range configMapVolumeMounts {
		if !configMapMount.ReadOnly {
			return fmt.Errorf("volume mount with name %s mounting configMap used by envoy filter cannot have readonly=%t", configMapMount.Name, configMapMount.ReadOnly)
		}
		for _, userVolume := range userVolumes {
			if userVolume.Name == configMapMount.Name {
				if userVolume.Secret != nil && userVolume.ConfigMap != nil {
					return fmt.Errorf("volume with name %s cannot create a volume from both a secret AND a configmap", userVolume.Name)
				}
				if userVolume.ConfigMap != nil {
					if userVolume.ConfigMap.Name == scope.AutoLoginConfig.LuaScriptConfig.LuaScriptConfigMapName {
						hasMountedCorrectConfigMap = true
					}
				} else {
					return fmt.Errorf("volume with name %s must create a volume from a configMap", configMapMount.Name)
				}
			}
		}
	}

	if len(envoySecretVolumeMounts) == 0 || !hasMountedCorrectSecret {
		return fmt.Errorf("secret used by envoyfilter is either not mounted in istio-proxy or is mounted incorrectly")
	}

	if len(configMapVolumeMounts) == 0 {
		scope.AutoLoginConfig.LuaScriptConfig.InjectLuaScriptAsInlineCode = true
	} else {
		if !hasMountedCorrectConfigMap {
			return fmt.Errorf("configmap with lua script not correctly mounted in istio-proxy")
		}
	}
	return nil
}
