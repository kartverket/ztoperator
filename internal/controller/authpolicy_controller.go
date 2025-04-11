package controller

import (
	"context"
	"fmt"
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/log"
	v3 "istio.io/api/security/v1"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	v1 "istio.io/client-go/pkg/apis/security/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

// AuthPolicyReconciler reconciles a AuthPolicy object
type AuthPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ztoperatorv1alpha1.AuthPolicy{}).
		Owns(&v1.RequestAuthentication{}).
		Owns(&v1.AuthorizationPolicy{}).
		Complete(r)
}

type reconcileFunc struct {
	ResourceKind string
	ResourceName string
	Func         func(context.Context, *scope, string, string) (ctrl.Result, error)
}

type scope struct {
	authPolicy  *ztoperatorv1alpha1.AuthPolicy
	descendants []descendant[client.Object]
}

type descendant[T client.Object] struct {
	ID             string
	Object         T
	ErrorMessage   *string
	SuccessMessage *string
}

// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies;requestauthentications,verbs=get;list;watch;create;update;patch;delete

func (r *AuthPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)

	authPolicy := new(ztoperatorv1alpha1.AuthPolicy)

	rLog.Info(fmt.Sprintf("Received reconcile request for AuthPolicy with name %s", req.NamespacedName.String()))

	if err := r.Client.Get(ctx, req.NamespacedName, authPolicy); err != nil {
		if apierrors.IsNotFound(err) {
			rLog.Debug(fmt.Sprintf("AuthPolicy with name %s not found. Probably a delete.", req.NamespacedName.String()))
			return reconcile.Result{}, nil
		}
		rLog.Error(err, fmt.Sprintf("Failed to get AuthPolicy with name %s", req.NamespacedName.String()))
		return reconcile.Result{}, err
	}

	r.Recorder.Eventf(authPolicy, "Normal", "ReconcileStarted", fmt.Sprintf("AuthPolicy with name %s started.", req.NamespacedName.String()))
	rLog.Debug(fmt.Sprintf("AuthPolicy with name %s found", req.NamespacedName.String()))

	authPolicy.InitializeStatus()
	originalAuthPolicy := authPolicy.DeepCopy()

	s := &scope{authPolicy: authPolicy}

	reconcileFuncs := []reconcileFunc{
		{
			ResourceKind: "RequestAuthentication",
			ResourceName: authPolicy.Name,
			Func:         r.reconcileRequestAuthentication,
		},
		{
			ResourceKind: "AuthorizationPolicy",
			ResourceName: authPolicy.Name + "-allow",
			Func:         r.reconcileIgnoreAuthAuthorizationPolicy,
		},
		{
			ResourceKind: "AuthorizationPolicy",
			ResourceName: authPolicy.Name + "-auth",
			Func:         r.reconcileRequireAuthAuthorizationPolicy,
		},
	}

	if !authPolicy.DeletionTimestamp.IsZero() {
		rLog.Info(fmt.Sprintf("Deleting AuthPolicy with name %s", req.NamespacedName.String()))
		return ctrl.Result{}, nil
	}

	defer func() {
		r.updateStatus(ctx, s, originalAuthPolicy, reconcileFuncs)
	}()

	return r.doReconcile(ctx, reconcileFuncs, s)
}

func (r *AuthPolicyReconciler) doReconcile(ctx context.Context, reconcileFuncs []reconcileFunc, s *scope) (ctrl.Result, error) {
	result := ctrl.Result{}
	var errs []error
	for _, rf := range reconcileFuncs {
		reconcileResult, err := rf.Func(ctx, s, rf.ResourceKind, rf.ResourceName)
		if err != nil {
			r.Recorder.Eventf(s.authPolicy, "Warning", fmt.Sprintf("%sReconcileFailed", rf.ResourceKind), fmt.Sprintf("%s with name %s failed during reconciliation.", rf.ResourceKind, rf.ResourceName))
			errs = append(errs, err)
		} else {
			r.Recorder.Eventf(s.authPolicy, "Normal", fmt.Sprintf("%sReconciledSuccessfully", rf.ResourceKind), fmt.Sprintf("%s with name %s reconciled successfully.", rf.ResourceKind, rf.ResourceName))
		}
		if len(errs) > 0 {
			continue
		}
		result = lowestNonZeroResult(result, reconcileResult)
	}

	if len(errs) > 0 {
		r.Recorder.Eventf(s.authPolicy, "Warning", "ReconcileFailed", "AuthPolicy failed during reconciliation")
		return ctrl.Result{}, errors.NewAggregate(errs)
	}
	r.Recorder.Eventf(s.authPolicy, "Normal", "ReconcileSuccess", "AuthPolicy reconciled successfully")
	return result, nil
}

func lowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.Requeue:
		return i
	case j.Requeue:
		return j
	case i.RequeueAfter < j.RequeueAfter:
		return i
	default:
		return j
	}
}

func (r *AuthPolicyReconciler) updateStatus(ctx context.Context, s *scope, original *ztoperatorv1alpha1.AuthPolicy, reconcileFuncs []reconcileFunc) {
	ap := s.authPolicy
	rLog := log.GetLogger(ctx)
	rLog.Debug(fmt.Sprintf("Updating AuthPolicy status for %s/%s", ap.Namespace, ap.Name))
	r.Recorder.Eventf(s.authPolicy, "Normal", "StatusUpdateStarted", "Status update of AuthPolicy started.")

	ap.Status.ObservedGeneration = ap.GetGeneration()
	authPolicyCondition := v2.Condition{
		Type:               GetID(strings.TrimPrefix(ap.Kind, "*"), ap.Name),
		LastTransitionTime: v2.Now(),
	}

	switch {
	case len(s.descendants) != len(reconcileFuncs):
		ap.Status.Phase = ztoperatorv1alpha1.PhasePending
		ap.Status.Ready = false
		ap.Status.Message = "AuthPolicy pending due to missing descendants."
		authPolicyCondition.Status = v2.ConditionUnknown
		authPolicyCondition.Reason = "ReconciliationPending"
		authPolicyCondition.Message = "Descendants of AuthPolicy are not yet reconciled."

	case len(s.GetErrors()) > 0:
		ap.Status.Phase = ztoperatorv1alpha1.PhaseFailed
		ap.Status.Ready = false
		ap.Status.Message = "AuthPolicy failed."
		authPolicyCondition.Status = v2.ConditionFalse
		authPolicyCondition.Reason = "ReconciliationFailed"
		authPolicyCondition.Message = "Descendants of AuthPolicy failed during reconciliation."

	default:
		ap.Status.Phase = ztoperatorv1alpha1.PhaseReady
		ap.Status.Ready = true
		ap.Status.Message = "AuthPolicy ready."
		authPolicyCondition.Status = v2.ConditionTrue
		authPolicyCondition.Reason = "ReconciliationSuccess"
		authPolicyCondition.Message = "Descendants of AuthPolicy reconciled successfully."
	}

	var conditions []v2.Condition
	descendantIDs := map[string]bool{}

	for _, d := range s.descendants {
		descendantIDs[d.ID] = true
		cond := v2.Condition{
			Type:               d.ID,
			LastTransitionTime: v2.Now(),
		}
		if d.ErrorMessage != nil {
			cond.Status = v2.ConditionFalse
			cond.Reason = "Error"
			cond.Message = *d.ErrorMessage
		} else if d.SuccessMessage != nil {
			cond.Status = v2.ConditionTrue
			cond.Reason = "Success"
			cond.Message = *d.SuccessMessage
		} else {
			cond.Status = v2.ConditionUnknown
			cond.Reason = "Unknown"
			cond.Message = "No status message set"
		}
		conditions = append(conditions, cond)
	}
	for _, rf := range reconcileFuncs {
		expectedID := GetID(rf.ResourceKind, rf.ResourceName)
		if !descendantIDs[expectedID] {
			conditions = append(conditions, v2.Condition{
				Type:               expectedID,
				Status:             v2.ConditionFalse,
				Reason:             "NotFound",
				Message:            fmt.Sprintf("Expected resource %s of kind %s was not created", rf.ResourceName, rf.ResourceKind),
				LastTransitionTime: v2.Now(),
			})
		}
	}

	ap.Status.Conditions = append([]v2.Condition{authPolicyCondition}, conditions...)

	if !equality.Semantic.DeepEqual(original.Status, ap.Status) {
		rLog.Debug(fmt.Sprintf("Updating AuthPolicy status with name %s/%s", ap.Namespace, ap.Name))
		if err := r.updateStatusWithRetriesOnConflict(ctx, ap); err != nil {
			rLog.Error(err, fmt.Sprintf("Failed to update AuthPolicy status with name %s/%s", ap.Namespace, ap.Name))
			r.Recorder.Eventf(s.authPolicy, "Warning", "StatusUpdateFailed", "Status update of AuthPolicy failed.")
		} else {
			r.Recorder.Eventf(s.authPolicy, "Normal", "StatusUpdateSuccess", "Status update of AuthPolicy updated successfully.")
		}
	}
}

