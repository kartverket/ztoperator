package statusmanager

import (
	"context"
	"fmt"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconciliationState int

// NB: used in switch cases, ensure they are exhaustive.
const (
	StateInvalid ReconciliationState = iota
	StatePending
	StateFailed
	StateReady
)

// UpdateAuthPolicyStatus builds the new status, compares with the original, and updates if changed.
func UpdateAuthPolicyStatus(
	ctx context.Context,
	k8sClient client.Client,
	recorder record.EventRecorder,
	scope *state.Scope,
	originalAuthPolicy *ztoperatorv1alpha1.AuthPolicy,
	reconcileActions []reconciliation.ReconcileAction,
) {
	ap := &scope.AuthPolicy
	rLog := log.GetLogger(ctx)
	rLog.Debug(fmt.Sprintf("Updating AuthPolicy status for %s/%s", ap.Namespace, ap.Name))
	recorder.Eventf(ap, "Normal", "StatusUpdateStarted", "Status update of AuthPolicy started.")

	reconciliationState := DetermineReconciliationState(scope, reconcileActions)

	ap.Status.ObservedGeneration = ap.GetGeneration()
	ap.Status.Phase = determinePhase(reconciliationState)
	ap.Status.Ready = determineReadiness(reconciliationState)
	ap.Status.Message = statusMessage(reconciliationState, scope.ValidationErrorMessage)
	ap.Status.Conditions = BuildConditions(
		ap,
		reconciliationState,
		scope.ValidationErrorMessage,
		scope.Descendants,
		reconcileActions,
		originalAuthPolicy.Status.Conditions,
	)

	if !equality.Semantic.DeepEqual(originalAuthPolicy.Status, ap.Status) {
		rLog.Debug(fmt.Sprintf("Updating AuthPolicy status with name %s/%s", ap.Namespace, ap.Name))
		if err := UpdateStatus(ctx, k8sClient, *ap); err != nil {
			rLog.Error(
				err,
				fmt.Sprintf("Failed to update AuthPolicy status with name %s/%s", ap.Namespace, ap.Name),
			)
			recorder.Eventf(ap, "Warning", "StatusUpdateFailed", "Status update of AuthPolicy failed.")
		} else {
			recorder.Eventf(ap, "Normal", "StatusUpdateSuccess", "Status update of AuthPolicy updated successfully.")
		}
	}
}

func DetermineReconciliationState(
	scope *state.Scope,
	reconcileActions []reconciliation.ReconcileAction,
) ReconciliationState {
	switch {
	case scope.InvalidConfig:
		return StateInvalid
	case len(scope.Descendants) != reconciliation.CountReconciledResources(reconcileActions):
		return StatePending
	case len(scope.GetErrors()) > 0:
		return StateFailed
	default:
		return StateReady
	}
}

func determinePhase(state ReconciliationState) ztoperatorv1alpha1.Phase {
	switch state {
	case StateInvalid:
		return ztoperatorv1alpha1.PhaseInvalid
	case StatePending:
		return ztoperatorv1alpha1.PhasePending
	case StateFailed:
		return ztoperatorv1alpha1.PhaseFailed
	case StateReady:
		return ztoperatorv1alpha1.PhaseReady
	}
	panic("could not determine phase")
}

func determineReadiness(state ReconciliationState) bool {
	switch state {
	case StateInvalid, StatePending, StateFailed:
		return false
	case StateReady:
		return true
	}
	panic("could not determine readiness")
}

func statusMessage(state ReconciliationState, validationErrorMessage *string) string {
	switch state {
	case StateInvalid:
		return *validationErrorMessage
	case StatePending:
		return "AuthPolicy pending due to missing Descendants."
	case StateFailed:
		return "AuthPolicy failed."
	case StateReady:
		return "AuthPolicy ready."
	}
	panic("could not determine status message")
}
