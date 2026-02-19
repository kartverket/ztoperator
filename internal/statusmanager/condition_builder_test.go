package statusmanager_test

import (
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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBuildAuthPolicyCondition_WithInvalidState_ReturnsFalseCondition_WithErrorMessage(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy()
	validationError := "some validation error"

	// 2. Act
	condition := statusmanager.BuildAuthPolicyCondition(
		authPolicy,
		statusmanager.StateInvalid,
		&validationError,
		[]metav1.Condition{},
	)

	// 3. Assert
	assert.Equal(t, "AuthPolicy-test-policy", condition.Type)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, "InvalidConfiguration", condition.Reason)
	assert.Equal(t, validationError, condition.Message)
	assert.False(t, condition.LastTransitionTime.IsZero())
}

func TestBuildAuthPolicyCondition_WithPendingState_ReturnsUnknownCondition(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy()

	// 2. Act
	condition := statusmanager.BuildAuthPolicyCondition(
		authPolicy,
		statusmanager.StatePending,
		helperfunctions.Ptr("ignored"),
		[]metav1.Condition{},
	)

	// 3. Assert
	assert.Equal(t, "AuthPolicy-test-policy", condition.Type)
	assert.Equal(t, metav1.ConditionUnknown, condition.Status)
	assert.Equal(t, "ReconciliationPending", condition.Reason)
	assert.False(t, condition.LastTransitionTime.IsZero())
}

func TestBuildAuthPolicyCondition_WithFailedState_ReturnsFalseCondition(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy()

	// 2. Act
	condition := statusmanager.BuildAuthPolicyCondition(
		authPolicy,
		statusmanager.StateFailed,
		helperfunctions.Ptr("ignored"),
		[]metav1.Condition{},
	)

	// 3. Assert
	assert.Equal(t, "AuthPolicy-test-policy", condition.Type)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, "ReconciliationFailed", condition.Reason)
	assert.Equal(t, "Descendants of AuthPolicy failed during reconciliation.", condition.Message)
}

func TestBuildAuthPolicyCondition_WithReadyState_ReturnsTrueCondition(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy()

	// 2. Act
	condition := statusmanager.BuildAuthPolicyCondition(
		authPolicy,
		statusmanager.StateReady,
		helperfunctions.Ptr("ignored"),
		[]metav1.Condition{},
	)

	// 3. Assert
	assert.Equal(t, "AuthPolicy-test-policy", condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, "ReconciliationSuccess", condition.Reason)
	assert.False(t, condition.LastTransitionTime.IsZero())
}

