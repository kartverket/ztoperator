package validation_test

import (
	"strings"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/names"
	"github.com/kartverket/ztoperator/pkg/validation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	istioCredentialsDirectory = "/etc/istio/config"
	authPolicyName            = "test-policy"
	testVolumeName            = "v"
)

func TestValidatePod_AutoLoginDisabled_ReturnsNil(t *testing.T) {
	authPolicy := buildAuthPolicy(false, true)

	if err := validation.ValidatePod(&corev1.Pod{}, authPolicy); err != nil {
		t.Errorf("expected nil error when autoLogin is disabled, got: %v", err)
	}
}

func TestValidatePod_MountErrors(t *testing.T) {
	authPolicy := buildAuthPolicy(true, true)
	secret := names.EnvoySecret(authPolicyName)

	tests := map[string]*corev1.Pod{
		"no istio-proxy container": podWith(nil, secretVolume(secret), nil),
		"no volume": podWith(
			[]corev1.Container{proxyWithMount(istioCredentialsDirectory)},
			nil,
			nil,
		),
		"volume references other secret": podWith(
			[]corev1.Container{proxyWithMount(istioCredentialsDirectory)},
			secretVolume("other-secret"),
			nil,
		),
		"mounted at wrong path": podWith(
			[]corev1.Container{proxyWithMount("/some/other/path")},
			secretVolume(secret),
			nil,
		),
		"mounted in other container only": podWith(
			[]corev1.Container{{
				Name:         "app",
				VolumeMounts: []corev1.VolumeMount{{Name: testVolumeName, MountPath: istioCredentialsDirectory}},
			}},
			secretVolume(secret),
			nil,
		),
		"annotations without a mounted spec": podWith(nil, nil, map[string]string{
			validation.IstioUserVolumeAnnotation:      `[{"name":"v","secret":{"secretName":"` + secret + `"}}]`,
			validation.IstioUserVolumeMountAnnotation: `[{"name":"v","mountPath":"/etc/istio/config"}]`,
		}),
	}

	for name, pod := range tests {
		t.Run(name, func(t *testing.T) {
			err := validation.ValidatePod(pod, authPolicy)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), secret) {
				t.Errorf("expected error to mention %q, got: %v", secret, err)
			}
		})
	}
}

func TestValidatePod_SecretMountedInIstioProxy_ReturnsNil(t *testing.T) {
	authPolicy := buildAuthPolicy(true, true)
	secret := names.EnvoySecret(authPolicyName)

	regular := podWith(
		[]corev1.Container{proxyWithMount(istioCredentialsDirectory)},
		secretVolume(secret),
		nil,
	)
	native := podWith(nil, secretVolume(secret), nil)
	native.Spec.InitContainers = []corev1.Container{proxyWithMount(istioCredentialsDirectory)}

	for name, pod := range map[string]*corev1.Pod{"container": regular, "init container": native} {
		if err := validation.ValidatePod(pod, authPolicy); err != nil {
			t.Errorf("expected nil error for istio-proxy in %s, got: %v", name, err)
		}
	}
}

func TestValidatePod_SidecarInjectionDisabled(t *testing.T) {
	labelPod := podWith(nil, nil, nil)
	labelPod.Labels = map[string]string{validation.IstioInjectKey: "false"}
	annotationPod := podWith(nil, nil, map[string]string{validation.IstioInjectKey: "false"})

	for name, pod := range map[string]*corev1.Pod{"label": labelPod, "annotation": annotationPod} {
		if err := validation.ValidatePod(pod, buildAuthPolicy(false, true)); err == nil {
			t.Errorf("expected error for inject=false %s with enabled AuthPolicy", name)
		}
		if err := validation.ValidatePod(pod, buildAuthPolicy(false, false)); err != nil {
			t.Errorf("expected nil for inject=false %s with disabled AuthPolicy, got: %v", name, err)
		}
	}
}

func buildAuthPolicy(autoLoginEnabled, enabled bool) ztoperatorv1alpha1.AuthPolicy {
	authPolicy := ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: authPolicyName, Namespace: "default"},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			Enabled:  enabled,
			Selector: ztoperatorv1alpha1.WorkloadSelector{MatchLabels: map[string]string{"app": "test"}},
		},
	}
	if autoLoginEnabled {
		authPolicy.Spec.AutoLogin = &ztoperatorv1alpha1.AutoLogin{Enabled: true}
	}
	return authPolicy
}

func podWith(containers []corev1.Container, volume *corev1.Volume, annotations map[string]string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default", Annotations: annotations},
		Spec:       corev1.PodSpec{Containers: containers},
	}
	if volume != nil {
		pod.Spec.Volumes = []corev1.Volume{*volume}
	}
	return pod
}

func secretVolume(secretName string) *corev1.Volume {
	return &corev1.Volume{
		Name:         testVolumeName,
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}},
	}
}

func proxyWithMount(mountPath string) corev1.Container {
	return corev1.Container{
		Name:         validation.IstioProxyContainerName,
		VolumeMounts: []corev1.VolumeMount{{Name: testVolumeName, MountPath: mountPath}},
	}
}
