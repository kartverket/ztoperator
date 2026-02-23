package statusmanager_test

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/internal/statusmanager"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDetermineReconciliationState_WithInvalidConfig_ReturnsStateInvalid(t *testing.T) {
	// 1. Arrange
	scope := &state.Scope{
		InvalidConfig:          true,
		ValidationErrorMessage: helperfunctions.Ptr("Invalid configuration"),
		Descendants:            []state.Descendant[client.Object]{},
	}
	var reconcileActions []reconciliation.ReconcileAction

	// 2. Act
	result := statusmanager.DetermineReconciliationState(scope, reconcileActions)

	// 3. Assert
	assert.Equal(t, statusmanager.StateInvalid, result)
}

func TestDetermineReconciliationState_WithMissingDescendants_ReturnsStatePending(t *testing.T) {
	// 1. Arrange
	scope := &state.Scope{
		InvalidConfig: false,
		Descendants: []state.Descendant[client.Object]{
			{ID: "Secret-oauth-secret", Object: &v1.Secret{}},
		},
	}
	reconcileActions := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "oauth-secret", false),           // Exists
		createMockReconcileAction("RequestAuthentication", "my-auth", false), // Missing
	}

	// 2. Act
	result := statusmanager.DetermineReconciliationState(scope, reconcileActions)

	// 3. Assert
	assert.Equal(t, statusmanager.StatePending, result)
}

func TestDetermineReconciliationState_WithErrors_ReturnsStateFailed(t *testing.T) {
	// 1. Arrange
	errorMsg := "Failed to create resource"
	scope := &state.Scope{
		InvalidConfig: false,
		Descendants: []state.Descendant[client.Object]{
			{
				ID:           "Secret-oauth-secret",
				Object:       &v1.Secret{},
				ErrorMessage: &errorMsg,
			},
		},
	}
	reconcileActions := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "oauth-secret", false),
	}

	// 2. Act
	result := statusmanager.DetermineReconciliationState(scope, reconcileActions)

	// 3. Assert
	assert.Equal(t, statusmanager.StateFailed, result)
}

func TestDetermineReconciliationState_WithValidConfigAndDescendantsAndNoErrors_ReturnsStateReady(t *testing.T) {
	// 1. Arrange
	successMsg := "Created successfully"
	scope := &state.Scope{
		InvalidConfig: false,
		Descendants: []state.Descendant[client.Object]{
			{
				ID:             "Secret-oauth-secret",
				Object:         &v1.Secret{},
				SuccessMessage: &successMsg,
			},
		},
	}
	reconcileActions := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "oauth-secret", false),
	}

	// 2. Act
	result := statusmanager.DetermineReconciliationState(scope, reconcileActions)

	// 3. Assert
	assert.Equal(t, statusmanager.StateReady, result)
}

// Test UpdateAuthPolicyStatus

func TestDetermineReconciliationState_InvalidConfigTakesPrecedenceOverPendingAndFailed(t *testing.T) {
	// 1. Arrange - Invalid config + missing descendants + errors
	errorMsg := "Some error"
	scope := &state.Scope{
		InvalidConfig:          true,
		ValidationErrorMessage: helperfunctions.Ptr("Invalid configuration"),
		Descendants: []state.Descendant[client.Object]{
			{
				ID:           "Secret-oauth-secret",
				Object:       &v1.Secret{},
				ErrorMessage: &errorMsg,
			},
		},
	}
	reconcileActions := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "oauth-secret", false),
		createMockReconcileAction("RequestAuthentication", "my-auth", false),
	}

	// 2. Act
	result := statusmanager.DetermineReconciliationState(scope, reconcileActions)

	// 3. Assert
	assert.Equal(t, statusmanager.StateInvalid, result, "Invalid config should take precedence over other states")
}

func TestDetermineReconciliationState_PendingTakesPrecedenceOverFailed(t *testing.T) {
	// 1. Arrange - Valid config + missing descendants + errors
	errorMsg := "Some error"
	scope := &state.Scope{
		InvalidConfig: false,
		Descendants: []state.Descendant[client.Object]{
			{
				ID:           "Secret-oauth-secret",
				Object:       &v1.Secret{},
				ErrorMessage: &errorMsg,
			},
		},
	}
	reconcileActions := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "oauth-secret", false),           // Exists
		createMockReconcileAction("RequestAuthentication", "my-auth", false), // Missing
	}

	// 2. Act
	result := statusmanager.DetermineReconciliationState(scope, reconcileActions)

	// 3. Assert
	assert.Equal(t, statusmanager.StatePending, result, "Pending state should take precedence over failed state")
}