func (r *AuthPolicyReconciler) updateStatusWithRetriesOnConflict(ctx context.Context, ap *ztoperatorv1alpha1.AuthPolicy) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &ztoperatorv1alpha1.AuthPolicy{}
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(ap), latest); err != nil {
			return err
		}
		latest.Status = ap.Status
		return r.Status().Update(ctx, latest)
	})
}

func (r *AuthPolicyReconciler) reconcileRequestAuthentication(ctx context.Context, scope *scope, resourceKind string, resourceName string) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	desired := &v1.RequestAuthentication{
		ObjectMeta: buildObjectMeta(resourceName, authPolicy.Namespace),
		Spec: v1beta1.RequestAuthentication{
			Selector: &v1beta2.WorkloadSelector{MatchLabels: authPolicy.Spec.Selector.MatchLabels},
			JwtRules: authPolicy.Spec.JWTRules.ToIstioRequestAuthenticationJWTRules(),
		},
	}

	return reconcileAuthPolicy[*v1.RequestAuthentication](
		ctx,
		r.Client,
		r.Scheme,
		scope,
		resourceKind,
		resourceName,
		desired,
		func(current, desired *v1.RequestAuthentication) bool {
			return !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) ||
				!reflect.DeepEqual(current.Spec.JwtRules, desired.Spec.JwtRules)
		},
		func(current, desired *v1.RequestAuthentication) {
			current.Spec.Selector = desired.Spec.Selector
			current.Spec.JwtRules = desired.Spec.JwtRules
		},
	)
}

func (r *AuthPolicyReconciler) reconcileIgnoreAuthAuthorizationPolicy(ctx context.Context, scope *scope, resourceKind string, resourceName string) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	desired := &v1.AuthorizationPolicy{
		ObjectMeta: buildObjectMeta(resourceName, authPolicy.Namespace),
		Spec: v1beta1.AuthorizationPolicy{
			Selector: &v1beta2.WorkloadSelector{MatchLabels: authPolicy.Spec.Selector.MatchLabels},
			Rules: []*v3.Rule{
				{
					To: []*v1beta1.Rule_To{
						{
							Operation: &v1beta1.Operation{
								Paths: []string{"*"},
							},
						},
					},
				},
			},
		},
	}

	return reconcileAuthPolicy[*v1.AuthorizationPolicy](
		ctx,
		r.Client,
		r.Scheme,
		scope,
		resourceKind,
		resourceName,
		desired,
		func(current, desired *v1.AuthorizationPolicy) bool {
			return !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) ||
				!reflect.DeepEqual(current.Spec.Rules, desired.Spec.Rules)
		},
		func(current, desired *v1.AuthorizationPolicy) {
			current.Spec.Selector = desired.Spec.Selector
			current.Spec.Rules = desired.Spec.Rules
		},
	)
}

func (r *AuthPolicyReconciler) reconcileRequireAuthAuthorizationPolicy(ctx context.Context, scope *scope, resourceKind string, resourceName string) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	desired := &v1.AuthorizationPolicy{
		ObjectMeta: buildObjectMeta(resourceName, authPolicy.Namespace),
		Spec: v1beta1.AuthorizationPolicy{
			Selector: &v1beta2.WorkloadSelector{MatchLabels: authPolicy.Spec.Selector.MatchLabels},
			Rules: []*v3.Rule{
				{
					To: []*v1beta1.Rule_To{
						{
							Operation: &v1beta1.Operation{
								Paths: []string{"*"},
							},
						},
					},
					When: []*v1beta1.Condition{
						{
							Key:    "request.auth.claims[iss]",
							Values: []string{"example-issuer"},
						},
					},
				},
			},
		},
	}

	return reconcileAuthPolicy[*v1.AuthorizationPolicy](
		ctx,
		r.Client,
		r.Scheme,
		scope,
		resourceKind,
		resourceName,
		desired,
		func(current, desired *v1.AuthorizationPolicy) bool {
			return !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) ||
				!reflect.DeepEqual(current.Spec.Rules, desired.Spec.Rules)
		},
		func(current, desired *v1.AuthorizationPolicy) {
			current.Spec.Selector = desired.Spec.Selector
			current.Spec.Rules = desired.Spec.Rules
		},
	)
}

