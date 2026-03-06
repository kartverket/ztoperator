package validation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	istioUserVolumeAnnotation      = "sidecar.istio.io/userVolume"
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

func podAnnotationErrorMessageSuffix() string {
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

func validatePodAnnotations(ctx context.Context, k8sClient client.Client, scope *state.Scope) error {
	if !scope.AutoLoginConfig.Enabled {
		return nil
	}

	pods, getPodsErr := helperfunctions.GetProtectedPods(ctx, k8sClient, scope.AuthPolicy)
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
	envoySecretVolumeMounts := make([]istioUserVolumeMount, 0, len(volumeMounts))
	for _, volumeMount := range volumeMounts {
		if volumeMount.MountPath == configpatch.IstioCredentialsDirectory {
			envoySecretVolumeMounts = append(envoySecretVolumeMounts, volumeMount)
		}
	}

	if len(envoySecretVolumeMounts) == 0 || !validateSecretVolumeMount(
		envoySecretVolumeMounts,
		volumes,
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