func TestBuildAuthPolicyCondition_WithNoExistingConditions_SetsNewLastTransitionTime(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy()

	// 2. Act
	condition := statusmanager.BuildAuthPolicyCondition(
		authPolicy,
		statusmanager.StateReady,
		helperfunctions.Ptr("ignored"),
		[]metav1.Condition{},
	)

	// 3. Assert
	assert.Equal(t, "AuthPolicy-test-policy", condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.False(t, condition.LastTransitionTime.IsZero(), "LastTransitionTime should be set")
}

func TestBuildAuthPolicyCondition_WithIdenticalExistingCondition_PreservesLastTransitionTime(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy()
	existingCondition := statusmanager.BuildAuthPolicyCondition(
		authPolicy,
		statusmanager.StateReady,
		helperfunctions.Ptr("ignored"),
		[]metav1.Condition{},
	)

	// 2. Act
	condition := statusmanager.BuildAuthPolicyCondition(
		authPolicy,
		statusmanager.StateReady,
		helperfunctions.Ptr("ignored"),
		[]metav1.Condition{existingCondition},
	)

	// 3. Assert
	assert.Equal(t, "AuthPolicy-test-policy", condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, "ReconciliationSuccess", condition.Reason)
	assert.Equal(
		t,
		existingCondition.LastTransitionTime,
		condition.LastTransitionTime,
		"LastTransitionTime should be preserved from existing condition",
	)
}

func TestBuildAuthPolicyCondition_WithDifferentExistingCondition_UpdatesLastTransitionTime(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy()
	existingCondition := statusmanager.BuildAuthPolicyCondition(
		authPolicy,
		statusmanager.StateReady,
		helperfunctions.Ptr("ignored"),
		[]metav1.Condition{},
	)
	existingCondition.Message = "Different message to force condition change"

	// 2. Act
	condition := statusmanager.BuildAuthPolicyCondition(
		authPolicy,
		statusmanager.StateReady,
		helperfunctions.Ptr("ignored"),
		[]metav1.Condition{existingCondition},
	)

	// 3. Assert
	assert.Equal(t, "AuthPolicy-test-policy", condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, "ReconciliationSuccess", condition.Reason)
	assert.NotEqual(
		t,
		existingCondition.LastTransitionTime,
		condition.LastTransitionTime,
		"LastTransitionTime should be updated when condition changes",
	)
	assert.False(t, condition.LastTransitionTime.IsZero())
}

func TestBuildDescendantConditions_WithNoDescendants_ReturnsEmptySlice(t *testing.T) {
	// 1. Arrange
	descendants := []state.Descendant[client.Object]{}

	// 2. Act
	conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

	// 3. Assert
	assert.Empty(t, conditions)
}

func TestBuildDescendantConditions_WithSuccessfulDescendant_ReturnsTrueCondition(t *testing.T) {
	// 1. Arrange
	successMsg := "Resource created successfully"
	descendants := []state.Descendant[client.Object]{
		{
			ID:             "Secret-my-secret",
			Object:         &v1.Secret{},
			SuccessMessage: &successMsg,
		},
	}

	// 2. Act
	conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

	// 3. Assert
	require.Len(t, conditions, 1)
	assert.Equal(t, "Secret-my-secret", conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, conditions[0].Status)
	assert.Equal(t, "Success", conditions[0].Reason)
	assert.Equal(t, successMsg, conditions[0].Message)
	assert.False(t, conditions[0].LastTransitionTime.IsZero())
}

func TestBuildDescendantConditions_WithFailedDescendant_ReturnsFalseCondition(t *testing.T) {
	// 1. Arrange
	errorMsg := "Failed to create resource"
	descendants := []state.Descendant[client.Object]{
		{
			ID:           "Secret-my-secret",
			Object:       &v1.Secret{},
			ErrorMessage: &errorMsg,
		},
	}

	// 2. Act
	conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

	// 3. Assert
	require.Len(t, conditions, 1)
	assert.Equal(t, "Secret-my-secret", conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, conditions[0].Status)
	assert.Equal(t, "Error", conditions[0].Reason)
	assert.Equal(t, errorMsg, conditions[0].Message)
	assert.False(t, conditions[0].LastTransitionTime.IsZero())
}

func TestBuildDescendantConditions_WithDescendantWithoutMessage_ReturnsUnknownCondition(t *testing.T) {
	// 1. Arrange
	descendants := []state.Descendant[client.Object]{
		{
			ID:     "Secret-my-secret",
			Object: &v1.Secret{},
		},
	}

	// 2. Act
	conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

	// 3. Assert
	require.Len(t, conditions, 1)
	assert.Equal(t, "Secret-my-secret", conditions[0].Type)
	assert.Equal(t, metav1.ConditionUnknown, conditions[0].Status)
	assert.Equal(t, "Unknown", conditions[0].Reason)
	assert.Equal(t, "No status message set", conditions[0].Message)
	assert.False(t, conditions[0].LastTransitionTime.IsZero())
}

func TestBuildDescendantConditions_WithMultipleDescendants_ReturnsAllConditions(t *testing.T) {
	// 1. Arrange
	successMsg := "Created"
	errorMsg := "Failed"
	descendants := []state.Descendant[client.Object]{
		{
			ID:             "Secret-secret-1",
			Object:         &v1.Secret{},
			SuccessMessage: &successMsg,
		},
		{
			ID:           "Secret-secret-2",
			Object:       &v1.Secret{},
			ErrorMessage: &errorMsg,
		},
		{
			ID:     "ConfigMap-config-1",
			Object: &v1.ConfigMap{},
		},
	}

	// 2. Act
	conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

	// 3. Assert
	require.Len(t, conditions, 3)
	assert.Equal(t, metav1.ConditionTrue, conditions[0].Status)
	assert.Equal(t, metav1.ConditionFalse, conditions[1].Status)
	assert.Equal(t, metav1.ConditionUnknown, conditions[2].Status)
}

func TestBuildDescendantConditions_WithNoExistingConditions_SetsNewLastTransitionTime(t *testing.T) {
	// 1. Arrange
	successMsg := "Created successfully"
	descendants := []state.Descendant[client.Object]{
		{
			ID:             "Secret-my-secret",
			Object:         &v1.Secret{},
			SuccessMessage: &successMsg,
		},
	}

	// 2. Act
	conditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})

	// 3. Assert
	require.Len(t, conditions, 1)
	assert.Equal(t, "Secret-my-secret", conditions[0].Type)
	assert.False(t, conditions[0].LastTransitionTime.IsZero(), "LastTransitionTime should be set")
}

