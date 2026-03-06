package validation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	return s
}

func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func makeAnnotations(volumes []istioUserVolume, mounts []istioUserVolumeMount) map[string]string {
	return map[string]string{
		istioUserVolumeAnnotation:      mustMarshal(volumes),
		istioUserVolumeMountAnnotation: mustMarshal(mounts),
	}
}

// Tests for validateSecretVolumeMount

func TestValidateSecretVolumeMount_EmptyMounts_ReturnsFalse(t *testing.T) {
	result := validateSecretVolumeMount(
		[]istioUserVolumeMount{},
		[]istioUserVolume{{Name: "my-vol", Secret: &istioUserVolumeSecret{SecretName: "my-secret"}}},
		"my-secret",
	)
	if result {
		t.Error("expected false for empty mounts, got true")
	}
}

func TestValidateSecretVolumeMount_MatchingMountAndSecret_ReturnsTrue(t *testing.T) {
	mounts := []istioUserVolumeMount{{Name: "my-vol", MountPath: configpatch.IstioCredentialsDirectory}}
	volumes := []istioUserVolume{{Name: "my-vol", Secret: &istioUserVolumeSecret{SecretName: "my-secret"}}}

	result := validateSecretVolumeMount(mounts, volumes, "my-secret")
	if !result {
		t.Error("expected true for matching mount and secret, got false")
	}
}

func TestValidateSecretVolumeMount_WrongSecretName_ReturnsFalse(t *testing.T) {
	mounts := []istioUserVolumeMount{{Name: "my-vol", MountPath: configpatch.IstioCredentialsDirectory}}
	volumes := []istioUserVolume{{Name: "my-vol", Secret: &istioUserVolumeSecret{SecretName: "other-secret"}}}

	result := validateSecretVolumeMount(mounts, volumes, "my-secret")
	if result {
		t.Error("expected false for wrong secret name, got true")
	}
}

func TestValidateSecretVolumeMount_MountNameDoesNotMatchVolumeName_ReturnsFalse(t *testing.T) {
	mounts := []istioUserVolumeMount{{Name: "other-vol", MountPath: configpatch.IstioCredentialsDirectory}}
	volumes := []istioUserVolume{{Name: "my-vol", Secret: &istioUserVolumeSecret{SecretName: "my-secret"}}}

	result := validateSecretVolumeMount(mounts, volumes, "my-secret")
	if result {
		t.Error("expected false when mount name does not match volume name, got true")
	}
}

func TestValidateSecretVolumeMount_VolumeWithNilSecret_ReturnsFalse(t *testing.T) {
	mounts := []istioUserVolumeMount{{Name: "my-vol", MountPath: configpatch.IstioCredentialsDirectory}}
	volumes := []istioUserVolume{{Name: "my-vol", Secret: nil}}

	result := validateSecretVolumeMount(mounts, volumes, "my-secret")
	if result {
		t.Error("expected false when volume has no secret, got true")
	}
}

// Tests for collectIstioVolumesAndMountsFromPod

func TestCollectIstioVolumesAndMountsFromPod_MissingVolumeAnnotation_ReturnsError(t *testing.T) {
	annotations := map[string]string{
		istioUserVolumeMountAnnotation: mustMarshal([]istioUserVolumeMount{}),
	}

	_, _, err := collectIstioVolumesAndMountsFromPod(annotations)
	if err == nil {
		t.Error("expected error for missing volume annotation, got nil")
	}
}

func TestCollectIstioVolumesAndMountsFromPod_MissingMountAnnotation_ReturnsError(t *testing.T) {
	annotations := map[string]string{
		istioUserVolumeAnnotation: mustMarshal([]istioUserVolume{}),
	}

	_, _, err := collectIstioVolumesAndMountsFromPod(annotations)
	if err == nil {
		t.Error("expected error for missing mount annotation, got nil")
	}
}

func TestCollectIstioVolumesAndMountsFromPod_MalformedVolumeAnnotation_ReturnsError(t *testing.T) {
	annotations := map[string]string{
		istioUserVolumeAnnotation:      "not-valid-json",
		istioUserVolumeMountAnnotation: mustMarshal([]istioUserVolumeMount{}),
	}

	_, _, err := collectIstioVolumesAndMountsFromPod(annotations)
	if err == nil {
		t.Error("expected error for malformed volume annotation, got nil")
	}
}

func TestCollectIstioVolumesAndMountsFromPod_MalformedMountAnnotation_ReturnsError(t *testing.T) {
	annotations := map[string]string{
		istioUserVolumeAnnotation:      mustMarshal([]istioUserVolume{}),
		istioUserVolumeMountAnnotation: "not-valid-json",
	}

	_, _, err := collectIstioVolumesAndMountsFromPod(annotations)
	if err == nil {
		t.Error("expected error for malformed mount annotation, got nil")
	}
}

