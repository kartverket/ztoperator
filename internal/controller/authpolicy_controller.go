package controller

import (
	"context"
	"errors"
	"fmt"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/eventhandler/pod"
	"github.com/kartverket/ztoperator/internal/reconciler"
	"github.com/kartverket/ztoperator/internal/resolver"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/internal/statusmanager"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/metrics"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	"github.com/kartverket/ztoperator/pkg/rest"
	"github.com/kartverket/ztoperator/pkg/validation"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8sErrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AuthPolicyReconciler reconciles a AuthPolicy object.
type AuthPolicyReconciler struct {
	client.Client
	Scheme                    *runtime.Scheme
	Recorder                  events.EventRecorder
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

	rLog.Info(fmt.Sprintf("Received reconcile request for AuthPolicy with name %s", req.String()))

	if err := r.Get(ctx, req.NamespacedName, authPolicy); err != nil {
		if apierrors.IsNotFound(err) {
			rLog.Debug(
				fmt.Sprintf("AuthPolicy with name %s not found. Probably a delete.", req.String()),
			)
			metrics.DeleteAuthPolicyInfo(req.NamespacedName)
			return reconcile.Result{}, nil
		}
		rLog.Error(err, fmt.Sprintf("Failed to get AuthPolicy with name %s", req.String()))
		return reconcile.Result{}, err
	}

	r.Recorder.Eventf(
		authPolicy,
		nil,
		"Normal",
		"ReconcileStarted",
		"Reconcile",
		"AuthPolicy with name %s started.", req.String(),
	)
	rLog.Debug(fmt.Sprintf("AuthPolicy with name %s found", req.String()))

	authPolicy.InitializeStatus()
	originalAuthPolicy := authPolicy.DeepCopy()

	if !authPolicy.DeletionTimestamp.IsZero() {
		rLog.Info(fmt.Sprintf("Deleting AuthPolicy with name %s", req.String()))
		metrics.DeleteAuthPolicyInfo(req.NamespacedName)
		return ctrl.Result{}, nil
	}

	scope, err := resolveAuthPolicy(ctx, r.Client, authPolicy, r.DiscoveryDocumentResolver)
	if err != nil {
		rLog.Error(err, fmt.Sprintf("Failed to resolve AuthPolicy with name %s", req.String()))
		authPolicy.Status.Phase = ztoperatorv1alpha1.PhaseFailed
		authPolicy.Status.Message = err.Error()
		updateStatusOnResolveFailedErr := statusmanager.UpdateStatus(ctx, r.Client, *authPolicy)
		if updateStatusOnResolveFailedErr != nil {
			return ctrl.Result{}, updateStatusOnResolveFailedErr
		}
		return reconcile.Result{}, err
	}

	scope = validateAuthPolicy(ctx, r.Client, scope)

	reconcileActions := reconciler.ReconcileActions(scope)

	defer func() {
		statusmanager.UpdateAuthPolicyStatus(ctx, r.Client, r.Recorder, scope, originalAuthPolicy, reconcileActions)
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
				nil,
				"Warning",
				fmt.Sprintf("%sReconcileFailed", rf.GetResourceKind()),
				"Reconcile",
				"%s with name %s failed during reconciliation.", rf.GetResourceKind(), rf.GetResourceName(),
			)
			errs = append(errs, err)
		} else {
			r.Recorder.Eventf(
				&scope.AuthPolicy,
				nil,
				"Normal",
				fmt.Sprintf("%sReconciledSuccessfully", rf.GetResourceKind()),
				"Reconcile",
				"%s with name %s reconciled successfully.", rf.GetResourceKind(), rf.GetResourceName(),
			)
		}
		if len(errs) > 0 {
			continue
		}
		result = helperfunctions.LowestNonZeroResult(result, reconcileResult)
	}

	if len(errs) > 0 {
		r.Recorder.Eventf(
			&scope.AuthPolicy,
			nil,
			"Warning",
			"ReconcileFailed",
			"Reconcile",
			"AuthPolicy failed during reconciliation",
		)
		return ctrl.Result{}, k8sErrors.NewAggregate(errs)
	}
	r.Recorder.Eventf(
		&scope.AuthPolicy,
		nil,
		"Normal",
		"ReconcileSuccess",
		"Reconcile",
		"AuthPolicy reconciled successfully",
	)
	return result, nil
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

	autoLoginConfig := resolver.ResolveAutoLoginConfig(authPolicy, *identityProviderUris)

	resolvedAudiences, errAudiences := resolver.ResolveAudiences(
		ctx,
		k8sClient,
		authPolicy.Namespace,
		authPolicy.Spec.AllowedAudiences,
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
