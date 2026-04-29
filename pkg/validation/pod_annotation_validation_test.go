package validation_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/names"
	"github.com/kartverket/ztoperator/pkg/validation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	istioCredentialsDirectory = "/etc/istio/config"
	secretName                = "my-secret"
	authPolicyName            = "test-policy"
)

var envoySecretName = func(authPolicyName string) string {
	return names.EnvoySecret(authPolicyName)
}

func TestValidatePodAnnotations_AutoLoginDisabled_ReturnsNil(t *testing.T) {
	authPolicy := BuildAuthPolicy(authPolicyName, false, map[string]string{"app": "test"}, nil)

	err := validation.ValidatePodAnnotations(&corev1.Pod{}, authPolicy)
	if err != nil {
		t.Errorf("expected nil error when autoLogin is disabled, got: %v", err)
	}
}

func TestValidatePodAnnotations_MissingVolumeAnnotation_ReturnsError(t *testing.T) {
	pod := MakePod("test-pod", map[string]string{"app": "test"}, map[string]string{
		"sidecar.istio.io/userVolumeMount": MountAnnotation("my-vol", istioCredentialsDirectory),
	}, time.Now())

	authPolicy := BuildAuthPolicy(authPolicyName, true, map[string]string{"app": "test"}, nil)

	err := validation.ValidatePodAnnotations(pod, authPolicy)
	if err == nil {
		t.Error("expected error when userVolume annotation is missing, got nil")
	} else if !strings.Contains(err.Error(), "sidecar.istio.io/userVolume") {
		t.Errorf("expected error to mention 'sidecar.istio.io/userVolume', got: %v", err)
	}
}

func TestValidatePodAnnotations_MissingMountAnnotation_ReturnsError(t *testing.T) {
	pod := MakePod("test-pod", map[string]string{"app": "test"}, map[string]string{
		"sidecar.istio.io/userVolume": VolumeAnnotation("my-vol", secretName),
	}, time.Now())

	authPolicy := BuildAuthPolicy(authPolicyName, true, map[string]string{"app": "test"}, nil)

	err := validation.ValidatePodAnnotations(pod, authPolicy)
	if err == nil {
		t.Error("expected error when userVolumeMount annotation is missing, got nil")
	} else if !strings.Contains(err.Error(), "sidecar.istio.io/userVolumeMount") {
		t.Errorf("expected error to mention 'sidecar.istio.io/userVolumeMount', got: %v", err)
	}
}

func TestValidatePodAnnotations_MalformedVolumeAnnotation_ReturnsError(t *testing.T) {
	pod := MakePod("test-pod", map[string]string{"app": "test"}, map[string]string{
		"sidecar.istio.io/userVolume":      "not-valid-json",
		"sidecar.istio.io/userVolumeMount": MountAnnotation("my-vol", istioCredentialsDirectory),
	}, time.Now())

	authPolicy := BuildAuthPolicy(authPolicyName, true, map[string]string{"app": "test"}, nil)

	err := validation.ValidatePodAnnotations(pod, authPolicy)
	if err == nil {
		t.Error("expected error for malformed userVolume annotation, got nil")
	} else if !strings.Contains(err.Error(), "sidecar.istio.io/userVolume") {
		t.Errorf("expected error to mention 'sidecar.istio.io/userVolume', got: %v", err)
	}
}

func TestValidatePodAnnotations_MalformedMountAnnotation_ReturnsError(t *testing.T) {
	pod := MakePod("test-pod", map[string]string{"app": "test"}, map[string]string{
		"sidecar.istio.io/userVolume":      VolumeAnnotation("my-vol", secretName),
		"sidecar.istio.io/userVolumeMount": "not-valid-json",
	}, time.Now())

	authPolicy := BuildAuthPolicy(authPolicyName, true, map[string]string{"app": "test"}, nil)

	err := validation.ValidatePodAnnotations(pod, authPolicy)
	if err == nil {
		t.Error("expected error for malformed userVolumeMount annotation, got nil")
	} else if !strings.Contains(err.Error(), "sidecar.istio.io/userVolumeMount") {
		t.Errorf("expected error to mention 'sidecar.istio.io/userVolumeMount', got: %v", err)
	}
}

