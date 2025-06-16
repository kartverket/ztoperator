package controller

import (
	"bytes"
	"context"
	"fmt"
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy/deny"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy/ignore_auth"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy/require_auth"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/config_patch"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/requestauthentication"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/secret"
	"github.com/kartverket/ztoperator/pkg/rest"
	"github.com/kartverket/ztoperator/pkg/utils"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

type AuthPolicyAdapter[T client.Object] struct {
	reconciliation.ReconcileFuncAdapter[T]
}

func (a AuthPolicyAdapter[T]) Reconcile(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme) (ctrl.Result, error) {
	return reconcileAuthPolicy(
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

// SetupWithManager sets up the controller with the Manager.
func (r *AuthPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ztoperatorv1alpha1.AuthPolicy{}).
		Owns(&istioclientsecurityv1.RequestAuthentication{}).
		Owns(&istioclientsecurityv1.AuthorizationPolicy{}).
		Owns(&v1alpha4.EnvoyFilter{}).
		Owns(&v1.Secret{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies;requestauthentications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=envoyfilters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

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

	if !authPolicy.DeletionTimestamp.IsZero() {
		rLog.Info(fmt.Sprintf("Deleting AuthPolicy with name %s", req.NamespacedName.String()))
		return ctrl.Result{}, nil
	}

	scope, err := resolveAuthPolicy(ctx, r.Client, authPolicy)
	if err != nil {
		rLog.Error(err, fmt.Sprintf("Failed to resolve AuthPolicy with name %s", req.NamespacedName.String()))
		return reconcile.Result{}, err
	}

	if err := utils.ValidatePaths(authPolicy.GetPaths()); err != nil {
		rLog.Error(err, fmt.Sprintf("Path validation failed for AuthPolicy with name %s", req.NamespacedName.String()))
		rLog.Debug(fmt.Sprintf("Path validation failed for AuthPolicy with name %s. Defaulting to default deny on all paths.", req.NamespacedName.String()))
		scope.InvalidConfig = true
		pathValidationError := err.Error()
		scope.ValidationErrorMessage = &pathValidationError
	} else {
		scope.InvalidConfig = false
	}

	autoLoginSecretName := authPolicy.Name + "-envoy-secret"
	autoLoginEnvoyFilter := authPolicy.Name + "-login"
	requestAuthenticationName := authPolicy.Name
	denyAuthorizationPolicyName := authPolicy.Name + "-deny-auth-rules"
	ignoreAuthAuthorizationPolicyName := authPolicy.Name + "-ignore-auth"
	requireAuthAuthorizationPolicyName := authPolicy.Name + "-require-auth"

	reconcileFuncs := []reconciliation.ReconcileAction{
		AuthPolicyAdapter[*v1.Secret]{
			reconciliation.ReconcileFuncAdapter[*v1.Secret]{
				Func: reconciliation.ReconcileFunc[*v1.Secret]{
					ResourceKind:    "Secret",
					ResourceName:    autoLoginSecretName,
					DesiredResource: utils.Ptr(secret.GetDesired(scope, utils.BuildObjectMeta(autoLoginSecretName, authPolicy.Namespace))),
					Scope:           scope,
					ShouldUpdate: func(current, desired *v1.Secret) bool {
						desiredTokenSecret, hasDesired := desired.Data[config_patch.TokenSecretFileName]
						currentTokenSecret, hasCurrent := current.Data[config_patch.TokenSecretFileName]
						return !hasDesired || !hasCurrent || !bytes.Equal(currentTokenSecret, desiredTokenSecret)
					},
					UpdateFields: func(current, desired *v1.Secret) {
						current.Data = desired.Data
					},
				},
			},
		},
		AuthPolicyAdapter[*v1alpha4.EnvoyFilter]{
			reconciliation.ReconcileFuncAdapter[*v1alpha4.EnvoyFilter]{
				Func: reconciliation.ReconcileFunc[*v1alpha4.EnvoyFilter]{
					ResourceKind:    "EnvoyFilter",
					ResourceName:    autoLoginEnvoyFilter,
					DesiredResource: utils.Ptr(envoyfilter.GetDesired(scope, utils.BuildObjectMeta(autoLoginEnvoyFilter, authPolicy.Namespace))),
					Scope:           scope,
					ShouldUpdate: func(current, desired *v1alpha4.EnvoyFilter) bool {
						return !reflect.DeepEqual(current.Spec.WorkloadSelector, desired.Spec.WorkloadSelector) ||
							!reflect.DeepEqual(current.Spec.ConfigPatches, desired.Spec.ConfigPatches)
					},
					UpdateFields: func(current, desired *v1alpha4.EnvoyFilter) {
						current.Spec.WorkloadSelector = desired.Spec.WorkloadSelector
						current.Spec.ConfigPatches = desired.Spec.ConfigPatches
					},
				},
			},
		},
		AuthPolicyAdapter[*istioclientsecurityv1.RequestAuthentication]{
			reconciliation.ReconcileFuncAdapter[*istioclientsecurityv1.RequestAuthentication]{
				Func: reconciliation.ReconcileFunc[*istioclientsecurityv1.RequestAuthentication]{
					ResourceKind:    "RequestAuthentication",
					ResourceName:    requestAuthenticationName,
					DesiredResource: utils.Ptr(requestauthentication.GetDesired(scope, utils.BuildObjectMeta(requestAuthenticationName, authPolicy.Namespace))),
					Scope:           scope,
					ShouldUpdate: func(current, desired *istioclientsecurityv1.RequestAuthentication) bool {
						return !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) ||
							!reflect.DeepEqual(current.Spec.JwtRules, desired.Spec.JwtRules)
					},
					UpdateFields: func(current, desired *istioclientsecurityv1.RequestAuthentication) {
						current.Spec.Selector = desired.Spec.Selector
						current.Spec.JwtRules = desired.Spec.JwtRules
					},
				},
			},
		},
		AuthPolicyAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
			reconciliation.ReconcileFuncAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
				Func: reconciliation.ReconcileFunc[*istioclientsecurityv1.AuthorizationPolicy]{
					ResourceKind:    "AuthorizationPolicy",
					ResourceName:    denyAuthorizationPolicyName,
					DesiredResource: utils.Ptr(deny.GetDesired(scope, utils.BuildObjectMeta(denyAuthorizationPolicyName, authPolicy.Namespace))),
					Scope:           scope,
					ShouldUpdate: func(current, desired *istioclientsecurityv1.AuthorizationPolicy) bool {
						return !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) ||
							!reflect.DeepEqual(current.Spec.Rules, desired.Spec.Rules)
					},
					UpdateFields: func(current, desired *istioclientsecurityv1.AuthorizationPolicy) {
						current.Spec.Selector = desired.Spec.Selector
						current.Spec.Rules = desired.Spec.Rules
					},
				},
			},
		},
		AuthPolicyAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
			reconciliation.ReconcileFuncAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
				Func: reconciliation.ReconcileFunc[*istioclientsecurityv1.AuthorizationPolicy]{
					ResourceKind:    "AuthorizationPolicy",
					ResourceName:    ignoreAuthAuthorizationPolicyName,
					DesiredResource: utils.Ptr(ignore_auth.GetDesired(scope, utils.BuildObjectMeta(ignoreAuthAuthorizationPolicyName, authPolicy.Namespace))),
					Scope:           scope,
					ShouldUpdate: func(current, desired *istioclientsecurityv1.AuthorizationPolicy) bool {
						return !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) ||
							!reflect.DeepEqual(current.Spec.Rules, desired.Spec.Rules)
					},
					UpdateFields: func(current, desired *istioclientsecurityv1.AuthorizationPolicy) {
						current.Spec.Selector = desired.Spec.Selector
						current.Spec.Rules = desired.Spec.Rules
					},
				},
			},
		},
		AuthPolicyAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
			reconciliation.ReconcileFuncAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
				Func: reconciliation.ReconcileFunc[*istioclientsecurityv1.AuthorizationPolicy]{
					ResourceKind:    "AuthorizationPolicy",
					ResourceName:    requireAuthAuthorizationPolicyName,
					DesiredResource: utils.Ptr(require_auth.GetDesired(scope, utils.BuildObjectMeta(requireAuthAuthorizationPolicyName, authPolicy.Namespace))),
					Scope:           scope,
					ShouldUpdate: func(current, desired *istioclientsecurityv1.AuthorizationPolicy) bool {
						return !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) ||
							!reflect.DeepEqual(current.Spec.Rules, desired.Spec.Rules)
					},
					UpdateFields: func(current, desired *istioclientsecurityv1.AuthorizationPolicy) {
						current.Spec.Selector = desired.Spec.Selector
						current.Spec.Rules = desired.Spec.Rules
					},
				},
			},
		},
	}

	defer func() {
		r.updateStatus(ctx, scope, originalAuthPolicy, reconcileFuncs)
	}()

	return r.doReconcile(ctx, reconcileFuncs, scope)
}

