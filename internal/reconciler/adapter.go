package reconciler

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AuthPolicyAdapter implements the ReconcileAction interface.
type AuthPolicyAdapter[T client.Object] struct {
	reconciliation.ReconcileFuncAdapter[T]
}

func (a AuthPolicyAdapter[T]) Reconcile(
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
) (ctrl.Result, error) {
	return reconcileResource(
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

func (a AuthPolicyAdapter[T]) GetResourceKind() string {
	return a.Func.ResourceKind
}

func (a AuthPolicyAdapter[T]) GetResourceName() string {
	return a.Func.ResourceName
}

func (a AuthPolicyAdapter[T]) IsResourceNil() bool {
	return a.Func.DesiredResource == nil || reflect.ValueOf(*a.Func.DesiredResource).IsNil()
}

// reconcileResource handles the lifecycle of a Kubernetes resource:
// - Deletes it if desired is nil
// - Creates it if it doesn't exist
// - Updates it if it exists and differs from desired state.
func reconcileResource[T client.Object](
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	scope *state.Scope,
	resourceKind, resourceName string,
	desired *T,
	shouldUpdate func(current, desired T) bool,
	updateFields func(current, desired T),
) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)

	if desired == nil || reflect.ValueOf(*desired).IsNil() {
		return deleteResource[T](ctx, k8sClient, scope, resourceKind, resourceName)
	}

	deReferencedDesired := *desired

	kind := reflect.TypeOf(deReferencedDesired).Elem().Name()
	current, _ := reflect.New(reflect.TypeOf(deReferencedDesired).Elem()).Interface().(T)
	rLog.Info(
		fmt.Sprintf(
			"Trying to generate %s %s/%s",
			kind,
			deReferencedDesired.GetNamespace(),
			deReferencedDesired.GetName(),
		),
	)
	rLog.Debug(
		fmt.Sprintf(
			"Checking if %s %s/%s exists",
			kind,
			deReferencedDesired.GetNamespace(),
			deReferencedDesired.GetName(),
		),
	)

	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deReferencedDesired), current)
	if apierrors.IsNotFound(err) {
		return createResource(ctx, k8sClient, scheme, scope, deReferencedDesired, resourceKind, resourceName)
	}

	if err != nil {
		errorReason := fmt.Sprintf(
			"Unable to get %s %s/%s.",
			kind,
			deReferencedDesired.GetNamespace(),
			deReferencedDesired.GetName(),
		)
		scope.ReplaceDescendant(deReferencedDesired, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, err
	}

	rLog.Debug(fmt.Sprintf("%s %s/%s exists", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))
	rLog.Debug(
		fmt.Sprintf(
			"Determining if %s %s/%s should be updated",
			kind,
			deReferencedDesired.GetNamespace(),
			deReferencedDesired.GetName(),
		),
	)
	if shouldUpdate(current, deReferencedDesired) {
		rLog.Debug(
			fmt.Sprintf(
				"Current %s %s/%s != desired",
				kind,
				deReferencedDesired.GetNamespace(),
				deReferencedDesired.GetName(),
			),
		)
		rLog.Debug(
			fmt.Sprintf(
				"Updating current %s %s/%s with desired",
				kind,
				deReferencedDesired.GetNamespace(),
				deReferencedDesired.GetName(),
			),
		)
		updateFields(current, deReferencedDesired)

		if updateErr := k8sClient.Update(ctx, current); updateErr != nil {
			errorReason := fmt.Sprintf("Unable to update %s %s/%s.", kind, current.GetNamespace(), current.GetName())
			scope.ReplaceDescendant(current, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, updateErr
		}
	} else {
		rLog.Debug(
			fmt.Sprintf(
				"Current %s %s/%s == desired. No update needed.",
				kind,
				deReferencedDesired.GetNamespace(),
				deReferencedDesired.GetName(),
			),
		)
	}

	successMessage := fmt.Sprintf("Successfully generated %s %s/%s", kind, current.GetNamespace(), current.GetName())
	rLog.Info(successMessage)
	scope.ReplaceDescendant(current, nil, &successMessage, resourceKind, resourceName)

	return ctrl.Result{}, nil
}

func deleteResource[T client.Object](
	ctx context.Context,
	k8sClient client.Client,
	scope *state.Scope,
	resourceKind, resourceName string,
) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)

	resourceType := reflect.TypeOf((*T)(nil)).Elem()
	current, _ := reflect.New(resourceType.Elem()).Interface().(T)

	accessor := current
	accessor.SetNamespace(scope.AuthPolicy.Namespace)
	accessor.SetName(resourceName)

	rLog.Info(
		fmt.Sprintf(
			"Desired %s %s/%s is nil. Will try to delete it if it exist",
			resourceKind,
			accessor.GetNamespace(),
			accessor.GetName(),
		),
	)
	rLog.Debug(
		fmt.Sprintf("Checking if %s %s/%s exists", resourceKind, accessor.GetNamespace(), accessor.GetName()),
	)

	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(accessor), current)
	if err != nil {
		if apierrors.IsNotFound(err) {
			rLog.Debug(
				fmt.Sprintf("%s %s/%s already deleted", resourceKind, accessor.GetNamespace(), accessor.GetName()),
			)
			return ctrl.Result{}, nil
		}
		getErrorMessage := fmt.Sprintf(
			"Failed to get %s %s/%s when trying to delete it.",
			resourceKind,
			accessor.GetNamespace(),
			accessor.GetName(),
		)
		rLog.Error(err, getErrorMessage)
		scope.ReplaceDescendant(accessor, &getErrorMessage, nil, resourceKind, resourceName)
		return ctrl.Result{}, err
	}

	rLog.Info(
		fmt.Sprintf(
			"Deleting %s %s/%s as it's no longer desired",
			resourceKind,
			accessor.GetNamespace(),
			accessor.GetName(),
		),
	)
	if deleteErr := k8sClient.Delete(ctx, current); deleteErr != nil {
		deleteErrorMessage := fmt.Sprintf(
			"Failed to delete %s %s/%s",
			resourceKind,
			accessor.GetNamespace(),
			accessor.GetName(),
		)
		rLog.Error(deleteErr, deleteErrorMessage)
		scope.ReplaceDescendant(accessor, &deleteErrorMessage, nil, resourceKind, resourceName)
		return ctrl.Result{}, deleteErr
	}

	rLog.Debug(
		fmt.Sprintf("Successfully deleted %s %s/%s", resourceKind, accessor.GetNamespace(), accessor.GetName()),
	)
	successMsg := fmt.Sprintf(
		"Deleted %s %s/%s as it is no longer desired.",
		resourceKind,
		accessor.GetNamespace(),
		accessor.GetName(),
	)
	scope.ReplaceDescendant(accessor, nil, &successMsg, resourceKind, resourceName)
	return ctrl.Result{}, nil
}

