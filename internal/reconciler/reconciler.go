package reconciler

import (
	"context"
	"reflect"

	"github.com/kartverket/ztoperator/pkg/reconciliation"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControllerResourceAdapter implements the reconciliation.ControllerResource interface by delegating to the generic
// reconciliation engine, which reconciles the resource towards its desired state while respecting AuthPolicy ownership.
type ControllerResourceAdapter[T client.Object] struct {
	reconciliation.ReconcilerAdapter[T]
}

func (a ControllerResourceAdapter[T]) Reconcile(
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
) (ctrl.Result, error) {
	return reconciliation.ReconcileControllerResource(
		ctx,
		k8sClient,
		scheme,
		a.Func.Scope,
		a.Func.ResourceKind,
		a.Func.ResourceName,
		a.Func.DesiredResource,
		a.Func.ShouldUpdate,
		a.Func.UpdateFields,
	)
}

func (a ControllerResourceAdapter[T]) GetResourceKind() string {
	return a.Func.ResourceKind
}

func (a ControllerResourceAdapter[T]) GetResourceName() string {
	return a.Func.ResourceName
}

func (a ControllerResourceAdapter[T]) IsResourceNil() bool {
	return a.Func.DesiredResource == nil || reflect.ValueOf(*a.Func.DesiredResource).IsNil()
}
