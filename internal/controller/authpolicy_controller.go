package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/eventhandler/pod"
	"github.com/kartverket/ztoperator/internal/reconciler"
	"github.com/kartverket/ztoperator/internal/resolver"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/kartverket/ztoperator/pkg/metrics"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	"github.com/kartverket/ztoperator/pkg/rest"
	"github.com/kartverket/ztoperator/pkg/validation"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sErrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AuthPolicyReconciler reconciles a AuthPolicy object.
type AuthPolicyReconciler struct {
	client.Client
	Scheme                    *runtime.Scheme
	Recorder                  record.EventRecorder
	DiscoveryDocumentResolver rest.DiscoveryDocumentResolver
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ztoperatorv1alpha1.AuthPolicy{}).
		Owns(&istioclientsecurityv1.RequestAuthentication{}).
		Owns(&istioclientsecurityv1.AuthorizationPolicy{}).
		Owns(&v1alpha4.EnvoyFilter{}).
		Owns(&v1.Secret{}).
		Watches(&v1.Pod{}, pod.EventHandler(r.Client)).
		Complete(r)
}

// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=namespaces;pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies;requestauthentications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=envoyfilters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

func (r *AuthPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)

	authPolicy := new(ztoperatorv1alpha1.AuthPolicy)

	rLog.Info(fmt.Sprintf("Received reconcile request for AuthPolicy with name %s", req.NamespacedName.String()))

	if err := r.Client.Get(ctx, req.NamespacedName, authPolicy); err != nil {
		if apierrors.IsNotFound(err) {
			rLog.Debug(
				fmt.Sprintf("AuthPolicy with name %s not found. Probably a delete.", req.NamespacedName.String()),
			)
			metrics.DeleteAuthPolicyInfo(req.NamespacedName)
			return reconcile.Result{}, nil
		}
		rLog.Error(err, fmt.Sprintf("Failed to get AuthPolicy with name %s", req.NamespacedName.String()))
		return reconcile.Result{}, err
	}

	r.Recorder.Eventf(
		authPolicy,
		"Normal",
		"ReconcileStarted",
		fmt.Sprintf("AuthPolicy with name %s started.", req.NamespacedName.String()),
	)
	rLog.Debug(fmt.Sprintf("AuthPolicy with name %s found", req.NamespacedName.String()))

	authPolicy.InitializeStatus()
	originalAuthPolicy := authPolicy.DeepCopy()

	if !authPolicy.DeletionTimestamp.IsZero() {
		rLog.Info(fmt.Sprintf("Deleting AuthPolicy with name %s", req.NamespacedName.String()))
		metrics.DeleteAuthPolicyInfo(req.NamespacedName)
		return ctrl.Result{}, nil
	}

	scope, err := resolveAuthPolicy(ctx, r.Client, authPolicy, r.DiscoveryDocumentResolver)
	if err != nil {
		rLog.Error(err, fmt.Sprintf("Failed to resolve AuthPolicy with name %s", req.NamespacedName.String()))
		authPolicy.Status.Phase = ztoperatorv1alpha1.PhaseFailed
		authPolicy.Status.Message = err.Error()
		updateStatusOnResolveFailedErr := r.updateStatusWithRetriesOnConflict(ctx, *authPolicy)
		if updateStatusOnResolveFailedErr != nil {
			return ctrl.Result{}, updateStatusOnResolveFailedErr
		}
		return reconcile.Result{}, err
	}

	scope.AutoLoginConfig.EnvoySecretName = authPolicy.Name + "-envoy-secret"

	scope = validateAuthPolicy(ctx, r.Client, scope)

	reconcileActions := reconciler.ReconcileActions(scope)

	defer func() {
		r.updateStatus(ctx, scope, originalAuthPolicy, reconcileActions)
	}()

	return r.doReconcile(ctx, reconcileActions, scope)
}