func reconcileAuthPolicy[T client.Object](
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	scope *scope,
	resourceKind, resourceName string,
	desired T,
	shouldUpdate func(current, desired T) bool,
	updateFields func(current, desired T),
) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)
	kind := reflect.TypeOf(desired).Elem().Name()
	current := reflect.New(reflect.TypeOf(desired).Elem()).Interface().(T)

	rLog.Info(fmt.Sprintf("Trying to generate %s %s/%s", kind, desired.GetNamespace(), desired.GetName()))

	rLog.Debug(fmt.Sprintf("Checking if %s %s/%s exists", kind, desired.GetNamespace(), desired.GetName()))
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(desired), current)
	if apierrors.IsNotFound(err) {
		rLog.Debug(fmt.Sprintf("%s %s/%s does not exist", kind, desired.GetNamespace(), desired.GetName()))
		if err := ctrl.SetControllerReference(scope.authPolicy, desired, scheme); err != nil {
			errorReason := fmt.Sprintf("Unable to set AuthPolicy ownerReference on %s %s/%s.", kind, desired.GetNamespace(), desired.GetName())
			scope.ReplaceDescendant(desired, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}

		rLog.Info(fmt.Sprintf("Creating a %s %s/%s", kind, desired.GetNamespace(), desired.GetName()))
		if err := k8sClient.Create(ctx, desired); err != nil {
			errorReason := fmt.Sprintf("Unable to create %s %s/%s", kind, desired.GetNamespace(), desired.GetName())
			scope.ReplaceDescendant(desired, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}
		successMessage := fmt.Sprintf("Successfully created %s %s/%s.", kind, desired.GetNamespace(), desired.GetName())
		scope.ReplaceDescendant(desired, nil, &successMessage, resourceKind, resourceName)

		return ctrl.Result{}, nil
	}

	if err != nil {
		errorReason := fmt.Sprintf("Unable to get %s %s/%s.", kind, desired.GetNamespace(), desired.GetName())
		scope.ReplaceDescendant(desired, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, err
	}

	rLog.Debug(fmt.Sprintf("%s %s/%s exists", kind, desired.GetNamespace(), desired.GetName()))
	rLog.Debug(fmt.Sprintf("Determing if %s %s/%s should be updated", kind, desired.GetNamespace(), desired.GetName()))
	if shouldUpdate(current, desired) {
		rLog.Debug(fmt.Sprintf("Current %s %s/%s != desired", kind, desired.GetNamespace(), desired.GetName()))
		rLog.Debug(fmt.Sprintf("Updating current %s %s/%s with desired", kind, desired.GetNamespace(), desired.GetName()))
		updateFields(current, desired)

		if err := k8sClient.Update(ctx, current); err != nil {
			errorReason := fmt.Sprintf("Unable to update %s %s/%s.", kind, current.GetNamespace(), current.GetName())
			scope.ReplaceDescendant(current, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}

	} else {
		rLog.Debug(fmt.Sprintf("Current %s %s/%s == desired. No update needed.", kind, desired.GetNamespace(), desired.GetName()))
	}

	successMessage := fmt.Sprintf("Successfully generated %s %s/%s", kind, current.GetNamespace(), current.GetName())
	rLog.Info(successMessage)
	scope.ReplaceDescendant(current, nil, &successMessage, resourceKind, resourceName)

	return ctrl.Result{}, nil
}

func buildObjectMeta(name, namespace string) v2.ObjectMeta {
	return v2.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{"type": "ztoperator.kartverket.no"},
	}
}

func (s *scope) GetErrors() []string {
	var errs []string
	if s != nil {
		for _, d := range s.descendants {
			if d.ErrorMessage != nil {
				errs = append(errs, *d.ErrorMessage)
			}
		}
	}
	return errs
}

func (s *scope) ReplaceDescendant(obj client.Object, errorMessage *string, successMessage *string, resourceKind, resourceName string) {
	if s != nil {
		for i, d := range s.descendants {
			if reflect.TypeOf(d) == reflect.TypeOf(obj) && d.ID == obj.GetName() {
				s.descendants[i] = descendant[client.Object]{
					Object:         obj,
					ErrorMessage:   errorMessage,
					SuccessMessage: successMessage,
				}
				return
			}
		}
		s.descendants = append(s.descendants, descendant[client.Object]{
			ID:             GetID(resourceKind, resourceName),
			Object:         obj,
			ErrorMessage:   errorMessage,
			SuccessMessage: successMessage,
		})
	}
}

func GetID(resourceKind, resourceName string) string {
	return fmt.Sprintf("%s-%s", resourceKind, resourceName)
}
