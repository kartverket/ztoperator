package reconciler

import (
	"bytes"
	"reflect"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy/deny"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy/ignore"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy/require"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/requestauthentication"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/secret"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/api/core/v1"
)

// ReconcileActions creates all reconcile actions for the given AuthPolicy scope.
func ReconcileActions(scope *state.Scope) []reconciliation.ReconcileAction {
	return []reconciliation.ReconcileAction{
		secretReconcileAction(scope),
		envoyFilterReconcileAction(scope),
		requestAuthenticationReconcileAction(scope),
		denyAuthorizationPolicyReconcileAction(scope),
		ignoreAuthorizationPolicyReconcileAction(scope),
		requireAuthorizationPolicyReconcileAction(scope),
	}
}

/*
secretReconcileAction reconciles a Secret resource containing a HMAC secret (cookie signing key) and token secret
(OAuth client secret), if auto-login is enabled. The secrets are used by Envoy during Authorization Code Flow.
*/
func secretReconcileAction(scope *state.Scope) AuthPolicyAdapter[*v1.Secret] {
	desiredResource := secret.GetDesired(
		scope,
		helperfunctions.BuildObjectMeta(
			scope.AutoLoginConfig.EnvoySecretName,
			scope.AuthPolicy.Namespace,
		),
	)

	return AuthPolicyAdapter[*v1.Secret]{
		reconciliation.ReconcileFuncAdapter[*v1.Secret]{
			Func: reconciliation.ReconcileFunc[*v1.Secret]{
				ResourceKind:    "Secret",
				ResourceName:    scope.AutoLoginConfig.EnvoySecretName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    secretShouldUpdate,
				UpdateFields:    secretUpdateFields,
			},
		},
	}
}

func secretShouldUpdate(current, desired *v1.Secret) bool {
	desiredTokenSecret, hasDesired := desired.Data[configpatch.TokenSecretFileName]
	currentTokenSecret, hasCurrent := current.Data[configpatch.TokenSecretFileName]
	return !hasDesired || !hasCurrent || !bytes.Equal(currentTokenSecret, desiredTokenSecret)
}

func secretUpdateFields(current, desired *v1.Secret) {
	current.Data = desired.Data
}

/*
envoyFilterReconcileAction reconciles an EnvoyFilter resource based on the configured AuthPolicy, enforcing auto-login
behavior for unauthenticated requests when enabled. The EnvoyFilter handles OAuth2 Authorization Code Flow.
*/
func envoyFilterReconcileAction(scope *state.Scope) AuthPolicyAdapter[*v1alpha4.EnvoyFilter] {
	autoLoginEnvoyFilterName := scope.AuthPolicy.Name + "-login"
	desiredResource := envoyfilter.GetDesired(
		scope,
		helperfunctions.BuildObjectMeta(autoLoginEnvoyFilterName, scope.AuthPolicy.Namespace),
	)

	return AuthPolicyAdapter[*v1alpha4.EnvoyFilter]{
		reconciliation.ReconcileFuncAdapter[*v1alpha4.EnvoyFilter]{
			Func: reconciliation.ReconcileFunc[*v1alpha4.EnvoyFilter]{
				ResourceKind:    "EnvoyFilter",
				ResourceName:    autoLoginEnvoyFilterName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    envoyFilterShouldUpdate,
				UpdateFields:    envoyFilterUpdateFields,
			},
		},
	}
}

func envoyFilterShouldUpdate(current, desired *v1alpha4.EnvoyFilter) bool {
	return !reflect.DeepEqual(
		current.Spec.GetWorkloadSelector(),
		desired.Spec.GetWorkloadSelector(),
	) || !reflect.DeepEqual(
		current.Spec.GetConfigPatches(),
		desired.Spec.GetConfigPatches(),
	)
}

func envoyFilterUpdateFields(current, desired *v1alpha4.EnvoyFilter) {
	current.Spec.WorkloadSelector = desired.Spec.GetWorkloadSelector()
	current.Spec.ConfigPatches = desired.Spec.GetConfigPatches()
}

/*
requestAuthenticationReconcileAction reconciles a RequestAuthentication resource based on the configured AuthPolicy,
defining the JWT authentication requirements and how to forward the original token and output claims to http headers.
*/
func requestAuthenticationReconcileAction(
	scope *state.Scope,
) AuthPolicyAdapter[*istioclientsecurityv1.RequestAuthentication] {
	requestAuthenticationName := scope.AuthPolicy.Name
	desiredResource := requestauthentication.GetDesired(
		scope,
		helperfunctions.BuildObjectMeta(requestAuthenticationName, scope.AuthPolicy.Namespace),
	)

	return AuthPolicyAdapter[*istioclientsecurityv1.RequestAuthentication]{
		reconciliation.ReconcileFuncAdapter[*istioclientsecurityv1.RequestAuthentication]{
			Func: reconciliation.ReconcileFunc[*istioclientsecurityv1.RequestAuthentication]{
				ResourceKind:    "RequestAuthentication",
				ResourceName:    requestAuthenticationName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    requestAuthenticationShouldUpdate,
				UpdateFields:    requestAuthenticationUpdateFields,
			},
		},
	}
}