func TestBuildDescendantConditions_WithIdenticalExistingCondition_PreservesLastTransitionTime(t *testing.T) {
	// 1. Arrange
	successMsg := "Created successfully"
	descendants := []state.Descendant[client.Object]{
		{
			ID:             "Secret-my-secret",
			Object:         &v1.Secret{},
			SuccessMessage: &successMsg,
		},
	}

	existingConditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})
	require.Len(t, existingConditions, 1)

	// 2. Act
	conditions := statusmanager.BuildDescendantConditions(descendants, existingConditions)

	// 3. Assert
	require.Len(t, conditions, 1)
	assert.Equal(t, "Secret-my-secret", conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, conditions[0].Status)
	assert.Equal(
		t,
		existingConditions[0].LastTransitionTime,
		conditions[0].LastTransitionTime,
		"LastTransitionTime should be preserved from existing condition",
	)
	assert.False(t, conditions[0].LastTransitionTime.IsZero(), "LastTransitionTime should be set")
}

func TestBuildDescendantConditions_WithDifferentExistingCondition_UpdatesLastTransitionTime(t *testing.T) {
	// 1. Arrange
	successMsg := "Created successfully"
	descendants := []state.Descendant[client.Object]{
		{
			ID:             "Secret-my-secret",
			Object:         &v1.Secret{},
			SuccessMessage: &successMsg,
		},
	}

	existingConditions := statusmanager.BuildDescendantConditions(descendants, []metav1.Condition{})
	require.Len(t, existingConditions, 1)
	existingConditions[0].Message = "Different message to force condition change"

	// 2. Act
	conditions := statusmanager.BuildDescendantConditions(descendants, existingConditions)

	// 3. Assert
	require.Len(t, conditions, 1)
	assert.Equal(t, "Secret-my-secret", conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, conditions[0].Status)
	assert.NotEqual(
		t,
		existingConditions[0].LastTransitionTime,
		conditions[0].LastTransitionTime,
		"LastTransitionTime should be updated when condition changes",
	)
	assert.False(t, conditions[0].LastTransitionTime.IsZero(), "LastTransitionTime should be set")
}

func TestBuildMissingResourceConditions_WithNoMissingResources_ReturnsEmptySlice(t *testing.T) {
	// 1. Arrange
	descendants := []state.Descendant[client.Object]{
		{ID: "Secret-my-secret", Object: &v1.Secret{}},
	}
	reconcileFuncs := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "my-secret", false),
	}

	// 2. Act
	conditions := statusmanager.BuildMissingResourceConditions(descendants, reconcileFuncs, []metav1.Condition{})

	// 3. Assert
	assert.Empty(t, conditions)
}

func TestBuildMissingResourceConditions_WithMissingResource_ReturnsFalseCondition(t *testing.T) {
	// 1. Arrange
	descendants := []state.Descendant[client.Object]{}
	reconcileFuncs := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "expected-secret", false),
	}

	// 2. Act
	conditions := statusmanager.BuildMissingResourceConditions(descendants, reconcileFuncs, []metav1.Condition{})

	// 3. Assert
	require.Len(t, conditions, 1)
	assert.Equal(t, "Secret-expected-secret", conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, conditions[0].Status)
	assert.Equal(t, "NotFound", conditions[0].Reason)
	assert.False(t, conditions[0].LastTransitionTime.IsZero())
}

func TestBuildMissingResourceConditions_WithNilResource_IgnoresIt(t *testing.T) {
	// 1. Arrange
	descendants := []state.Descendant[client.Object]{}
	reconcileFuncs := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "some-secret", true),
	}

	// 2. Act
	conditions := statusmanager.BuildMissingResourceConditions(descendants, reconcileFuncs, []metav1.Condition{})

	// 3. Assert
	assert.Empty(t, conditions, "Nil resources should be ignored")
}

func TestBuildMissingResourceConditions_WithPartiallyMissingResources_ReturnsOnlyMissing(t *testing.T) {
	// 1. Arrange
	descendants := []state.Descendant[client.Object]{
		{ID: "Secret-present-secret", Object: &v1.Secret{}},
	}
	reconcileFuncs := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "present-secret", false),
		createMockReconcileAction("ConfigMap", "missing-config", false),
		createMockReconcileAction("Secret", "missing-secret", false),
	}

	// 2. Act
	conditions := statusmanager.BuildMissingResourceConditions(descendants, reconcileFuncs, []metav1.Condition{})

	// 3. Assert
	require.Len(t, conditions, 2)
	assert.Equal(t, "ConfigMap-missing-config", conditions[0].Type)
	assert.Equal(t, "Secret-missing-secret", conditions[1].Type)
}