func (r *AuthPolicyReconciler) doReconcile(
	ctx context.Context,
	reconcileFuncs []reconciliation.ReconcileAction,
	scope *state.Scope,
) (ctrl.Result, error) {
	result := ctrl.Result{}
	var errs []error
	for _, rf := range reconcileFuncs {
		reconcileResult, err := rf.Reconcile(ctx, r.Client, r.Scheme)
		if err != nil {
			r.Recorder.Eventf(
				&scope.AuthPolicy,
				"Warning",
				fmt.Sprintf("%sReconcileFailed", rf.GetResourceKind()),
				fmt.Sprintf(
					"%s with name %s failed during reconciliation.",
					rf.GetResourceKind(),
					rf.GetResourceName(),
				),
			)
			errs = append(errs, err)
		} else {
			r.Recorder.Eventf(&scope.AuthPolicy, "Normal", fmt.Sprintf("%sReconciledSuccessfully", rf.GetResourceKind()), fmt.Sprintf("%s with name %s reconciled successfully.", rf.GetResourceKind(), rf.GetResourceName()))
		}
		if len(errs) > 0 {
			continue
		}
		result = helperfunctions.LowestNonZeroResult(result, reconcileResult)
	}

	if len(errs) > 0 {
		r.Recorder.Eventf(&scope.AuthPolicy, "Warning", "ReconcileFailed", "AuthPolicy failed during reconciliation")
		return ctrl.Result{}, k8sErrors.NewAggregate(errs)
	}
	r.Recorder.Eventf(&scope.AuthPolicy, "Normal", "ReconcileSuccess", "AuthPolicy reconciled successfully")
	return result, nil
}

func (r *AuthPolicyReconciler) updateStatus(
	ctx context.Context,
	scope *state.Scope,
	original *ztoperatorv1alpha1.AuthPolicy,
	reconcileFuncs []reconciliation.ReconcileAction,
) {
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
		switch {
		case d.ErrorMessage != nil:
			cond.Status = metav1.ConditionFalse
			cond.Reason = "Error"
			cond.Message = *d.ErrorMessage
		case d.SuccessMessage != nil:
			cond.Status = metav1.ConditionTrue
			cond.Reason = "Success"
			cond.Message = *d.SuccessMessage
		default:
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
					Type:   expectedID,
					Status: metav1.ConditionFalse,
					Reason: "NotFound",
					Message: fmt.Sprintf(
						"Expected resource %s of kind %s was not created",
						rf.GetResourceName(),
						rf.GetResourceKind(),
					),
					LastTransitionTime: metav1.Now(),
				})
			}
		}
	}

	ap.Status.Conditions = append([]metav1.Condition{authPolicyCondition}, conditions...)

	if !equality.Semantic.DeepEqual(original.Status, ap.Status) {
		rLog.Debug(fmt.Sprintf("Updating AuthPolicy status with name %s/%s", ap.Namespace, ap.Name))
		if updateStatusWithRetriesErr := r.updateStatusWithRetriesOnConflict(ctx, ap); updateStatusWithRetriesErr != nil {
			rLog.Error(
				updateStatusWithRetriesErr,
				fmt.Sprintf(
					"Failed to update AuthPolicy status with name %s/%s",
					ap.Namespace,
					ap.Name,
				),
			)
			r.Recorder.Eventf(&ap, "Warning", "StatusUpdateFailed", "Status update of AuthPolicy failed.")
		} else {
			r.Recorder.Eventf(&ap, "Normal", "StatusUpdateSuccess", "Status update of AuthPolicy updated successfully.")
		}
	}
}

func (r *AuthPolicyReconciler) updateStatusWithRetriesOnConflict(
	ctx context.Context,
	authPolicy ztoperatorv1alpha1.AuthPolicy,
) error {
	metrics.DeleteAuthPolicyInfo(
		types.NamespacedName{
			Name:      authPolicy.Name,
			Namespace: authPolicy.Namespace,
		},
	)
	refreshAuthPolicyCustomMetricsErr := metrics.RefreshAuthPolicyInfo(ctx, r.Client, authPolicy)
	if refreshAuthPolicyCustomMetricsErr != nil {
		return refreshAuthPolicyCustomMetricsErr
	}
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &ztoperatorv1alpha1.AuthPolicy{}
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(&authPolicy), latest); err != nil {
			return err
		}
		latest.Status = authPolicy.Status
		return r.Status().Update(ctx, latest)
	})
}