func (r *AuthPolicyReconciler) doReconcile(ctx context.Context, reconcileFuncs []reconciliation.ReconcileAction, scope *state.Scope) (ctrl.Result, error) {
	result := ctrl.Result{}
	var errs []error
	for _, rf := range reconcileFuncs {
		reconcileResult, err := rf.Reconcile(ctx, r.Client, r.Scheme)
		if err != nil {
			r.Recorder.Eventf(&scope.AuthPolicy, "Warning", fmt.Sprintf("%sReconcileFailed", rf.GetResourceKind()), fmt.Sprintf("%s with name %s failed during reconciliation.", rf.GetResourceKind(), rf.GetResourceName()))
			errs = append(errs, err)
		} else {
			r.Recorder.Eventf(&scope.AuthPolicy, "Normal", fmt.Sprintf("%sReconciledSuccessfully", rf.GetResourceKind()), fmt.Sprintf("%s with name %s reconciled successfully.", rf.GetResourceKind(), rf.GetResourceName()))
		}
		if len(errs) > 0 {
			continue
		}
		result = utils.LowestNonZeroResult(result, reconcileResult)
	}

	if len(errs) > 0 {
		r.Recorder.Eventf(&scope.AuthPolicy, "Warning", "ReconcileFailed", "AuthPolicy failed during reconciliation")
		return ctrl.Result{}, errors.NewAggregate(errs)
	}
	r.Recorder.Eventf(&scope.AuthPolicy, "Normal", "ReconcileSuccess", "AuthPolicy reconciled successfully")
	return result, nil
}