func TestBuildMissingResourceConditions_WithNoExistingConditions_SetsNewLastTransitionTime(t *testing.T) {
	// 1. Arrange
	var descendants []state.Descendant[client.Object]
	reconcileFuncs := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "expected-secret", false),
	}

	// 2. Act
	conditions := statusmanager.BuildMissingResourceConditions(descendants, reconcileFuncs, []metav1.Condition{})

	// 3. Assert
	require.Len(t, conditions, 1)
	assert.Equal(t, "Secret-expected-secret", conditions[0].Type)
	assert.False(t, conditions[0].LastTransitionTime.IsZero(), "LastTransitionTime should be set")
}

func TestBuildMissingResourceConditions_WithIdenticalExistingCondition_PreservesLastTransitionTime(t *testing.T) {
	// 1. Arrange
	var descendants []state.Descendant[client.Object]
	reconcileFuncs := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "expected-secret", false),
	}

	existingConditions := statusmanager.BuildMissingResourceConditions(
		descendants,
		reconcileFuncs,
		[]metav1.Condition{},
	)
	require.Len(t, existingConditions, 1)

	// 2. Act - rebuild with same data
	conditions := statusmanager.BuildMissingResourceConditions(descendants, reconcileFuncs, existingConditions)

	// 3. Assert
	require.Len(t, conditions, 1)
	assert.Equal(t, "Secret-expected-secret", conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, conditions[0].Status)
	assert.Equal(
		t,
		existingConditions[0].LastTransitionTime,
		conditions[0].LastTransitionTime,
		"LastTransitionTime should be preserved from existing condition",
	)
	assert.False(t, conditions[0].LastTransitionTime.IsZero(), "LastTransitionTime should be set")
}

func TestBuildMissingResourceConditions_WithDifferentExistingCondition_UpdatesLastTransitionTime(t *testing.T) {
	// 1. Arrange
	var descendants []state.Descendant[client.Object]
	reconcileFuncs := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "expected-secret", false),
	}

	existingConditions := statusmanager.BuildMissingResourceConditions(
		descendants,
		reconcileFuncs,
		[]metav1.Condition{},
	)
	require.Len(t, existingConditions, 1)
	existingConditions[0].Message = "Different message to force condition change"

	// 2. Act - rebuild with different data
	conditions := statusmanager.BuildMissingResourceConditions(descendants, reconcileFuncs, existingConditions)

	// 3. Assert
	require.Len(t, conditions, 1)
	assert.Equal(t, "Secret-expected-secret", conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, conditions[0].Status)
	assert.NotEqual(
		t,
		existingConditions[0].LastTransitionTime,
		conditions[0].LastTransitionTime,
		"LastTransitionTime should be updated when condition changes",
	)
	assert.False(t, conditions[0].LastTransitionTime.IsZero(), "LastTransitionTime should be set")
}

func TestBuildConditions_IncludesAllConditionTypes(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy()
	reconciliationState := statusmanager.StateReady

	successMsg := "Created successfully"
	descendants := []state.Descendant[client.Object]{
		{
			ID:             "Secret-oauth-secret",
			Object:         &v1.Secret{},
			SuccessMessage: &successMsg,
		},
	}

	reconcileFuncs := []reconciliation.ReconcileAction{
		createMockReconcileAction("Secret", "oauth-secret", false),
		createMockReconcileAction("RequestAuthentication", "my-auth", false),
	}

	// 2. Act
	conditions := statusmanager.BuildConditions(
		authPolicy,
		reconciliationState,
		helperfunctions.Ptr("ignored"),
		descendants,
		reconcileFuncs,
		[]metav1.Condition{},
	)

	// 3. Assert
	require.Len(t, conditions, 3, "Should have AuthPolicy + descendant + missing resource")

	// First condition should be AuthPolicy
	assert.Equal(t, "AuthPolicy-test-policy", conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, conditions[0].Status)

	// Second should be descendant
	assert.Equal(t, "Secret-oauth-secret", conditions[1].Type)
	assert.Equal(t, metav1.ConditionTrue, conditions[1].Status)

	// Third should be missing resource
	assert.Equal(t, "RequestAuthentication-my-auth", conditions[2].Type)
	assert.Equal(t, metav1.ConditionFalse, conditions[2].Status)
	assert.Equal(t, "NotFound", conditions[2].Reason)
}

func createTestAuthPolicy() *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "AuthPolicy",
		},
	}
}
