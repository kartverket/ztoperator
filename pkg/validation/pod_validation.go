package validation

import (
	"fmt"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/names"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	corev1 "k8s.io/api/core/v1"
)

const (
	IstioUserVolumeAnnotation      = "sidecar.istio.io/userVolume"
	IstioUserVolumeMountAnnotation = "sidecar.istio.io/userVolumeMount"
	IstioInjectKey                 = "sidecar.istio.io/inject"

	IstioProxyContainerName = "istio-proxy"
)

// IsSidecarInjectionDisabled reports whether the pod opts out of Istio sidecar injection,
// either through the sidecar.istio.io/inject label or annotation.
func IsSidecarInjectionDisabled(pod *corev1.Pod) bool {
	return pod.Labels[IstioInjectKey] == "false" || pod.Annotations[IstioInjectKey] == "false"
}

// ValidatePod validates the final pod spec against the AuthPolicy.
func ValidatePod(pod *corev1.Pod, authPolicy v1alpha1.AuthPolicy) error {
	if authPolicy.Spec.Enabled && IsSidecarInjectionDisabled(pod) {
		return fmt.Errorf(
			"pod opts out of Istio sidecar injection (%s: \"false\"), "+
				"but the enabled AuthPolicy '%s' is enforced by the sidecar",
			IstioInjectKey,
			authPolicy.Name,
		)
	}

	if authPolicy.Spec.AutoLogin == nil || !authPolicy.Spec.AutoLogin.Enabled {
		return nil
	}

	envoySecretName := names.EnvoySecret(authPolicy.Name)
	if !IsEnvoySecretMountedInIstioProxy(pod, envoySecretName) {
		return fmt.Errorf(
			"secret with name '%s' used by OAuth-EnvoyFilter is not mounted at '%s' in the %s container, %s",
			envoySecretName,
			configpatch.IstioCredentialsDirectory,
			IstioProxyContainerName,
			PodErrorMessageSuffix(),
		)
	}

	return nil
}

func PodErrorMessageSuffix() string {
	return fmt.Sprintf(
		"see https://github.com/kartverket/ztoperator/blob/%s/README.md#-mounting-oauth-credentials-in-the-istio-sidecar "+
			"on how to do it correctly",
		config.Get().GitRef,
	)
}

// IsEnvoySecretMountedInIstioProxy reports whether the pod spec mounts the given secret
// at the Istio credentials directory in the istio-proxy container (regular or native sidecar).
func IsEnvoySecretMountedInIstioProxy(pod *corev1.Pod, secretName string) bool {
	secretVolumes := map[string]bool{}
	for _, volume := range pod.Spec.Volumes {
		if volume.Secret != nil && volume.Secret.SecretName == secretName {
			secretVolumes[volume.Name] = true
		}
	}
	if len(secretVolumes) == 0 {
		return false
	}

	containers := make([]corev1.Container, 0, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
	containers = append(containers, pod.Spec.Containers...)
	containers = append(containers, pod.Spec.InitContainers...)
	for _, container := range containers {
		if container.Name != IstioProxyContainerName {
			continue
		}
		for _, mount := range container.VolumeMounts {
			if mount.MountPath == configpatch.IstioCredentialsDirectory && secretVolumes[mount.Name] {
				return true
			}
		}
	}
	return false
}
