package statusmanager_test

import (
	"context"

	"github.com/kartverket/ztoperator/pkg/reconciliation"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// mockReconcileAction implements the reconciliation.ReconcileAction interface.
type mockReconcileAction struct {
	resourceKind string
	resourceName string
	isNil        bool
}

func (m *mockReconcileAction) GetResourceKind() string {
	return m.resourceKind
}

func (m *mockReconcileAction) GetResourceName() string {
	return m.resourceName
}

func (m *mockReconcileAction) IsResourceNil() bool {
	return m.isNil
}

func (m *mockReconcileAction) Reconcile(
	_ context.Context,
	_ client.Client,
	_ *runtime.Scheme,
) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func createMockReconcileAction(kind, name string, isNil bool) reconciliation.ReconcileAction {
	return &mockReconcileAction{
		resourceKind: kind,
		resourceName: name,
		isNil:        isNil,
	}
}
