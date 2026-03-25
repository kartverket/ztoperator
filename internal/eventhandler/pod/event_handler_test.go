package pod_test

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/eventhandler/pod"
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

func TestEventHandler_WithNonPodObject_ReturnsNoRequests(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	k8sClient := createFakeClientForPodHandler()
	h := pod.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "some-configmap", Namespace: "default"},
	}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: configMap}, queue)

	// 3. Assert
	assert.Equal(t, 0, queue.Len(), "Expected no reconcile requests for non-pod object")
}

func TestEventHandler_WithPodInNamespaceWithNoAuthPolicies_ReturnsNoRequests(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	k8sClient := createFakeClientForPodHandler()
	h := pod.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
	}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: p}, queue)

	// 3. Assert
	assert.Equal(t, 0, queue.Len(), "Expected no reconcile requests when no AuthPolicies exist in namespace")
}

func TestEventHandler_WithPodInNamespaceWithAuthPolicies_ReturnsRequestForEachAuthPolicy(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy1 := &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-one", Namespace: "default"},
	}
	authPolicy2 := &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-two", Namespace: "default"},
	}
	k8sClient := createFakeClientForPodHandler(authPolicy1, authPolicy2)
	h := pod.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
	}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: p}, queue)

	// 3. Assert
	requests := drainQueue(queue)
	assert.Len(t, requests, 2)
	assert.Contains(t, requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: "policy-one", Namespace: "default"}})
	assert.Contains(t, requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: "policy-two", Namespace: "default"}})
}

func TestEventHandler_WithPodInNamespace_DoesNotEnqueueAuthPoliciesFromOtherNamespaces(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicyInSameNamespace := &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "same-namespace-policy", Namespace: "default"},
	}
	authPolicyInOtherNamespace := &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "other-namespace-policy", Namespace: "other"},
	}
	k8sClient := createFakeClientForPodHandler(authPolicyInSameNamespace, authPolicyInOtherNamespace)
	h := pod.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
	}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: p}, queue)

	// 3. Assert
	requests := drainQueue(queue)
	assert.Len(t, requests, 1)
	assert.Contains(t, requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: "same-namespace-policy", Namespace: "default"}})
}

func createFakeClientForPodHandler(objects ...client.Object) client.Client {
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