func (r *AuthPolicyReconciler) updateStatus(ctx context.Context, scope *state.Scope, original *ztoperatorv1alpha1.AuthPolicy, reconcileFuncs []reconciliation.ReconcileAction) {
	ap := scope.AuthPolicy
	rLog := log.GetLogger(ctx)
	rLog.Debug(fmt.Sprintf("Updating AuthPolicy status for %s/%s", ap.Namespace, ap.Name))
	r.Recorder.Eventf(&ap, "Normal", "StatusUpdateStarted", "Status update of AuthPolicy started.")

	ap.Status.ObservedGeneration = ap.GetGeneration()
	authPolicyCondition := metav1.Condition{
		Type:               state.GetID(strings.TrimPrefix(ap.Kind, "*"), ap.Name),
		LastTransitionTime: metav1.Now(),
	}

	switch {
	case scope.InvalidConfig:
		ap.Status.Phase = ztoperatorv1alpha1.PhaseInvalid
		ap.Status.Ready = false
		ap.Status.Message = *scope.ValidationErrorMessage
		authPolicyCondition.Status = metav1.ConditionFalse
		authPolicyCondition.Reason = "InvalidConfiguration"
		authPolicyCondition.Message = *scope.ValidationErrorMessage

	case len(scope.Descendants) != reconciliation.CountReconciledResources(reconcileFuncs):
		ap.Status.Phase = ztoperatorv1alpha1.PhasePending
		ap.Status.Ready = false
		ap.Status.Message = "AuthPolicy pending due to missing Descendants."
		authPolicyCondition.Status = metav1.ConditionUnknown
		authPolicyCondition.Reason = "ReconciliationPending"
		authPolicyCondition.Message = "Descendants of AuthPolicy are not yet reconciled."

	case len(scope.GetErrors()) > 0:
		ap.Status.Phase = ztoperatorv1alpha1.PhaseFailed
		ap.Status.Ready = false
		ap.Status.Message = "AuthPolicy failed."
		authPolicyCondition.Status = metav1.ConditionFalse
		authPolicyCondition.Reason = "ReconciliationFailed"
		authPolicyCondition.Message = "Descendants of AuthPolicy failed during reconciliation."

	default:
		ap.Status.Phase = ztoperatorv1alpha1.PhaseReady
		ap.Status.Ready = true
		ap.Status.Message = "AuthPolicy ready."
		authPolicyCondition.Status = metav1.ConditionTrue
		authPolicyCondition.Reason = "ReconciliationSuccess"
		authPolicyCondition.Message = "Descendants of AuthPolicy reconciled successfully."
	}

	var conditions []metav1.Condition
	descendantIDs := map[string]bool{}

	for _, d := range scope.Descendants {
		descendantIDs[d.ID] = true
		cond := metav1.Condition{
			Type:               d.ID,
			LastTransitionTime: metav1.Now(),
		}
		if d.ErrorMessage != nil {
			cond.Status = metav1.ConditionFalse
			cond.Reason = "Error"
			cond.Message = *d.ErrorMessage
		} else if d.SuccessMessage != nil {
			cond.Status = metav1.ConditionTrue
			cond.Reason = "Success"
			cond.Message = *d.SuccessMessage
		} else {
			cond.Status = metav1.ConditionUnknown
			cond.Reason = "Unknown"
			cond.Message = "No status message set"
		}
		conditions = append(conditions, cond)
	}
	for _, rf := range reconcileFuncs {
		if !rf.IsResourceNil() {
			expectedID := state.GetID(rf.GetResourceKind(), rf.GetResourceName())
			if !descendantIDs[expectedID] {
				conditions = append(conditions, metav1.Condition{
					Type:               expectedID,
					Status:             metav1.ConditionFalse,
					Reason:             "NotFound",
					Message:            fmt.Sprintf("Expected resource %s of kind %s was not created", rf.GetResourceName(), rf.GetResourceKind()),
					LastTransitionTime: metav1.Now(),
				})
			}
		}
	}

	ap.Status.Conditions = append([]metav1.Condition{authPolicyCondition}, conditions...)

	if !equality.Semantic.DeepEqual(original.Status, ap.Status) {
		rLog.Debug(fmt.Sprintf("Updating AuthPolicy status with name %s/%s", ap.Namespace, ap.Name))
		if err := r.updateStatusWithRetriesOnConflict(ctx, ap); err != nil {
			rLog.Error(err, fmt.Sprintf("Failed to update AuthPolicy status with name %s/%s", ap.Namespace, ap.Name))
			r.Recorder.Eventf(&ap, "Warning", "StatusUpdateFailed", "Status update of AuthPolicy failed.")
		} else {
			r.Recorder.Eventf(&ap, "Normal", "StatusUpdateSuccess", "Status update of AuthPolicy updated successfully.")
		}
	}
}