func createResource[T client.Object](
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	scope *state.Scope,
	desired T,
	resourceKind, resourceName string,
) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)
	kind := reflect.TypeOf(desired).Elem().Name()

	rLog.Debug(
		fmt.Sprintf(
			"%s %s/%s does not exist",
			kind,
			desired.GetNamespace(),
			desired.GetName(),
		),
	)

	if controllerRefErr := ctrl.SetControllerReference(&scope.AuthPolicy, desired, scheme); controllerRefErr != nil {
		errorReason := fmt.Sprintf(
			"Unable to set AuthPolicy ownerReference on %s %s/%s.",
			kind,
			desired.GetNamespace(),
			desired.GetName(),
		)
		scope.ReplaceDescendant(desired, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, controllerRefErr
	}

	rLog.Info(
		fmt.Sprintf("Creating %s %s/%s", kind, desired.GetNamespace(), desired.GetName()),
	)
	if createErr := k8sClient.Create(ctx, desired); createErr != nil {
		errorReason := fmt.Sprintf(
			"Unable to create %s %s/%s",
			kind,
			desired.GetNamespace(),
			desired.GetName(),
		)
		scope.ReplaceDescendant(desired, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, createErr
	}

	successMessage := fmt.Sprintf(
		"Successfully created %s %s/%s.",
		kind,
		desired.GetNamespace(),
		desired.GetName(),
	)
	scope.ReplaceDescendant(desired, nil, &successMessage, resourceKind, resourceName)

	return ctrl.Result{}, nil
}