func TestCollectIstioVolumesAndMountsFromPod_ValidAnnotations_ReturnsVolumesAndMounts(t *testing.T) {
	expectedVolumes := []istioUserVolume{
		{Name: "my-vol", Secret: &istioUserVolumeSecret{SecretName: "my-secret"}},
	}
	expectedMounts := []istioUserVolumeMount{
		{Name: "my-vol", MountPath: configpatch.IstioCredentialsDirectory, ReadOnly: true},
	}

	annotations := makeAnnotations(expectedVolumes, expectedMounts)

	volumes, mounts, err := collectIstioVolumesAndMountsFromPod(annotations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(volumes) != 1 || volumes[0].Name != "my-vol" {
		t.Errorf("unexpected volumes: %+v", volumes)
	}
	if len(mounts) != 1 || mounts[0].Name != "my-vol" {
		t.Errorf("unexpected mounts: %+v", mounts)
	}
}

// Tests for validatePodAnnotations

func buildScope(autoLoginEnabled bool, envoySecretName string, matchLabels map[string]string) *state.Scope {
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

func TestValidatePodAnnotations_AutoLoginDisabled_ReturnsNil(t *testing.T) {
	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).Build()
	scope := buildScope(false, "my-secret", map[string]string{"app": "test"})

	err := validatePodAnnotations(context.Background(), k8sClient, scope)
	if err != nil {
		t.Errorf("expected nil error when autoLogin is disabled, got: %v", err)
	}
}

func TestValidatePodAnnotations_NoPodsFound_ReturnsError(t *testing.T) {
	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).Build()
	scope := buildScope(true, "my-secret", map[string]string{"app": "test"})

	err := validatePodAnnotations(context.Background(), k8sClient, scope)
	if err == nil {
		t.Error("expected error when no pods found, got nil")
	}
}

func TestValidatePodAnnotations_PodMissingVolumeAnnotation_ReturnsError(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels:    map[string]string{"app": "test"},
			Annotations: map[string]string{
				istioUserVolumeMountAnnotation: mustMarshal([]istioUserVolumeMount{}),
			},
			CreationTimestamp: metav1.Now(),
		},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(pod).Build()
	scope := buildScope(true, "my-secret", map[string]string{"app": "test"})

	err := validatePodAnnotations(context.Background(), k8sClient, scope)
	if err == nil {
		t.Error("expected error when volume annotation is missing, got nil")
	}
}

func TestValidatePodAnnotations_SecretNotMounted_ReturnsError(t *testing.T) {
	volumes := []istioUserVolume{
		{Name: "other-vol", Secret: &istioUserVolumeSecret{SecretName: "other-secret"}},
	}
	mounts := []istioUserVolumeMount{
		{Name: "other-vol", MountPath: configpatch.IstioCredentialsDirectory},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-pod",
			Namespace:         "default",
			Labels:            map[string]string{"app": "test"},
			Annotations:       makeAnnotations(volumes, mounts),
			CreationTimestamp: metav1.Now(),
		},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(pod).Build()
	scope := buildScope(true, "my-secret", map[string]string{"app": "test"})

	err := validatePodAnnotations(context.Background(), k8sClient, scope)
	if err == nil {
		t.Error("expected error when secret is not mounted, got nil")
	}
}

func TestValidatePodAnnotations_CorrectlyMountedSecret_ReturnsNil(t *testing.T) {
	secretName := "my-secret"
	volumes := []istioUserVolume{
		{Name: "my-vol", Secret: &istioUserVolumeSecret{SecretName: secretName}},
	}
	mounts := []istioUserVolumeMount{
		{Name: "my-vol", MountPath: configpatch.IstioCredentialsDirectory},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-pod",
			Namespace:         "default",
			Labels:            map[string]string{"app": "test"},
			Annotations:       makeAnnotations(volumes, mounts),
			CreationTimestamp: metav1.Now(),
		},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(pod).Build()
	scope := buildScope(true, secretName, map[string]string{"app": "test"})

	err := validatePodAnnotations(context.Background(), k8sClient, scope)
	if err != nil {
		t.Errorf("expected nil error for correctly mounted secret, got: %v", err)
	}
}

func TestValidatePodAnnotations_MultiplePodsUsesYoungest(t *testing.T) {
	secretName := "my-secret"
	correctVolumes := []istioUserVolume{
		{Name: "my-vol", Secret: &istioUserVolumeSecret{SecretName: secretName}},
	}
	correctMounts := []istioUserVolumeMount{
		{Name: "my-vol", MountPath: configpatch.IstioCredentialsDirectory},
	}
	wrongVolumes := []istioUserVolume{
		{Name: "wrong-vol", Secret: &istioUserVolumeSecret{SecretName: "wrong-secret"}},
	}
	wrongMounts := []istioUserVolumeMount{
		{Name: "wrong-vol", MountPath: configpatch.IstioCredentialsDirectory},
	}

	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "old-pod",
			Namespace:         "default",
			Labels:            map[string]string{"app": "test"},
			Annotations:       makeAnnotations(wrongVolumes, wrongMounts),
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
		},
	}
	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "new-pod",
			Namespace:         "default",
			Labels:            map[string]string{"app": "test"},
			Annotations:       makeAnnotations(correctVolumes, correctMounts),
			CreationTimestamp: metav1.Now(),
		},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(oldPod, newPod).Build()
	scope := buildScope(true, secretName, map[string]string{"app": "test"})

	err := validatePodAnnotations(context.Background(), k8sClient, scope)
	if err != nil {
		t.Errorf("expected nil error when youngest pod is correctly configured, got: %v", err)
	}
}
