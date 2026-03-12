package validation_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/validation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	istioCredentialsDirectory = "/etc/istio/config"
	envoySecretName           = "my-secret"
)

func TestValidatePodAnnotations_AutoLoginDisabled_ReturnsNil(t *testing.T) {
	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).Build()
	scope := buildScope(false, map[string]string{"app": "test"})

	err := validation.ValidatePodAnnotations(context.Background(), k8sClient, scope)
	if err != nil {
		t.Errorf("expected nil error when autoLogin is disabled, got: %v", err)
	}
}

func TestValidatePodAnnotations_NoPodsFound_ReturnsError(t *testing.T) {
	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).Build()
	scope := buildScope(true, map[string]string{"app": "test"})

	err := validation.ValidatePodAnnotations(context.Background(), k8sClient, scope)
	if err == nil {
		t.Error("expected error when no pods are found, got nil")
	} else if !strings.Contains(err.Error(), "no pods found") {
		t.Errorf("expected error to mention 'no pods found', got: %v", err)
	}
}

func TestValidatePodAnnotations_MissingVolumeAnnotation_ReturnsError(t *testing.T) {
	pod := makePod("test-pod", map[string]string{"app": "test"}, map[string]string{
		"sidecar.istio.io/userVolumeMount": mountAnnotation("my-vol", istioCredentialsDirectory),
	}, time.Now())

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(pod).Build()
	scope := buildScope(true, map[string]string{"app": "test"})

	err := validation.ValidatePodAnnotations(context.Background(), k8sClient, scope)
	if err == nil {
		t.Error("expected error when userVolume annotation is missing, got nil")
	} else if !strings.Contains(err.Error(), "sidecar.istio.io/userVolume") {
		t.Errorf("expected error to mention 'sidecar.istio.io/userVolume', got: %v", err)
	}
}

func TestValidatePodAnnotations_MissingMountAnnotation_ReturnsError(t *testing.T) {
	pod := makePod("test-pod", map[string]string{"app": "test"}, map[string]string{
		"sidecar.istio.io/userVolume": volumeAnnotation("my-vol", envoySecretName),
	}, time.Now())

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(pod).Build()
	scope := buildScope(true, map[string]string{"app": "test"})

	err := validation.ValidatePodAnnotations(context.Background(), k8sClient, scope)
	if err == nil {
		t.Error("expected error when userVolumeMount annotation is missing, got nil")
	} else if !strings.Contains(err.Error(), "sidecar.istio.io/userVolumeMount") {
		t.Errorf("expected error to mention 'sidecar.istio.io/userVolumeMount', got: %v", err)
	}
}

func TestValidatePodAnnotations_MalformedVolumeAnnotation_ReturnsError(t *testing.T) {
	pod := makePod("test-pod", map[string]string{"app": "test"}, map[string]string{
		"sidecar.istio.io/userVolume":      "not-valid-json",
		"sidecar.istio.io/userVolumeMount": mountAnnotation("my-vol", istioCredentialsDirectory),
	}, time.Now())

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(pod).Build()
	scope := buildScope(true, map[string]string{"app": "test"})

	err := validation.ValidatePodAnnotations(context.Background(), k8sClient, scope)
	if err == nil {
		t.Error("expected error for malformed userVolume annotation, got nil")
	} else if !strings.Contains(err.Error(), "sidecar.istio.io/userVolume") {
		t.Errorf("expected error to mention 'sidecar.istio.io/userVolume', got: %v", err)
	}
}

func TestValidatePodAnnotations_MalformedMountAnnotation_ReturnsError(t *testing.T) {
	pod := makePod("test-pod", map[string]string{"app": "test"}, map[string]string{
		"sidecar.istio.io/userVolume":      volumeAnnotation("my-vol", envoySecretName),
		"sidecar.istio.io/userVolumeMount": "not-valid-json",
	}, time.Now())

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(pod).Build()
	scope := buildScope(true, map[string]string{"app": "test"})

	err := validation.ValidatePodAnnotations(context.Background(), k8sClient, scope)
	if err == nil {
		t.Error("expected error for malformed userVolumeMount annotation, got nil")
	} else if !strings.Contains(err.Error(), "sidecar.istio.io/userVolumeMount") {
		t.Errorf("expected error to mention 'sidecar.istio.io/userVolumeMount', got: %v", err)
	}
}

