package configmap_test

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/eventhandler/configmap"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestConfigMapEventHandler_WithNonConfigMapObject_ReturnsNoRequests(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	k8sClient := createFakeClientForConfigMapHandler()
	h := configmap.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "some-secret", Namespace: "default"},
	}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: secret}, queue)

	// 3. Assert
	assert.Equal(t, 0, queue.Len(), "Expected no reconcile requests for non-configmap object")
}

func TestConfigMapEventHandler_WithConfigMapOwnedByAuthPolicy_ReturnsNoRequests(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	k8sClient := createFakeClientForConfigMapHandler()
	h := configmap.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owned-configmap",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "ztoperator.kartverket.no/v1alpha1",
					Kind:       "AuthPolicy",
					Name:       "my-auth-policy",
				},
			},
		},
	}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: cm}, queue)

	// 3. Assert
	assert.Equal(t, 0, queue.Len(), "Expected no reconcile requests for ConfigMap owned by AuthPolicy")
}

func TestConfigMapEventHandler_WithConfigMapInNamespaceWithNoAuthPolicies_ReturnsNoRequests(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	k8sClient := createFakeClientForConfigMapHandler()
	h := configmap.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-configmap", Namespace: "default"},
	}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: cm}, queue)

	// 3. Assert
	assert.Equal(t, 0, queue.Len(), "Expected no reconcile requests when no AuthPolicies exist in namespace")
}

func TestConfigMapEventHandler_WithConfigMapInNamespaceWithAuthPolicies_ReturnsRequestForEachAuthPolicy(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy1 := &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-one", Namespace: "default"},
	}
	authPolicy2 := &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-two", Namespace: "default"},
	}
	k8sClient := createFakeClientForConfigMapHandler(authPolicy1, authPolicy2)
	h := configmap.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-configmap", Namespace: "default"},
	}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: cm}, queue)

	// 3. Assert
	requests := drainQueue(queue)
	assert.Len(t, requests, 2)
	assert.Contains(t, requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: "policy-one", Namespace: "default"}})
	assert.Contains(t, requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: "policy-two", Namespace: "default"}})
}

func TestConfigMapEventHandler_WithConfigMapInNamespace_DoesNotEnqueueAuthPoliciesFromOtherNamespaces(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicyInSameNamespace := &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "same-namespace-policy", Namespace: "default"},
	}
	authPolicyInOtherNamespace := &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "other-namespace-policy", Namespace: "other"},
	}
	k8sClient := createFakeClientForConfigMapHandler(authPolicyInSameNamespace, authPolicyInOtherNamespace)
	h := configmap.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-configmap", Namespace: "default"},
	}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: cm}, queue)

	// 3. Assert
	requests := drainQueue(queue)
	assert.Len(t, requests, 1)
	assert.Contains(t, requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: "same-namespace-policy", Namespace: "default"}})
}

func createFakeClientForConfigMapHandler(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = ztoperatorv1alpha1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func drainQueue(queue workqueue.TypedRateLimitingInterface[reconcile.Request]) []reconcile.Request {
	var requests []reconcile.Request
	for queue.Len() > 0 {
		item, _ := queue.Get()
		requests = append(requests, item)
		queue.Done(item)
	}
	return requests
}
