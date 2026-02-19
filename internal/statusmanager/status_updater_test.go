package statusmanager_test

import (
	"context"
	"sync/atomic"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/statusmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestUpdateStatus_WithSuccessfulUpdate_UpdatesStatus(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createTestAuthPolicyWithStatusPending()

	// Set up fake client with the AuthPolicy already in the cluster
	scheme := runtime.NewScheme()
	_ = ztoperatorv1alpha1.AddToScheme(scheme)
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(authPolicy).
		WithStatusSubresource(authPolicy).
		Build()

	// Verify initial status in the cluster is Pending
	before := &ztoperatorv1alpha1.AuthPolicy{}
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(authPolicy), before)
	require.NoError(t, err)
	assert.Equal(t, ztoperatorv1alpha1.PhasePending, before.Status.Phase)
	assert.False(t, before.Status.Ready)

	// Modify the local copy to Ready status
	authPolicy.Status.Phase = ztoperatorv1alpha1.PhaseReady
	authPolicy.Status.Ready = true
	authPolicy.Status.Message = "Ready"

	// 2. Act
	err = statusmanager.UpdateStatus(ctx, k8sClient, *authPolicy)

	// 3. Assert
	require.NoError(t, err, "UpdateStatus should not return an error")

	// Verify status was updated in the cluster
	updated := &ztoperatorv1alpha1.AuthPolicy{}
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(authPolicy), updated)
	require.NoError(t, err)
	assert.Equal(t, ztoperatorv1alpha1.PhaseReady, updated.Status.Phase)
	assert.True(t, updated.Status.Ready)
	assert.Equal(t, "Ready", updated.Status.Message)
}

func TestUpdateStatus_WithNonExistentAuthPolicy_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createTestAuthPolicyWithStatusPending()

	scheme := runtime.NewScheme()
	_ = ztoperatorv1alpha1.AddToScheme(scheme)
	// Create client WITHOUT the authPolicy
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	authPolicy.Status.Phase = ztoperatorv1alpha1.PhaseReady

	// 2. Act
	err := statusmanager.UpdateStatus(ctx, k8sClient, *authPolicy)

	// 3. Assert
	require.Error(t, err, "UpdateStatus should return an error when AuthPolicy doesn't exist")
	assert.True(t, apierrors.IsNotFound(err), "Error should be NotFound")
}

func TestUpdateStatus_WithConflict_RetriesAndSucceeds(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createTestAuthPolicyWithStatusPending()

	scheme := runtime.NewScheme()
	_ = ztoperatorv1alpha1.AddToScheme(scheme)

	var updateCallCount atomic.Int32

	// Create client with interceptor that simulates conflict on first call
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(authPolicy).
		WithStatusSubresource(authPolicy).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				callNum := updateCallCount.Add(1)
				if callNum == 1 {
					// first call should fail
					return apierrors.NewConflict(
						schema.GroupResource{Group: "ztoperator.kartverket.no", Resource: "authpolicies"},
						obj.GetName(),
						nil,
					)
				}
				// subsequent calls should succeed
				return client.SubResource(subResourceName).Update(ctx, obj, opts...)
			},
		}).
		Build()

	authPolicy.Status.Phase = ztoperatorv1alpha1.PhaseReady

	// 2. Act
	err := statusmanager.UpdateStatus(ctx, k8sClient, *authPolicy)

	// 3. Assert
	require.NoError(t, err, "UpdateStatus should succeed after retry")
	assert.Equal(t, int32(2), updateCallCount.Load(), "Should have retried once after conflict")

	// Verify status was updated
	updated := &ztoperatorv1alpha1.AuthPolicy{}
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(authPolicy), updated)
	require.NoError(t, err)
	assert.Equal(t, ztoperatorv1alpha1.PhaseReady, updated.Status.Phase)
}

func createTestAuthPolicyWithStatusPending() *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-policy",
			Namespace:  "default",
			Generation: 1,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "AuthPolicy",
			APIVersion: "ztoperator.kartverket.no/v1alpha1",
		},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			WellKnownURI: "http://test-idp.example.com/.well-known/openid-configuration",
		},
		Status: ztoperatorv1alpha1.AuthPolicyStatus{
			Phase:              ztoperatorv1alpha1.PhasePending,
			Ready:              false,
			Message:            "Pending",
			ObservedGeneration: 1,
		},
	}
}