func TestValidatePodAnnotations_WrongSecretName_ReturnsError(t *testing.T) {
	pod := makePod("test-pod", map[string]string{"app": "test"},
		correctAnnotations("my-vol", "other-secret"),
		time.Now(),
	)

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(pod).Build()
	scope := buildScope(true, map[string]string{"app": "test"})

	err := validation.ValidatePodAnnotations(context.Background(), k8sClient, scope)
	if err == nil {
		t.Error("expected error when mounted secret name does not match, got nil")
	} else if !strings.Contains(err.Error(), envoySecretName) {
		t.Errorf("expected error to mention %q, got: %v", envoySecretName, err)
	}
}

func TestValidatePodAnnotations_MountAtWrongPath_ReturnsError(t *testing.T) {
	pod := makePod("test-pod", map[string]string{"app": "test"}, map[string]string{
		"sidecar.istio.io/userVolume":      volumeAnnotation("my-vol", envoySecretName),
		"sidecar.istio.io/userVolumeMount": mountAnnotation("my-vol", "/some/other/path"),
	}, time.Now())

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(pod).Build()
	scope := buildScope(true, map[string]string{"app": "test"})

	err := validation.ValidatePodAnnotations(context.Background(), k8sClient, scope)
	if err == nil {
		t.Error("expected error when mount path is wrong, got nil")
	} else if !strings.Contains(err.Error(), envoySecretName) {
		t.Errorf("expected error to mention %q, got: %v", envoySecretName, err)
	}
}

func TestValidatePodAnnotations_CorrectlyMountedSecret_ReturnsNil(t *testing.T) {
	pod := makePod("test-pod", map[string]string{"app": "test"},
		correctAnnotations("my-vol", envoySecretName),
		time.Now(),
	)

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(pod).Build()
	scope := buildScope(true, map[string]string{"app": "test"})

	err := validation.ValidatePodAnnotations(context.Background(), k8sClient, scope)
	if err != nil {
		t.Errorf("expected nil error for correctly mounted secret, got: %v", err)
	}
}

func TestValidatePodAnnotations_MultiplePodsUsesYoungest(t *testing.T) {
	oldPod := makePod("old-pod", map[string]string{"app": "test"},
		correctAnnotations("wrong-vol", "wrong-secret"),
		time.Now().Add(-1*time.Hour),
	)
	newPod := makePod("new-pod", map[string]string{"app": "test"},
		correctAnnotations("my-vol", envoySecretName),
		time.Now(),
	)

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(oldPod, newPod).Build()
	scope := buildScope(true, map[string]string{"app": "test"})

	err := validation.ValidatePodAnnotations(context.Background(), k8sClient, scope)
	if err != nil {
		t.Errorf("expected nil error when youngest pod is correctly configured, got: %v", err)
	}
}

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	return s
}

func volumeAnnotation(volName, secretName string) string {
	return fmt.Sprintf(`[{"name":%q,"secret":{"secretName":%q}}]`, volName, secretName)
}

func mountAnnotation(volName, mountPath string) string {
	return fmt.Sprintf(`[{"name":%q,"mountPath":%q}]`, volName, mountPath)
}

func correctAnnotations(volName, secretName string) map[string]string {
	return map[string]string{
		"sidecar.istio.io/userVolume":      volumeAnnotation(volName, secretName),
		"sidecar.istio.io/userVolumeMount": mountAnnotation(volName, istioCredentialsDirectory),
	}
}

func buildScope(autoLoginEnabled bool, matchLabels map[string]string) *state.Scope {
	return &state.Scope{
		AuthPolicy: ztoperatorv1alpha1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: "default",
			},
			Spec: ztoperatorv1alpha1.AuthPolicySpec{
				Selector: ztoperatorv1alpha1.WorkloadSelector{
					MatchLabels: matchLabels,
				},
			},
		},
		AutoLoginConfig: state.AutoLoginConfig{
			Enabled:         autoLoginEnabled,
			EnvoySecretName: envoySecretName,
		},
	}
}

func makePod(name string, labels map[string]string, annotations map[string]string, creationTime time.Time) *corev1.Pod {
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
