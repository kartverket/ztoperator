package controller

import (
	"context"
	"fmt"
	"maps"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/eventhandler/configmap"
	"github.com/kartverket/ztoperator/internal/eventhandler/deployments"
	"github.com/kartverket/ztoperator/internal/eventhandler/secret"
	"github.com/kartverket/ztoperator/internal/predicates"
	"github.com/kartverket/ztoperator/internal/reconciler"
	"github.com/kartverket/ztoperator/internal/resolver"
	"github.com/kartverket/ztoperator/internal/statusmanager"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/labels"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/metrics"
	"github.com/kartverket/ztoperator/pkg/model"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8sErrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AuthPolicyReconciler reconciles a AuthPolicy object.
type AuthPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(
			&ztoperatorv1alpha1.AuthPolicy{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&istioclientsecurityv1.RequestAuthentication{}, builder.WithPredicates(predicates.SpecOrLabelsChanged())).
		Owns(&istioclientsecurityv1.AuthorizationPolicy{}, builder.WithPredicates(predicates.SpecOrLabelsChanged())).
		Owns(&v1alpha4.EnvoyFilter{}, builder.WithPredicates(predicates.SpecOrLabelsChanged())).
		Owns(&corev1.Secret{}, builder.WithPredicates(predicates.SecretContentOrLabelsChanged())).
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(predicates.ConfigMapContentOrLabelsChanged())).
		Watches(&corev1.Secret{}, secret.EventHandler(r.Client), builder.WithPredicates(predicates.SecretContentOrLabelsChanged())).
		Watches(&corev1.ConfigMap{}, configmap.EventHandler(r.Client), builder.WithPredicates(predicates.ConfigMapContentOrLabelsChanged())).
		Watches(&appsv1.Deployment{}, deployments.EventHandler(r.Client), builder.WithPredicates(predicates.DeploymentPodTemplateMetadataOrLabelsChanged())).
		Complete(r)
}

// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ztoperator.kartverket.no,resources=authpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=namespaces;pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies;requestauthentications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=envoyfilters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets;configmaps,verbs=get;list;watch;create;update;patch;delete

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

	if originalAuthPolicy.Labels == nil {
		originalAuthPolicy.Labels = make(map[string]string)
	}
	maps.Copy(originalAuthPolicy.Labels, labels.AuthPolicyStandardLabels())
	if !maps.Equal(originalAuthPolicy.Labels, authPolicy.Labels) {
		err := r.Patch(ctx, originalAuthPolicy, client.MergeFrom(authPolicy))
		if err != nil {
			return r.failReconcile(ctx, authPolicy, err, "failed to patch AuthPolicy with AuthPolicy standard labels")
		}
	}

	scope, err := resolveAuthPolicy(ctx, r.Client, *authPolicy)
	if err != nil {
		return r.failReconcile(ctx, authPolicy, err, err.Error())
	}

	controllerResources := reconciler.ControllerResources(scope)

	defer func() {
		statusmanager.UpdateAuthPolicyStatus(ctx, r.Client, r.Recorder, scope, originalAuthPolicy, controllerResources)
	}()

	return r.doReconcile(ctx, controllerResources, scope)
}

func (r *AuthPolicyReconciler) doReconcile(
	ctx context.Context,
	reconcileFuncs []reconciliation.ControllerResource,
	scope *model.Scope,
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
	authPolicy ztoperatorv1alpha1.AuthPolicy,
) (*model.Scope, error) {
	rLog := log.GetLogger(ctx)
	rLog.Info(fmt.Sprintf("Trying to resolve AuthPolicy %s/%s", authPolicy.Namespace, authPolicy.Name))

	scope := model.Scope{AuthPolicy: authPolicy}
	if err := resolveCommon(ctx, k8sClient, &scope); err != nil {
		return nil, fmt.Errorf("failed to resolve common AuthPolicy fields: %w", err)
	}
	if config.Get().FeatureIstioAmbient {
		istioAmbientConfig, err := resolveIstioAmbientConfig(ctx, k8sClient, &scope)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to resolve Istio Ambient config for AuthPolicy %s/%s: %w",
				authPolicy.Namespace,
				authPolicy.Name,
				err,
			)
		}
		scope.IstioAmbientConfig = *istioAmbientConfig
	}

	rLog.Info(fmt.Sprintf("Successfully resolved AuthPolicy with name %s/%s", authPolicy.Namespace, authPolicy.Name))

	return &scope, nil
}

func resolveCommon(
	ctx context.Context,
	k8sClient client.Client,
	scope *model.Scope,
) error {
	identityProviderUris, errIdentityProviderUris := resolver.ResolveDiscoveryDocument(scope.AuthPolicy)
	if errIdentityProviderUris != nil {
		return errIdentityProviderUris
	}
	scope.IdentityProviderUris = *identityProviderUris

	autoLoginConfig := resolver.ResolveAutoLoginConfig(scope.AuthPolicy, *identityProviderUris)
	scope.AutoLoginConfig = autoLoginConfig

	oAuthCredentials, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, scope.AuthPolicy)
	if err != nil {
		return err
	}
	scope.OAuthCredentials = *oAuthCredentials

	resolvedAudiences, errAudiences := resolver.ResolveAudiences(
		ctx,
		k8sClient,
		scope.AuthPolicy.Namespace,
		scope.AuthPolicy.Spec.AllowedAudiences,
	)
	if errAudiences != nil {
		return fmt.Errorf("failed to resolve audiences: %w", errAudiences)
	}
	scope.Audiences = *resolvedAudiences
	return nil
}

func resolveIstioAmbientConfig(
	ctx context.Context,
	k8sClient client.Client,
	scope *model.Scope,
) (*model.IstioAmbientConfig, error) {
	istioAmbientConfig := model.IstioAmbientConfig{Enabled: false}
	namespace, err := helperfunctions.GetNamespace(ctx, k8sClient, scope.AuthPolicy.Namespace)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get namespace %s: %w",
			scope.AuthPolicy.Namespace,
			err,
		)
	}

	deployment, err := helperfunctions.GetProtectedDeployment(ctx, k8sClient, scope.AuthPolicy)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get deployment selected by AuthPolicy selector %v: %w",
			scope.AuthPolicy.Spec.Selector,
			err,
		)
	}

	isIstioAmbientEnabled := helperfunctions.NamespaceIsIstioAmbientEligible(namespace) &&
		helperfunctions.DeploymentIsIstioAmbientEligible(*deployment)

	if isIstioAmbientEnabled {
		envoyBootstrap, resolveEnvoyBootstrapErr := resolver.ResolveEnvoyBootstrap(
			scope.AuthPolicy,
			scope.AutoLoginConfig,
			scope.OAuthCredentials,
			scope.IdentityProviderUris,
			scope.Audiences,
			*deployment,
		)
		if resolveEnvoyBootstrapErr != nil {
			return nil, resolveEnvoyBootstrapErr
		}
		istioAmbientConfig.Enabled = true
		istioAmbientConfig.EnvoyBootstrap = envoyBootstrap
	}
	return &istioAmbientConfig, nil
}

func (r *AuthPolicyReconciler) failReconcile(
	ctx context.Context,
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
	err error,
	errMsg string,
) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)
	rLog.Error(err, errMsg)
	authPolicy.Status.Phase = ztoperatorv1alpha1.PhaseFailed
	authPolicy.Status.Message = errMsg
	if updateErr := statusmanager.UpdateStatus(ctx, r.Client, *authPolicy); updateErr != nil {
		return ctrl.Result{}, updateErr
	}
	return ctrl.Result{}, err
}
