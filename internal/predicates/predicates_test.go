package predicates_test

import (
	"testing"

	"github.com/kartverket/ztoperator/internal/predicates"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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


func istioLikeObject(generation int64, labels map[string]string, resourceVersion string) client.Object {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "some-resource",
			Namespace:       "default",
			Generation:      generation,
			Labels:          labels,
			ResourceVersion: resourceVersion,
		},
	}
}

func updateEvent(old, updated client.Object) event.UpdateEvent {
	return event.UpdateEvent{ObjectOld: old, ObjectNew: updated}
}
