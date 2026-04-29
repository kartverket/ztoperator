package validation

import (
	"encoding/json"
	"fmt"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/names"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	corev1 "k8s.io/api/core/v1"
)

const (
	IstioUserVolumeAnnotation      = "sidecar.istio.io/userVolume"
	istioUserVolumeMountAnnotation = "sidecar.istio.io/userVolumeMount"
)

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

func ValidatePodAnnotations(pod *corev1.Pod, authPolicy v1alpha1.AuthPolicy) error {
	if authPolicy.Spec.AutoLogin == nil || !authPolicy.Spec.AutoLogin.Enabled {
		return nil
	}

	volumes, volumeMounts, collectIstioMountsErr := collectIstioVolumesAndMountsFromPod(pod.Annotations)
	if collectIstioMountsErr != nil {
		return collectIstioMountsErr
	}
	envoySecretVolumeMounts := make([]istioUserVolumeMount, 0, len(volumeMounts))
	for _, volumeMount := range volumeMounts {
		if volumeMount.MountPath == configpatch.IstioCredentialsDirectory {
			envoySecretVolumeMounts = append(envoySecretVolumeMounts, volumeMount)
		}
	}

	envoySecretName := names.EnvoySecret(authPolicy.Name)
	if len(envoySecretVolumeMounts) == 0 || !validateSecretVolumeMount(
		envoySecretVolumeMounts,
		volumes,
		envoySecretName,
	) {
		return fmt.Errorf(
			"secret with name '%s' used by OAuth-EnvoyFilter is not mounted in istio-proxy, %s",
			envoySecretName,
			PodAnnotationErrorMessageSuffix(),
		)
	}

	return nil
}

func PodAnnotationErrorMessageSuffix() string {
	return fmt.Sprintf(
		"see https://github.com/kartverket/ztoperator/blob/%s/README.md#-mounting-oauth-credentials-in-the-istio-sidecar "+
			"on how to do it correctly",
		config.Get().GitRef,
	)
}

func collectIstioVolumesAndMountsFromPod(
	podAnnotations map[string]string,
) ([]istioUserVolume, []istioUserVolumeMount, error) {
	var volumes []istioUserVolume
	if err := json.Unmarshal([]byte(podAnnotations[IstioUserVolumeAnnotation]), &volumes); err != nil {
		return nil, nil, fmt.Errorf(
			"the required annotation '%s' is either missing or its content is not properly formatted, %s",
			IstioUserVolumeAnnotation,
			PodAnnotationErrorMessageSuffix(),
		)
	}

	var volumeMounts []istioUserVolumeMount
	if err := json.Unmarshal([]byte(podAnnotations[istioUserVolumeMountAnnotation]), &volumeMounts); err != nil {
		return nil, nil, fmt.Errorf(
			"the required annotation '%s' is either missing or its content is not properly formatted, %s",
			istioUserVolumeMountAnnotation,
			PodAnnotationErrorMessageSuffix(),
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

	for _, mount := range mounts {
		for _, volume := range userVolumes {
			if volume.Name != mount.Name {
				continue
			}
			if volume.Secret != nil && volume.Secret.SecretName == expectedSecretName {
				return true
			}
		}
	}

	return false
}
