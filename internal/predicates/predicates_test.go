package predicates_test

import (
	"testing"

	"github.com/kartverket/ztoperator/internal/predicates"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestSpecOrLabelsChanged_WithStatusOnlyUpdate_DropsEvent(t *testing.T) {
	old := istioLikeObject(1, map[string]string{"app": "some-app"}, "1000")
	updated := istioLikeObject(1, map[string]string{"app": "some-app"}, "1001")

	assert.False(t, predicates.SpecOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestSpecOrLabelsChanged_WithGenerationBump_PassesEvent(t *testing.T) {
	old := istioLikeObject(1, map[string]string{"app": "some-app"}, "1000")
	updated := istioLikeObject(2, map[string]string{"app": "some-app"}, "1001")

	assert.True(t, predicates.SpecOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestSpecOrLabelsChanged_WithChangedLabels_PassesEvent(t *testing.T) {
	old := istioLikeObject(1, map[string]string{"app": "some-app"}, "1000")
	updated := istioLikeObject(1, map[string]string{"app": "some-other-app"}, "1001")

	assert.True(t, predicates.SpecOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestSpecOrLabelsChanged_WithCreateOrDelete_PassesEvent(t *testing.T) {
	obj := istioLikeObject(1, nil, "1000")

	assert.True(t, predicates.SpecOrLabelsChanged().Create(event.CreateEvent{Object: obj}))
	assert.True(t, predicates.SpecOrLabelsChanged().Delete(event.DeleteEvent{Object: obj}))
}

func TestSecretContentOrLabelsChanged_WithAnnotationOnlyUpdate_DropsEvent(t *testing.T) {
	old := &corev1.Secret{
		Name: "my-secret", Namespace: "default",
		Data: map[string][]byte{"client-secret": []byte("hunter2")},
	}
	updated := old.DeepCopy()
	updated.Annotations = map[string]string{"some.controller/last-seen": "now"}

	assert.False(t, predicates.SecretContentOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestSecretContentOrLabelsChanged_WithChangedData_PassesEvent(t *testing.T) {
	old := &corev1.Secret{
		Name: "my-secret", Namespace: "default",
		Data: map[string][]byte{"client-secret": []byte("hunter2")},
	}
	updated := old.DeepCopy()
	updated.Data["client-secret"] = []byte("rotated")

	assert.True(t, predicates.SecretContentOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestSecretContentOrLabelsChanged_WithChangedLabels_PassesEvent(t *testing.T) {
	old := &corev1.Secret{
		Name: "my-secret", Namespace: "default", Labels: map[string]string{"app": "some-app"},
		Data: map[string][]byte{"client-secret": []byte("hunter2")},
	}
	updated := old.DeepCopy()
	updated.Labels["app"] = "some-other-app"

	assert.True(t, predicates.SecretContentOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestConfigMapContentOrLabelsChanged_WithAnnotationOnlyUpdate_DropsEvent(t *testing.T) {
	old := &corev1.ConfigMap{
		Name: "my-configmap", Namespace: "default",
		Data: map[string]string{"AUDIENCE": "some-audience"},
	}
	updated := old.DeepCopy()
	updated.Annotations = map[string]string{"some.controller/last-seen": "now"}

	assert.False(t, predicates.ConfigMapContentOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestConfigMapContentOrLabelsChanged_WithChangedData_PassesEvent(t *testing.T) {
	old := &corev1.ConfigMap{
		Name: "my-configmap", Namespace: "default",
		Data: map[string]string{"AUDIENCE": "some-audience"},
	}
	updated := old.DeepCopy()
	updated.Data["AUDIENCE"] = "some-other-audience"

	assert.True(t, predicates.ConfigMapContentOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestConfigMapContentOrLabelsChanged_WithChangedLabels_PassesEvent(t *testing.T) {
	old := &corev1.ConfigMap{
		Name: "my-configmap", Namespace: "default", Labels: map[string]string{"app": "some-app"},
		Data: map[string]string{"AUDIENCE": "some-audience"},
	}
	updated := old.DeepCopy()
	updated.Labels["app"] = "some-other-app"

	assert.True(t, predicates.ConfigMapContentOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestDeploymentPodTemplateMetadataOrLabelsChanged_WithReplicaOnlyUpdate_DropsEvent(t *testing.T) {
	old := deployment(nil, nil)
	updated := old.DeepCopy()
	updated.Spec.Replicas = new(int32(3))
	updated.Generation = 2

	assert.False(t, predicates.DeploymentPodTemplateMetadataOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestDeploymentPodTemplateMetadataOrLabelsChanged_WithStatusOnlyUpdate_DropsEvent(t *testing.T) {
	old := deployment(nil, nil)
	updated := old.DeepCopy()
	updated.Status.ReadyReplicas = 1
	updated.ResourceVersion = "1001"

	assert.False(t, predicates.DeploymentPodTemplateMetadataOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestDeploymentPodTemplateMetadataOrLabelsChanged_WithChangedTemplateLabels_PassesEvent(t *testing.T) {
	old := deployment(map[string]string{"app": "some-app"}, nil)
	updated := old.DeepCopy()
	updated.Spec.Template.Labels["istio.io/dataplane-mode"] = "none"

	assert.True(t, predicates.DeploymentPodTemplateMetadataOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestDeploymentPodTemplateMetadataOrLabelsChanged_WithAddedTemplateAnnotation_PassesEvent(t *testing.T) {
	old := deployment(map[string]string{"app": "some-app"}, nil)
	updated := old.DeepCopy()
	updated.Spec.Template.Annotations = map[string]string{"ztoperator.kartverket.no/inject": "true"}

	assert.True(t, predicates.DeploymentPodTemplateMetadataOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestDeploymentPodTemplateMetadataOrLabelsChanged_WithRemovedTemplateAnnotation_PassesEvent(t *testing.T) {
	old := deployment(nil, map[string]string{"ztoperator.kartverket.no/inject": "true"})
	updated := old.DeepCopy()
	delete(updated.Spec.Template.Annotations, "ztoperator.kartverket.no/inject")

	assert.True(t, predicates.DeploymentPodTemplateMetadataOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestDeploymentPodTemplateMetadataOrLabelsChanged_WithNilVsEmptyTemplateLabels_DropsEvent(t *testing.T) {
	old := deployment(nil, nil)
	updated := old.DeepCopy()
	updated.Spec.Template.Labels = map[string]string{}

	assert.False(t, predicates.DeploymentPodTemplateMetadataOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestDeploymentPodTemplateMetadataOrLabelsChanged_WithNonDeploymentObjects_PassesEvent(t *testing.T) {
	old := istioLikeObject(1, nil, "1000")
	updated := istioLikeObject(1, nil, "1001")

	assert.True(t, predicates.DeploymentPodTemplateMetadataOrLabelsChanged().Update(updateEvent(old, updated)))
}

func TestDeploymentPodTemplateMetadataOrLabelsChanged_WithCreateOrDelete_PassesEvent(t *testing.T) {
	obj := deployment(nil, nil)

	assert.True(t, predicates.DeploymentPodTemplateMetadataOrLabelsChanged().Create(event.CreateEvent{Object: obj}))
	assert.True(t, predicates.DeploymentPodTemplateMetadataOrLabelsChanged().Delete(event.DeleteEvent{Object: obj}))
}

func deployment(templateLabels, templateAnnotations map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		Name: "some-deployment", Namespace: "default", Generation: 1, ResourceVersion: "1000",
		Labels: map[string]string{"app": "some-app"},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Template: corev1.PodTemplateSpec{
				Labels:      templateLabels,
				Annotations: templateAnnotations,
			},
		},
	}
}

func istioLikeObject(generation int64, labels map[string]string, resourceVersion string) client.Object {
	return &corev1.Pod{
		Name:            "some-resource",
		Namespace:       "default",
		Generation:      generation,
		Labels:          labels,
		ResourceVersion: resourceVersion,
	}
}

func updateEvent(old, updated client.Object) event.UpdateEvent {
	return event.UpdateEvent{ObjectOld: old, ObjectNew: updated}
}
