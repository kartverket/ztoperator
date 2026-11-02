package deployments_test

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/eventhandler/deployments"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestDeploymentEventHandler_WithNonDeploymentObject_ReturnsNoRequests(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	k8sClient := createFakeClientForDeploymentHandler()
	h := deployments.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	secret := &corev1.Secret{Name: "some-secret", Namespace: "default"}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: secret}, queue)

	// 3. Assert
	assert.Equal(t, 0, queue.Len(), "Expected no reconcile requests for non-deployment object")
}

func TestDeploymentEventHandler_WithDeploymentInNamespaceWithNoAuthPolicies_ReturnsNoRequests(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	k8sClient := createFakeClientForDeploymentHandler()
	h := deployments.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	deploy := &appsv1.Deployment{Name: "my-deployment", Namespace: "default"}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: deploy}, queue)

	// 3. Assert
	assert.Equal(t, 0, queue.Len(), "Expected no reconcile requests when no AuthPolicies exist in namespace")
}

func TestDeploymentEventHandler_WithDeploymentInNamespaceWithAuthPolicies_ReturnsRequestForEachAuthPolicy(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy1 := &ztoperatorv1alpha1.AuthPolicy{Name: "policy-one", Namespace: "default"}
	authPolicy2 := &ztoperatorv1alpha1.AuthPolicy{Name: "policy-two", Namespace: "default"}
	k8sClient := createFakeClientForDeploymentHandler(authPolicy1, authPolicy2)
	h := deployments.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	deploy := &appsv1.Deployment{Name: "my-deployment", Namespace: "default"}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: deploy}, queue)

	// 3. Assert
	requests := drainQueue(queue)
	assert.Len(t, requests, 2)
	assert.Contains(t, requests, reconcile.Request{Name: "policy-one", Namespace: "default"})
	assert.Contains(t, requests, reconcile.Request{Name: "policy-two", Namespace: "default"})
}

func TestDeploymentEventHandler_WithDeploymentInNamespace_DoesNotEnqueueAuthPoliciesFromOtherNamespaces(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicyInSameNamespace := &ztoperatorv1alpha1.AuthPolicy{Name: "same-namespace-policy", Namespace: "default"}
	authPolicyInOtherNamespace := &ztoperatorv1alpha1.AuthPolicy{Name: "other-namespace-policy", Namespace: "other"}
	k8sClient := createFakeClientForDeploymentHandler(authPolicyInSameNamespace, authPolicyInOtherNamespace)
	h := deployments.EventHandler(k8sClient)
	queue := workqueue.NewTypedRateLimitingQueue[reconcile.Request](workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer queue.ShutDown()

	deploy := &appsv1.Deployment{Name: "my-configmap", Namespace: "default"}

	// 2. Act
	h.Create(ctx, event.CreateEvent{Object: deploy}, queue)

	// 3. Assert
	requests := drainQueue(queue)
	assert.Len(t, requests, 1)
	assert.Contains(t, requests, reconcile.Request{Name: "same-namespace-policy", Namespace: "default"})
}

func createFakeClientForDeploymentHandler(objects ...client.Object) client.Client {
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
