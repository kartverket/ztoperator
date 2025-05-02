package reconciliation

import (
	"context"
	runtime2 "github.com/kartverket/ztoperator/internal/state"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcileAction interface {
	Reconcile(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme) (ctrl.Result, error)
	GetResourceKind() string
	GetResourceName() string
}

type ReconcileFuncAdapter[T client.Object] struct {
	Func ReconcileFunc[T]
}

type ReconcileFunc[T client.Object] struct {
	ResourceKind    string
	ResourceName    string
	DesiredResource T
	Scope           *runtime2.Scope
	ShouldUpdate    func(current T, desired T) bool
	UpdateFields    func(current T, desired T)
}
