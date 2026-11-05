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

// IsSidecarInjectionDisabled says whether the pod opts out of Istio sidecar injection.
// Like Istio, the sidecar.istio.io/inject label takes precedence over the annotation.
func IsSidecarInjectionDisabled(pod *corev1.Pod) bool {
	if value, ok := pod.Labels[IstioInjectKey]; ok {
		return value == "false"
	}
	return pod.Annotations[IstioInjectKey] == "false"
}

// IsEnvoySecretMountedInIstioProxy says whether the pod spec mounts the given secret, with both
// credential files available, at the Istio credentials directory in a running istio-proxy container
// (regular container or restartable native sidecar).
func IsEnvoySecretMountedInIstioProxy(pod *corev1.Pod, secretName string) bool {
	secretVolumes := credentialVolumeNames(pod, secretName)
	if len(secretVolumes) == 0 {
		return false
	}

	for _, container := range runningContainers(pod) {
		if container.Name == IstioProxyContainerName && mountsCredentialsDirectory(container, secretVolumes) {
			return true
		}
	}
	return false
}

// credentialVolumeNames returns the names of volumes that expose the given secret with both credential files.
func credentialVolumeNames(pod *corev1.Pod, secretName string) map[string]bool {
	volumes := map[string]bool{}
	for _, volume := range pod.Spec.Volumes {
		if volume.Secret != nil && volume.Secret.SecretName == secretName && exposesCredentialFiles(volume.Secret) {
			volumes[volume.Name] = true
		}
	}
	return volumes
}

// runningContainers returns the regular containers and the restartable init containers (native sidecars).
func runningContainers(pod *corev1.Pod) []corev1.Container {
	containers := append([]corev1.Container{}, pod.Spec.Containers...)
	for _, container := range pod.Spec.InitContainers {
		if container.RestartPolicy != nil && *container.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			containers = append(containers, container)
		}
	}
	return containers
}

func mountsCredentialsDirectory(container corev1.Container, volumes map[string]bool) bool {
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == configpatch.IstioCredentialsDirectory &&
			mount.SubPath == "" && mount.SubPathExpr == "" && volumes[mount.Name] {
			return true
		}
	}
	return false
}

// exposesCredentialFiles reports whether the secret volume exposes both credential files
// under their expected names. Without explicit items, all keys are projected.
func exposesCredentialFiles(secret *corev1.SecretVolumeSource) bool {
	if len(secret.Items) == 0 {
		return true
	}
	exposed := map[string]bool{}
	for _, item := range secret.Items {
		if item.Key == item.Path {
			exposed[item.Path] = true
		}
	}
	return exposed[configpatch.TokenSecretFileName] && exposed[configpatch.HmacSecretFileName]
}

func PodErrorMessageSuffix() string {
	return fmt.Sprintf(
		"see https://github.com/kartverket/ztoperator/blob/%s/README.md#-mounting-oauth-credentials-in-the-istio-sidecar "+
			"on how to do it correctly",
		config.Get().GitRef,
	)
}