func requestAuthenticationShouldUpdate(current, desired *istioclientsecurityv1.RequestAuthentication) bool {
	return !reflect.DeepEqual(current.Spec.GetSelector(), desired.Spec.GetSelector()) ||
		!reflect.DeepEqual(current.Spec.GetJwtRules(), desired.Spec.GetJwtRules())
}

func requestAuthenticationUpdateFields(current, desired *istioclientsecurityv1.RequestAuthentication) {
	current.Spec.Selector = desired.Spec.GetSelector()
	current.Spec.JwtRules = desired.Spec.GetJwtRules()
}

/*
denyAuthorizationPolicyReconcileAction reconciles DENY AuthorizationPolicy resources based on the configured AuthRules
and BaselineAuth, denying requests that do not satisfy the configured authentication requirements. DENY policies take
precedence over ALLOW policies.
*/
func denyAuthorizationPolicyReconcileAction(
	scope *state.Scope,
) AuthPolicyAdapter[*istioclientsecurityv1.AuthorizationPolicy] {
	denyAuthorizationPolicyName := scope.AuthPolicy.Name + "-deny-auth-rules"
	desiredResource := deny.GetDesired(
		scope,
		helperfunctions.BuildObjectMeta(denyAuthorizationPolicyName, scope.AuthPolicy.Namespace),
	)

	return AuthPolicyAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
		reconciliation.ReconcileFuncAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
			Func: reconciliation.ReconcileFunc[*istioclientsecurityv1.AuthorizationPolicy]{
				ResourceKind:    "AuthorizationPolicy",
				ResourceName:    denyAuthorizationPolicyName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    authorizationPolicyShouldUpdate,
				UpdateFields:    authorizationPolicyUpdateFields,
			},
		},
	}
}

/*
ignoreAuthorizationPolicyReconcileAction reconciles ALLOW AuthorizationPolicy resources based on the configured
IgnoreAuthRules, allowing requests that satisfy the configured authentication requirements unless denied by any DENY
policy.
*/
func ignoreAuthorizationPolicyReconcileAction(
	scope *state.Scope,
) AuthPolicyAdapter[*istioclientsecurityv1.AuthorizationPolicy] {
	ignoreAuthAuthorizationPolicyName := scope.AuthPolicy.Name + "-ignore-auth"
	desiredResource := ignore.GetDesired(
		scope,
		helperfunctions.BuildObjectMeta(ignoreAuthAuthorizationPolicyName, scope.AuthPolicy.Namespace),
	)

	return AuthPolicyAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
		reconciliation.ReconcileFuncAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
			Func: reconciliation.ReconcileFunc[*istioclientsecurityv1.AuthorizationPolicy]{
				ResourceKind:    "AuthorizationPolicy",
				ResourceName:    ignoreAuthAuthorizationPolicyName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    authorizationPolicyShouldUpdate,
				UpdateFields:    authorizationPolicyUpdateFields,
			},
		},
	}
}

/*
requireAuthorizationPolicyReconcileAction reconciles ALLOW AuthorizationPolicy resources based on the configured
AuthRules, BaselineAuth and IgnoreAuthRules, allowing requests that satisfy the configured authentication requirements
unless denied by any DENY policy.
*/
func requireAuthorizationPolicyReconcileAction(
	scope *state.Scope,
) AuthPolicyAdapter[*istioclientsecurityv1.AuthorizationPolicy] {
	requireAuthAuthorizationPolicyName := scope.AuthPolicy.Name + "-require-auth"
	desiredResource := require.GetDesired(
		scope,
		helperfunctions.BuildObjectMeta(requireAuthAuthorizationPolicyName, scope.AuthPolicy.Namespace),
	)

	return AuthPolicyAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
		reconciliation.ReconcileFuncAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
			Func: reconciliation.ReconcileFunc[*istioclientsecurityv1.AuthorizationPolicy]{
				ResourceKind:    "AuthorizationPolicy",
				ResourceName:    requireAuthAuthorizationPolicyName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    authorizationPolicyShouldUpdate,
				UpdateFields:    authorizationPolicyUpdateFields,
			},
		},
	}
}

func authorizationPolicyShouldUpdate(current, desired *istioclientsecurityv1.AuthorizationPolicy) bool {
	return !reflect.DeepEqual(current.Spec.GetSelector(), desired.Spec.GetSelector()) ||
		!reflect.DeepEqual(current.Spec.GetRules(), desired.Spec.GetRules())
}

func authorizationPolicyUpdateFields(current, desired *istioclientsecurityv1.AuthorizationPolicy) {
	current.Spec.Selector = desired.Spec.GetSelector()
	current.Spec.Rules = desired.Spec.GetRules()
}
