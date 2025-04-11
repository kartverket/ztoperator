package controller

import (
	"context"
	"fmt"
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	v3 "istio.io/api/security/v1"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	v1 "istio.io/client-go/pkg/apis/security/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

// AuthPolicyReconciler reconciles a AuthPolicy object
type AuthPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies;requestauthentications,verbs=get;list;watch;create;update;patch;delete

func (r *AuthPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	authPolicy := new(ztoperatorv1alpha1.AuthPolicy)

	log.Info(fmt.Sprintf("Received reconcile request for AuthPolicy with name %s", req.NamespacedName.String()))

	if err := r.Client.Get(ctx, req.NamespacedName, authPolicy); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, fmt.Sprintf("AuthPolicy with name %s not found", req.NamespacedName.String()))
			return reconcile.Result{}, nil
		}
		log.Error(err, fmt.Sprintf("Failed to get AuthPolicy with name %s", req.NamespacedName.String()))
		return reconcile.Result{}, err
	}

	log.Info(fmt.Sprintf("AuthPolicy with name %s found", req.NamespacedName.String()))

	// TODO: Finalize object if it's being deleted
	// TODO: Add finalizers + r.Client.Update() if missing

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

	defer func() {
		r.updateStatus(ctx, s, originalAuthPolicy, reconcileFuncs)
	}()

	if !authPolicy.DeletionTimestamp.IsZero() {
		log.Info(fmt.Sprintf("Deleting AuthPolicy with name %s", req.NamespacedName.String()))
		reconcileFuncs = append(reconcileFuncs, reconcileFunc{
			Func: r.reconcileRequestAuthentication,
		})
	}
	return doReconcile(ctx, reconcileFuncs, s)
}

func doReconcile(ctx context.Context, reconcileFuncs []reconcileFunc, s *scope) (ctrl.Result, error) {
	result := ctrl.Result{}
	var errs []error
	for _, reconcileFunc := range reconcileFuncs {
		reconcileResult, err := reconcileFunc.Func(ctx, s, reconcileFunc.ResourceKind, reconcileFunc.ResourceName)
		if err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			continue
		}
		result = lowestNonZeroResult(result, reconcileResult)
	}

	if len(errs) > 0 {
		return ctrl.Result{}, errors.NewAggregate(errs)
	}
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
	log := ctrl.LoggerFrom(ctx)
	log.Info(fmt.Sprintf("Updating AuthPolicy status for %s/%s", ap.Namespace, ap.Name))

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
		log.Info(fmt.Sprintf("Patching AuthPolicy status with name %s/%s", ap.Namespace, ap.Name))
		//TODO: Hadde .Patch() her før, men den feilet grunnet status.Ready: null
		if err := r.Status().Update(ctx, ap); err != nil {
			log.Error(err, fmt.Sprintf("Failed to patch AuthPolicy status with name %s/%s", ap.Namespace, ap.Name))
		}
	}
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
				!reflect.DeepEqual(current.Spec.JwtRules, desired.Spec.JwtRules) ||
				!reflect.DeepEqual(current.Status.Conditions, desired.Status.Conditions) ||
				!reflect.DeepEqual(current.Status.ValidationMessages, desired.Status.ValidationMessages)
		},
		func(current, desired *v1.RequestAuthentication) {
			current.Spec.Selector = desired.Spec.Selector
			current.Spec.JwtRules = desired.Spec.JwtRules
			current.Status.Conditions = desired.Status.Conditions
			current.Status.ValidationMessages = desired.Status.ValidationMessages
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

	return reconcileAuthorizationPolicy(ctx, r.Client, r.Scheme, scope, desired, resourceKind, resourceName)
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

	return reconcileAuthorizationPolicy(ctx, r.Client, r.Scheme, scope, desired, resourceKind, resourceName)
}