func TestValidatePodAnnotations_WrongSecretName_ReturnsError(t *testing.T) {
	pod := MakePod("test-pod", map[string]string{"app": "test"},
		CorrectAnnotations("my-vol", "other-secret"),
		time.Now(),
	)

	authPolicy := BuildAuthPolicy(authPolicyName, true, map[string]string{"app": "test"}, nil)

	err := validation.ValidatePodAnnotations(pod, authPolicy)
	if err == nil {
		t.Error("expected error when mounted secret name does not match, got nil")
	} else if !strings.Contains(err.Error(), envoySecretName(authPolicy.Name)) {
		t.Errorf("expected error to mention %q, got: %v", envoySecretName(authPolicy.Name), err)
	}
}

func TestValidatePodAnnotations_MountAtWrongPath_ReturnsError(t *testing.T) {
	pod := MakePod("test-pod", map[string]string{"app": "test"}, map[string]string{
		"sidecar.istio.io/userVolume":      VolumeAnnotation("my-vol", secretName),
		"sidecar.istio.io/userVolumeMount": MountAnnotation("my-vol", "/some/other/path"),
	}, time.Now())

	authPolicy := BuildAuthPolicy(authPolicyName, true, map[string]string{"app": "test"}, nil)

	err := validation.ValidatePodAnnotations(pod, authPolicy)
	if err == nil {
		t.Error("expected error when mount path is wrong, got nil")
	} else if !strings.Contains(err.Error(), envoySecretName(authPolicy.Name)) {
		t.Errorf("expected error to mention %q, got: %v", envoySecretName(authPolicy.Name), err)
	}
}

func TestValidatePodAnnotations_CorrectlyMountedSecret_ReturnsNil(t *testing.T) {
	authPolicy := BuildAuthPolicy(authPolicyName, true, map[string]string{"app": "test"}, nil)
	pod := MakePod("test-pod", map[string]string{"app": "test"},
		CorrectAnnotations("my-vol", envoySecretName(authPolicy.Name)),
		time.Now(),
	)

	err := validation.ValidatePodAnnotations(pod, authPolicy)
	if err != nil {
		t.Errorf("expected nil error for correctly mounted secret, got: %v", err)
	}
}

func VolumeAnnotation(volName, secretName string) string {
	return fmt.Sprintf(`[{"name":%q,"secret":{"secretName":%q}}]`, volName, secretName)
}

func MountAnnotation(volName, mountPath string) string {
	return fmt.Sprintf(`[{"name":%q,"mountPath":%q}]`, volName, mountPath)
}

func CorrectAnnotations(volName, secretName string) map[string]string {
	return map[string]string{
		"sidecar.istio.io/userVolume":      VolumeAnnotation(volName, secretName),
		"sidecar.istio.io/userVolumeMount": MountAnnotation(volName, istioCredentialsDirectory),
	}
}

func BuildAuthPolicy(
	name string,
	autoLoginEnabled bool,
	matchLabels map[string]string,
	ignoreAuthRules *[]ztoperatorv1alpha1.RequestMatcher,
) ztoperatorv1alpha1.AuthPolicy {
	authPolicy := ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			Selector: ztoperatorv1alpha1.WorkloadSelector{
				MatchLabels: matchLabels,
			},
		},
	}
	if autoLoginEnabled {
		authPolicy.Spec.AutoLogin = &ztoperatorv1alpha1.AutoLogin{
			Enabled: true,
		}
	}
	if ignoreAuthRules != nil {
		authPolicy.Spec.IgnoreAuthRules = ignoreAuthRules
	}
	return authPolicy
}

func MakePod(name string, labels map[string]string, annotations map[string]string, creationTime time.Time) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			Labels:            labels,
			Annotations:       annotations,
			CreationTimestamp: metav1.Time{Time: creationTime},
		},
	}
}