func TestUpdateAuthPolicyStatus_WithNoStatusChange_DoesNotUpdate(t *testing.T) {
	// 1. Arrange
	ctx := context.Background()
	authPolicy := createTestAuthPolicyForStatusManager()

	// Pre-build the expected conditions so they match what will be generated
	authPolicy.Status.Conditions = []metav1.Condition{
		{
			Type:               "AuthPolicy-test-policy",
			Status:             metav1.ConditionTrue,
			Reason:             "ReconciliationSuccess",
			Message:            "Descendants of AuthPolicy reconciled successfully.",
			LastTransitionTime: metav1.Now(),
		},
	}

	originalAuthPolicy := authPolicy.DeepCopy()

	scheme := runtime.NewScheme()
	_ = ztoperatorv1alpha1.AddToScheme(scheme)
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(authPolicy).
		WithStatusSubresource(authPolicy).
		Build()

	fakeRecorder := events.NewFakeRecorder(10)

	scope := &state.Scope{
		AuthPolicy:    *authPolicy,
		InvalidConfig: false,
		Descendants:   []state.Descendant[client.Object]{},
	}
	reconcileActions := []reconciliation.ReconcileAction{}

	// 2. Act
	statusmanager.UpdateAuthPolicyStatus(ctx, k8sClient, fakeRecorder, scope, originalAuthPolicy, reconcileActions)

	// 3. Assert
	// Verify no status update recordedEvents were recorded (only StatusUpdateStarted)
	close(fakeRecorder.Events)
	recordedEvents := make([]string, 0, len(fakeRecorder.Events))
	for event := range fakeRecorder.Events {
		recordedEvents = append(recordedEvents, event)
	}

	// Should only have StatusUpdateStarted event, no StatusUpdateSuccess
	assert.Len(t, recordedEvents, 1)
	assert.Contains(t, recordedEvents[0], "StatusUpdateStarted")
	assert.NotContains(t, recordedEvents[0], "StatusUpdateSuccess")
}

func TestUpdateAuthPolicyStatus_WithSuccessfulStatusChange_RecordsNormalEvent(t *testing.T) {
	// 1. Arrange
	ctx := context.Background()
	authPolicy := createTestAuthPolicyForStatusManager()
	authPolicy.Status.Phase = ztoperatorv1alpha1.PhasePending
	authPolicy.Status.Ready = false
	originalAuthPolicy := authPolicy.DeepCopy()

	scheme := runtime.NewScheme()
	_ = ztoperatorv1alpha1.AddToScheme(scheme)
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(authPolicy).
		WithStatusSubresource(authPolicy).
		Build()

	fakeRecorder := events.NewFakeRecorder(10)

	// Create scope that will result in Ready state
	successMsg := "Created"
	scope := &state.Scope{
		AuthPolicy:    *authPolicy,
		InvalidConfig: false,
		Descendants: []state.Descendant[client.Object]{
			{
				ID:             "Secret-test",
				Object:         &v1.Secret{},
				SuccessMessage: &successMsg,
			},
		},
	}
	reconcileActions := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "test", false),
	}

	// 2. Act
	statusmanager.UpdateAuthPolicyStatus(ctx, k8sClient, fakeRecorder, scope, originalAuthPolicy, reconcileActions)

	// 3. Assert
	close(fakeRecorder.Events)
	recordedEvents := make([]string, 0, len(fakeRecorder.Events))
	for event := range fakeRecorder.Events {
		recordedEvents = append(recordedEvents, event)
	}

	// Should have StatusUpdateStarted and StatusUpdateSuccess
	assert.Len(t, recordedEvents, 2)
	assert.Contains(t, recordedEvents[0], "Normal StatusUpdateStarted")
	assert.Contains(t, recordedEvents[1], "Normal StatusUpdateSuccess")

	// Verify status was actually updated
	updated := &ztoperatorv1alpha1.AuthPolicy{}
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(authPolicy), updated)
	require.NoError(t, err)
	assert.Equal(t, ztoperatorv1alpha1.PhaseReady, updated.Status.Phase)
	assert.True(t, updated.Status.Ready)
}

func TestUpdateAuthPolicyStatus_WithFailedStatusUpdate_RecordsWarningEvent(t *testing.T) {
	// 1. Arrange
	ctx := context.Background()
	authPolicy := createTestAuthPolicyForStatusManager()
	authPolicy.Status.Phase = ztoperatorv1alpha1.PhasePending
	originalAuthPolicy := authPolicy.DeepCopy()

	scheme := runtime.NewScheme()
	_ = ztoperatorv1alpha1.AddToScheme(scheme)
	// Create client WITHOUT the authPolicy to trigger update failure
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	fakeRecorder := events.NewFakeRecorder(10)

	scope := &state.Scope{
		AuthPolicy:    *authPolicy,
		InvalidConfig: false,
		Descendants:   []state.Descendant[client.Object]{},
	}
	reconcileActions := []reconciliation.ReconcileAction{}

	// 2. Act
	statusmanager.UpdateAuthPolicyStatus(ctx, k8sClient, fakeRecorder, scope, originalAuthPolicy, reconcileActions)

	// 3. Assert
	close(fakeRecorder.Events)
	recordedEvents := make([]string, 0, len(fakeRecorder.Events))
	for event := range fakeRecorder.Events {
		recordedEvents = append(recordedEvents, event)
	}

	// Should have StatusUpdateStarted and StatusUpdateFailed
	assert.Len(t, recordedEvents, 2)
	assert.Contains(t, recordedEvents[0], "Normal StatusUpdateStarted")
	assert.Contains(t, recordedEvents[1], "Warning StatusUpdateFailed")
}

func createTestAuthPolicyForStatusManager() *ztoperatorv1alpha1.AuthPolicy {
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
			Phase:              ztoperatorv1alpha1.PhaseReady,
			Ready:              true,
			Message:            "AuthPolicy ready.",
			ObservedGeneration: 1,
		},
	}
}