func (r *AuthPolicyReconciler) reconcileDelete(ctx context.Context, scope *scope) (ctrl.Result, error) {
	authPolicy := scope.authPolicy

	resources := []client.Object{
		&v1.RequestAuthentication{
			ObjectMeta: buildObjectMeta(authPolicy.Name, authPolicy.Namespace),
		},
		&v1.AuthorizationPolicy{
			ObjectMeta: buildObjectMeta(authPolicy.Name, authPolicy.Namespace),
		},
	}

	var errs []error
	for _, resource := range resources {
		if err := r.Client.Delete(ctx, resource); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, err)
		}
	}

	// TODO: If we have finalizers, remove them (example: "ztoperator.kartverket.no/finalizer")
	if len(errs) > 0 {
		return ctrl.Result{}, errors.NewAggregate(errs)
	}
	return ctrl.Result{}, nil
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
	log := ctrl.LoggerFrom(ctx)
	kind := reflect.TypeOf(desired).Elem().Name()
	current := reflect.New(reflect.TypeOf(desired).Elem()).Interface().(T)

	log.Info(fmt.Sprintf("Generating %s %s/%s", kind, desired.GetNamespace(), desired.GetName()))

	log.Info(fmt.Sprintf("Checking if %s %s/%s exists", kind, desired.GetNamespace(), desired.GetName()))
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(desired), current)
	if apierrors.IsNotFound(err) {
		log.Info(fmt.Sprintf("%s %s/%s does not exist", kind, desired.GetNamespace(), desired.GetName()))
		if err := ctrl.SetControllerReference(scope.authPolicy, desired, scheme); err != nil {
			errorReason := fmt.Sprintf("Unable to set AuthPolicy ownerReference on %s %s/%s.", kind, desired.GetNamespace(), desired.GetName())
			scope.ReplaceDescendant(desired, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}

		log.Info(fmt.Sprintf("Creating a %s %s/%s", kind, desired.GetNamespace(), desired.GetName()))
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

	log.Info(fmt.Sprintf("%s %s/%s exists", kind, desired.GetNamespace(), desired.GetName()))
	log.Info(fmt.Sprintf("Determing if %s %s/%s should be updated", kind, desired.GetNamespace(), desired.GetName()))
	if shouldUpdate(current, desired) {
		log.Info(fmt.Sprintf("Current %s %s/%s != desired", kind, desired.GetNamespace(), desired.GetName()))
		log.Info(fmt.Sprintf("Updating current %s %s/%s with desired", kind, desired.GetNamespace(), desired.GetName()))
		updateFields(current, desired)
		if err := k8sClient.Update(ctx, current); err != nil {
			errorReason := fmt.Sprintf("Unable to update %s %s/%s.", kind, current.GetNamespace(), current.GetName())
			scope.ReplaceDescendant(current, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}
	} else {
		log.Info(fmt.Sprintf("Current %s %s/%s == desired. No update needed.", kind, desired.GetNamespace(), desired.GetName()))
	}

	successMessage := fmt.Sprintf("Successfully updated %s %s/%s", kind, current.GetNamespace(), current.GetName())
	scope.ReplaceDescendant(current, nil, &successMessage, resourceKind, resourceName)

	return ctrl.Result{}, nil
}

func reconcileAuthorizationPolicy(
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	scope *scope,
	desired *v1.AuthorizationPolicy,
	resourceKind, resourceName string,
) (ctrl.Result, error) {
	return reconcileAuthPolicy[*v1.AuthorizationPolicy](
		ctx,
		k8sClient,
		scheme,
		scope,
		resourceKind,
		resourceName,
		desired,
		func(current, desired *v1.AuthorizationPolicy) bool {
			return !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) ||
				!reflect.DeepEqual(current.Spec.Rules, desired.Spec.Rules) ||
				!reflect.DeepEqual(current.Status.Conditions, desired.Status.Conditions)
		},
		func(current, desired *v1.AuthorizationPolicy) {
			current.Spec.Selector = desired.Spec.Selector
			current.Spec.Rules = desired.Spec.Rules
			current.Status.Conditions = desired.Status.Conditions
		},
	)
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