func (r *AuthPolicyReconciler) updateStatusWithRetriesOnConflict(ctx context.Context, authPolicy ztoperatorv1alpha1.AuthPolicy) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &ztoperatorv1alpha1.AuthPolicy{}
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(&authPolicy), latest); err != nil {
			return err
		}
		latest.Status = authPolicy.Status
		return r.Status().Update(ctx, latest)
	})
}

func reconcileAuthPolicy[T client.Object](
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
		// Resource is not desired. Try deleting the existing one if it exists.
		resourceType := reflect.TypeOf((*T)(nil)).Elem()
		current := reflect.New(resourceType.Elem()).Interface().(T)

		accessor := current
		accessor.SetNamespace(scope.AuthPolicy.Namespace)
		accessor.SetName(resourceName)

		rLog.Info(fmt.Sprintf("Desired %s %s/%s is nil. Will try to delete it if it exist", resourceKind, accessor.GetNamespace(), accessor.GetName()))
		rLog.Debug(fmt.Sprintf("Checking if %s %s/%s exists", resourceKind, accessor.GetNamespace(), accessor.GetName()))

		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(accessor), current)
		if err != nil {
			if apierrors.IsNotFound(err) {
				rLog.Debug(fmt.Sprintf("%s %s/%s already deleted", resourceKind, accessor.GetNamespace(), accessor.GetName()))
				return ctrl.Result{}, nil
			}
			getErrorMessage := fmt.Sprintf("Failed to get %s %s/%s when trying to delete it.", resourceKind, accessor.GetNamespace(), accessor.GetName())
			rLog.Error(err, getErrorMessage)
			scope.ReplaceDescendant(accessor, &getErrorMessage, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}

		rLog.Info(fmt.Sprintf("Deleting %s %s/%s as it's no longer desired", resourceKind, accessor.GetNamespace(), accessor.GetName()))
		if err := k8sClient.Delete(ctx, current); err != nil {
			deleteErrorMessage := fmt.Sprintf("Failed to delete %s %s/%s", resourceKind, accessor.GetNamespace(), accessor.GetName())
			rLog.Error(err, deleteErrorMessage)
			scope.ReplaceDescendant(accessor, &deleteErrorMessage, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}

		rLog.Debug(fmt.Sprintf("Successfully deleted %s %s/%s", resourceKind, accessor.GetNamespace(), accessor.GetName()))
		successMsg := fmt.Sprintf("Deleted %s %s/%s as it is no longer desired.", resourceKind, accessor.GetNamespace(), accessor.GetName())
		scope.ReplaceDescendant(accessor, nil, &successMsg, resourceKind, resourceName)
		return ctrl.Result{}, nil
	}

	deReferencedDesired := *desired

	kind := reflect.TypeOf(deReferencedDesired).Elem().Name()
	current := reflect.New(reflect.TypeOf(deReferencedDesired).Elem()).Interface().(T)

	rLog.Info(fmt.Sprintf("Trying to generate %s %s/%s", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))

	rLog.Debug(fmt.Sprintf("Checking if %s %s/%s exists", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deReferencedDesired), current)
	if apierrors.IsNotFound(err) {
		rLog.Debug(fmt.Sprintf("%s %s/%s does not exist", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))
		if err := ctrl.SetControllerReference(&scope.AuthPolicy, deReferencedDesired, scheme); err != nil {
			errorReason := fmt.Sprintf("Unable to set AuthPolicy ownerReference on %s %s/%s.", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName())
			scope.ReplaceDescendant(deReferencedDesired, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}

		rLog.Info(fmt.Sprintf("Creating %s %s/%s", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))
		if err := k8sClient.Create(ctx, deReferencedDesired); err != nil {
			errorReason := fmt.Sprintf("Unable to create %s %s/%s", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName())
			scope.ReplaceDescendant(deReferencedDesired, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}
		successMessage := fmt.Sprintf("Successfully created %s %s/%s.", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName())
		scope.ReplaceDescendant(deReferencedDesired, nil, &successMessage, resourceKind, resourceName)

		return ctrl.Result{}, nil
	}

	if err != nil {
		errorReason := fmt.Sprintf("Unable to get %s %s/%s.", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName())
		scope.ReplaceDescendant(deReferencedDesired, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, err
	}

	rLog.Debug(fmt.Sprintf("%s %s/%s exists", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))
	rLog.Debug(fmt.Sprintf("Determing if %s %s/%s should be updated", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))
	if shouldUpdate(current, deReferencedDesired) {
		rLog.Debug(fmt.Sprintf("Current %s %s/%s != desired", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))
		rLog.Debug(fmt.Sprintf("Updating current %s %s/%s with desired", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))
		updateFields(current, deReferencedDesired)

		if err := k8sClient.Update(ctx, current); err != nil {
			errorReason := fmt.Sprintf("Unable to update %s %s/%s.", kind, current.GetNamespace(), current.GetName())
			scope.ReplaceDescendant(current, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}

	} else {
		rLog.Debug(fmt.Sprintf("Current %s %s/%s == desired. No update needed.", kind, deReferencedDesired.GetNamespace(), deReferencedDesired.GetName()))
	}

	successMessage := fmt.Sprintf("Successfully generated %s %s/%s", kind, current.GetNamespace(), current.GetName())
	rLog.Info(successMessage)
	scope.ReplaceDescendant(current, nil, &successMessage, resourceKind, resourceName)

	return ctrl.Result{}, nil
}

func resolveAuthPolicy(ctx context.Context, k8sClient client.Client, authPolicy *ztoperatorv1alpha1.AuthPolicy) (*state.Scope, error) {
	rLog := log.GetLogger(ctx)
	if authPolicy == nil {
		return nil, fmt.Errorf("encountered AuthPolicy as null when resolving")
	}
	rLog.Info(fmt.Sprintf("Trying to resolve auth policy %s/%s", authPolicy.Namespace, authPolicy.Name))
	if !authPolicy.Spec.Enabled {
		rLog.Debug(fmt.Sprintf("AuthPolicy %s/%s is disabled", authPolicy.Namespace, authPolicy.Name))
		return &state.Scope{
			AuthPolicy: *authPolicy,
		}, nil
	}

	var oAuthCredentials state.OAuthCredentials

	if authPolicy.Spec.OAuthCredentials != nil {
		oAuthSecret, err := utils.GetSecret(k8sClient, ctx, types.NamespacedName{
			Namespace: authPolicy.Namespace,
			Name:      authPolicy.Spec.OAuthCredentials.SecretRef,
		})
		if err != nil {
			return nil, err
		}

		if authPolicy.Spec.AutoLogin != nil && authPolicy.Spec.AutoLogin.Enabled {
			clientId := string(oAuthSecret.Data[authPolicy.Spec.OAuthCredentials.ClientIdKey])
			oAuthCredentials.ClientId = &clientId

			clientSecret := string(oAuthSecret.Data[authPolicy.Spec.OAuthCredentials.ClientSecretKey])
			oAuthCredentials.ClientSecret = &clientSecret

			if oAuthCredentials.ClientId == nil {
				return nil, fmt.Errorf("client id with key: %s was nil when retrieving it from Secret with name %s/%s", authPolicy.Spec.OAuthCredentials.ClientIdKey, authPolicy.Namespace, authPolicy.Spec.OAuthCredentials.SecretRef)
			} else {
				if *oAuthCredentials.ClientId == "" {
					return nil, fmt.Errorf("client id with key: %s was empty string when retrieving it from Secret with name %s/%s", authPolicy.Spec.OAuthCredentials.ClientIdKey, authPolicy.Namespace, authPolicy.Spec.OAuthCredentials.SecretRef)
				}
			}

			if oAuthCredentials.ClientSecret == nil {
				return nil, fmt.Errorf("client secret with key: %s was nil when retrieving it from Secret with name %s/%s", authPolicy.Spec.OAuthCredentials.ClientSecretKey, authPolicy.Namespace, authPolicy.Spec.OAuthCredentials.SecretRef)
			} else {
				if *oAuthCredentials.ClientSecret == "" {
					return nil, fmt.Errorf("client secret with key: %s was empty string when retrieving it from Secret with name %s/%s", authPolicy.Spec.OAuthCredentials.ClientSecretKey, authPolicy.Namespace, authPolicy.Spec.OAuthCredentials.SecretRef)
				}
			}
		}
	}

	var identityProviderUris state.IdentityProviderUris
	rLog.Info(fmt.Sprintf("Trying to resolve discovery document from well-known uri: %s for AuthPolicy with name %s/%s", authPolicy.Spec.WellKnownUri, authPolicy.Namespace, authPolicy.Name))
	discoveryDocument, err := rest.GetOAuthDiscoveryDocument(authPolicy.Spec.WellKnownUri, rLog)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve discovery document from well-known uri: %s for AuthPolicy with name %s/%s: %w", authPolicy.Spec.WellKnownUri, authPolicy.Namespace, authPolicy.Name, err)
	}

	if discoveryDocument.Issuer == nil || discoveryDocument.JwksUri == nil || discoveryDocument.TokenEndpoint == nil || discoveryDocument.AuthorizationEndpoint == nil {
		return nil, fmt.Errorf("failed to parse discovery document from well-known uri: %s for AuthPolicy with name %s/%s", authPolicy.Spec.WellKnownUri, authPolicy.Namespace, authPolicy.Name)
	}
	identityProviderUris.IssuerUri = *discoveryDocument.Issuer
	identityProviderUris.JwksUri = *discoveryDocument.JwksUri
	identityProviderUris.TokenUri = *discoveryDocument.TokenEndpoint
	identityProviderUris.AuthorizationUri = *discoveryDocument.AuthorizationEndpoint

	autoLoginConfig := state.AutoLoginConfig{
		Enabled: false,
	}

	if authPolicy.Spec.AutoLogin != nil && authPolicy.Spec.AutoLogin.Enabled {
		autoLoginConfig.Enabled = authPolicy.Spec.AutoLogin.Enabled
		autoLoginConfig.LoginPath = authPolicy.Spec.AutoLogin.LoginPath
		autoLoginConfig.RedirectPath = authPolicy.Spec.AutoLogin.RedirectPath
		autoLoginConfig.LogoutPath = authPolicy.Spec.AutoLogin.LogoutPath
		autoLoginConfig.Scopes = authPolicy.Spec.AutoLogin.Scopes
	}

	rLog.Info(fmt.Sprintf("Successfully resolved AuthPolicy with name %s/%s", authPolicy.Namespace, authPolicy.Name))

	return &state.Scope{
		AuthPolicy:           *authPolicy,
		AutoLoginConfig:      autoLoginConfig,
		OAuthCredentials:     oAuthCredentials,
		IdentityProviderUris: identityProviderUris,
	}, nil
}
