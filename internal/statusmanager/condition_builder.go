package statusmanager

import (
	"fmt"
	"slices"
	"strings"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildConditions builds all conditions for the AuthPolicy status.
func BuildConditions(
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
	reconciliationState ReconciliationState,
	validationErrorMessage *string,
	descendants []state.Descendant[client.Object],
	reconcileFuncs []reconciliation.ReconcileAction,
	existingConditions []metav1.Condition,
) []metav1.Condition {
	authPolicyCondition := BuildAuthPolicyCondition(
		authPolicy,
		reconciliationState,
		validationErrorMessage,
		existingConditions,
	)
	descendantConditions := BuildDescendantConditions(descendants, existingConditions)
	missingResourceConditions := BuildMissingResourceConditions(descendants, reconcileFuncs, existingConditions)

	return slices.Concat([]metav1.Condition{authPolicyCondition}, descendantConditions, missingResourceConditions)
}

// BuildAuthPolicyCondition builds the AuthPolicy condition based on reconciliation state.
func BuildAuthPolicyCondition(
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
	reconciliationState ReconciliationState,
	validationErrorMessage *string,
	existingConditions []metav1.Condition,
) metav1.Condition {
	conditionType := state.GetID(strings.TrimPrefix(authPolicy.Kind, "*"), authPolicy.Name)

	condition := metav1.Condition{
		Type:               conditionType,
		LastTransitionTime: metav1.Now(),
	}

	switch reconciliationState {
	case StateInvalid:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "InvalidConfiguration"
		condition.Message = *validationErrorMessage

	case StatePending:
		condition.Status = metav1.ConditionUnknown
		condition.Reason = "ReconciliationPending"
		condition.Message = "Descendants of AuthPolicy are not yet reconciled."

	case StateFailed:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "ReconciliationFailed"
		condition.Message = "Descendants of AuthPolicy failed during reconciliation."

	case StateReady:
		condition.Status = metav1.ConditionTrue
		condition.Reason = "ReconciliationSuccess"
		condition.Message = "Descendants of AuthPolicy reconciled successfully."
	}

	return condition
}

// BuildDescendantConditions builds conditions for all descendants.
func BuildDescendantConditions(
	descendants []state.Descendant[client.Object],
	existingConditions []metav1.Condition,
) []metav1.Condition {
	var conditions []metav1.Condition

	for _, d := range descendants {
		condition := metav1.Condition{
			Type:               d.ID,
			LastTransitionTime: metav1.Now(),
		}

		switch {
		case d.ErrorMessage != nil:
			condition.Status = metav1.ConditionFalse
			condition.Reason = "Error"
			condition.Message = *d.ErrorMessage
		case d.SuccessMessage != nil:
			condition.Status = metav1.ConditionTrue
			condition.Reason = "Success"
			condition.Message = *d.SuccessMessage
		default:
			condition.Status = metav1.ConditionUnknown
			condition.Reason = "Unknown"
			condition.Message = "No status message set"
		}
		}

		conditions = append(conditions, condition)
	}

	return conditions
}

// BuildMissingResourceConditions builds conditions for resources that were expected but not found.
func BuildMissingResourceConditions(
	descendants []state.Descendant[client.Object],
	reconcileFuncs []reconciliation.ReconcileAction,
	existingConditions []metav1.Condition,
) []metav1.Condition {
	var conditions []metav1.Condition

	// Map of existing descendant IDs
	descendantIDs := make(map[string]bool)
	for _, d := range descendants {
		descendantIDs[d.ID] = true
	}

	for _, rf := range reconcileFuncs {
		if !rf.IsResourceNil() {
			expectedID := state.GetID(rf.GetResourceKind(), rf.GetResourceName())
			if !descendantIDs[expectedID] {
				condition := metav1.Condition{
					Type:   expectedID,
					Status: metav1.ConditionFalse,
					Reason: "NotFound",
					Message: fmt.Sprintf(
						"Expected resource %s of kind %s was not created",
						rf.GetResourceName(),
						rf.GetResourceKind(),
					),
					LastTransitionTime: metav1.Now(),
				}


				conditions = append(conditions, condition)
			}
		}
	}

	return conditions
}
