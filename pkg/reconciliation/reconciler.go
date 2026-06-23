package reconciliation

import (
	"context"

	"github.com/kartverket/ztoperator/internal/state"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ControllerResource interface {
	Reconcile(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme) (ctrl.Result, error)
	GetResourceKind() string
	GetResourceName() string
	IsResourceNil() bool
}

type ReconcilerAdapter[T client.Object] struct {
	Func ResourceReconciler[T]
}

type ResourceReconciler[T client.Object] struct {
	ResourceKind    string
	ResourceName    string
	DesiredResource *T
	Scope           *state.Scope
	ShouldUpdate    func(current T, desired T) bool
	UpdateFields    func(current T, desired T)
}

func CountNonNilResources(rfs []ControllerResource) int {
	count := 0
	for _, rf := range rfs {
		if !rf.IsResourceNil() {
			count++
		}
	}
	return count
}