func resolveAuthPolicy(
	ctx context.Context,
	k8sClient client.Client,
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
	discoveryDocumentResolver rest.DiscoveryDocumentResolver,
) (*state.Scope, error) {
	rLog := log.GetLogger(ctx)
	if authPolicy == nil {
		return nil, errors.New("encountered AuthPolicy as null when resolving")
	}
	rLog.Info(fmt.Sprintf("Trying to resolve auth policy %s/%s", authPolicy.Namespace, authPolicy.Name))

	oAuthCredentials, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, authPolicy)
	if err != nil {
		return nil, err
	}

	rLog.Info(
		fmt.Sprintf(
			"Trying to resolve discovery document from well-known uri: %s for AuthPolicy with name %s/%s",
			authPolicy.Spec.WellKnownURI,
			authPolicy.Namespace,
			authPolicy.Name,
		),
	)
	var identityProviderUris, errIdentityProviderUris = resolver.ResolveDiscoveryDocument(
		ctx,
		authPolicy,
		discoveryDocumentResolver,
	)
	if errIdentityProviderUris != nil {
		return nil, errIdentityProviderUris
	}

	autoLoginConfig := state.AutoLoginConfig{
		Enabled: false,
	}

	if authPolicy.Spec.AutoLogin != nil && authPolicy.Spec.AutoLogin.Enabled {
		autoLoginConfig.Enabled = authPolicy.Spec.AutoLogin.Enabled
		autoLoginConfig.LoginPath = authPolicy.Spec.AutoLogin.LoginPath
		autoLoginConfig.PostLogoutRedirectURI = authPolicy.Spec.AutoLogin.PostLogoutRedirectURI
		autoLoginConfig.Scopes = authPolicy.Spec.AutoLogin.Scopes
		autoLoginConfig.LoginParams = authPolicy.Spec.AutoLogin.LoginParams

		autoLoginConfig.SetSaneDefaults(*authPolicy.Spec.AutoLogin)

		autoLoginConfig.LuaScriptConfig = state.LuaScriptConfig{
			LuaScript: luascript.GetLuaScript(
				authPolicy,
				autoLoginConfig,
				*identityProviderUris,
			),
		}
	}

	resolvedAudiences, errAudiences := resolver.ResolveAudiences(
		ctx,
		k8sClient,
		authPolicy.Namespace,
		authPolicy.Spec.AllowedAudiences,
		//nolint:staticcheck // we have to use this field for backward compatibility
		authPolicy.Spec.Audience,
	)
	if errAudiences != nil {
		return nil, fmt.Errorf("failed to resolve audiences: %w", errAudiences)
	}

	rLog.Info(fmt.Sprintf("Successfully resolved AuthPolicy with name %s/%s", authPolicy.Namespace, authPolicy.Name))

	return &state.Scope{
		Audiences:            *resolvedAudiences,
		AuthPolicy:           *authPolicy,
		AutoLoginConfig:      autoLoginConfig,
		OAuthCredentials:     *oAuthCredentials,
		IdentityProviderUris: *identityProviderUris,
	}, nil
}

func validateAuthPolicy(ctx context.Context, k8sClient client.Client, scope *state.Scope) *state.Scope {
	rLog := log.GetLogger(ctx)
	for _, validator := range validation.GetValidators() {
		if validationErr := validator.Validate(ctx, k8sClient, scope); validationErr != nil {
			rLog.Error(
				validationErr,
				fmt.Sprintf(
					"%s failed for AuthPolicy with name %s/%s",
					validator.Type.String(),
					scope.AuthPolicy.Namespace,
					scope.AuthPolicy.Name,
				),
			)
			rLog.Debug(
				fmt.Sprintf(
					"%s failed for AuthPolicy with name %s/%s. Defaulting to default deny on all paths.",
					validator.Type.String(),
					scope.AuthPolicy.Namespace,
					scope.AuthPolicy.Name,
				),
			)
			scope.InvalidConfig = true
			validationErrorMessage := validationErr.Error()
			scope.ValidationErrorMessage = &validationErrorMessage
			return scope
		}
	}

	scope.InvalidConfig = false
	return scope
}
