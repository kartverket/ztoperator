package reconciler

import (
	"bytes"
	"reflect"

	"github.com/kartverket/ztoperator/internal/names"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/labels"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ControllerResources creates all reconcile actions for the given AuthPolicy scope.
func ControllerResources(scope *state.Scope) []reconciliation.ControllerResource {
	return []reconciliation.ControllerResource{
		secretResource(scope),
		envoyFilterResource(scope),
		requestAuthenticationResource(scope),
		denyAuthorizationPolicyResource(scope),
		ignoreAuthorizationPolicyResource(scope),
		requireAuthorizationPolicyResource(scope),
	}
}

/*
secretResource reconciles a Secret resource containing a HMAC secret (cookie signing key) and token secret
(OAuth client secret), if auto-login is enabled. The secrets are used by Envoy during Authorization Code Flow.
*/
func secretResource(scope *state.Scope) ControllerResourceAdapter[*v1.Secret] {
	desiredResource := secret.GetDesired(
		scope,
		buildObjectMeta(scope.AutoLoginConfig.EnvoySecretName, scope.AuthPolicy.Namespace),
	)

	return ControllerResourceAdapter[*v1.Secret]{
		reconciliation.ReconcilerAdapter[*v1.Secret]{
			Func: reconciliation.ResourceReconciler[*v1.Secret]{
				ResourceKind:    "Secret",
				ResourceName:    scope.AutoLoginConfig.EnvoySecretName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    SecretShouldUpdate,
				UpdateFields:    SecretUpdateFields,
			},
		},
	}
}

func SecretShouldUpdate(current, desired *v1.Secret) bool {
	desiredTokenSecret, hasDesired := desired.Data[configpatch.TokenSecretFileName]
	currentTokenSecret, hasCurrent := current.Data[configpatch.TokenSecretFileName]
	return !hasDesired || !hasCurrent || !bytes.Equal(currentTokenSecret, desiredTokenSecret) ||
		labelsNeedUpdate(current, desired)
}

func SecretUpdateFields(current, desired *v1.Secret) {
	current.Data = desired.Data
	current.Labels = desired.Labels
}

/*
envoyFilterResource reconciles an EnvoyFilter resource based on the configured AuthPolicy, enforcing auto-login
behavior for unauthenticated requests when enabled. The EnvoyFilter handles OAuth2 Authorization Code Flow.
*/
func envoyFilterResource(scope *state.Scope) ControllerResourceAdapter[*v1alpha4.EnvoyFilter] {
	autoLoginEnvoyFilterName := names.EnvoyFilter(scope.AuthPolicy.Name)
	desiredResource := envoyfilter.GetDesired(
		scope,
		buildObjectMeta(autoLoginEnvoyFilterName, scope.AuthPolicy.Namespace),
	)

	return ControllerResourceAdapter[*v1alpha4.EnvoyFilter]{
		reconciliation.ReconcilerAdapter[*v1alpha4.EnvoyFilter]{
			Func: reconciliation.ResourceReconciler[*v1alpha4.EnvoyFilter]{
				ResourceKind:    "EnvoyFilter",
				ResourceName:    autoLoginEnvoyFilterName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    EnvoyFilterShouldUpdate,
				UpdateFields:    EnvoyFilterUpdateFields,
			},
		},
	}
}

func EnvoyFilterShouldUpdate(current, desired *v1alpha4.EnvoyFilter) bool {
	return !reflect.DeepEqual(
		current.Spec.GetWorkloadSelector(),
		desired.Spec.GetWorkloadSelector(),
	) || !reflect.DeepEqual(
		current.Spec.GetConfigPatches(),
		desired.Spec.GetConfigPatches(),
	) ||
		labelsNeedUpdate(current, desired)
}

func EnvoyFilterUpdateFields(current, desired *v1alpha4.EnvoyFilter) {
	current.Spec.WorkloadSelector = desired.Spec.GetWorkloadSelector()
	current.Spec.ConfigPatches = desired.Spec.GetConfigPatches()
	current.Labels = desired.Labels
}

/*
requestAuthenticationResource reconciles a RequestAuthentication resource based on the configured AuthPolicy,
defining the JWT authentication requirements and how to forward the original token and output claims to http headers.
*/
func requestAuthenticationResource(
	scope *state.Scope,
) ControllerResourceAdapter[*istioclientsecurityv1.RequestAuthentication] {
	requestAuthenticationName := scope.AuthPolicy.Name
	desiredResource := requestauthentication.GetDesired(
		scope,
		buildObjectMeta(requestAuthenticationName, scope.AuthPolicy.Namespace),
	)

	return ControllerResourceAdapter[*istioclientsecurityv1.RequestAuthentication]{
		reconciliation.ReconcilerAdapter[*istioclientsecurityv1.RequestAuthentication]{
			Func: reconciliation.ResourceReconciler[*istioclientsecurityv1.RequestAuthentication]{
				ResourceKind:    "RequestAuthentication",
				ResourceName:    requestAuthenticationName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    RequestAuthenticationShouldUpdate,
				UpdateFields:    RequestAuthenticationUpdateFields,
			},
		},
	}
}

func RequestAuthenticationShouldUpdate(current, desired *istioclientsecurityv1.RequestAuthentication) bool {
	return !reflect.DeepEqual(current.Spec.GetSelector(), desired.Spec.GetSelector()) ||
		!reflect.DeepEqual(current.Spec.GetJwtRules(), desired.Spec.GetJwtRules()) ||
		labelsNeedUpdate(current, desired)
}

func RequestAuthenticationUpdateFields(current, desired *istioclientsecurityv1.RequestAuthentication) {
	current.Spec.Selector = desired.Spec.GetSelector()
	current.Spec.JwtRules = desired.Spec.GetJwtRules()
	current.Labels = desired.Labels
}

/*
denyAuthorizationPolicyResource reconciles DENY AuthorizationPolicy resources based on the configured AuthRules
and BaselineAuth, denying requests that do not satisfy the configured authentication requirements. DENY policies take
precedence over ALLOW policies.
*/
func denyAuthorizationPolicyResource(
	scope *state.Scope,
) ControllerResourceAdapter[*istioclientsecurityv1.AuthorizationPolicy] {
	denyAuthorizationPolicyName := names.DenyPolicy(scope.AuthPolicy.Name)
	desiredResource := deny.GetDesired(
		scope,
		buildObjectMeta(denyAuthorizationPolicyName, scope.AuthPolicy.Namespace),
	)

	return ControllerResourceAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
		reconciliation.ReconcilerAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
			Func: reconciliation.ResourceReconciler[*istioclientsecurityv1.AuthorizationPolicy]{
				ResourceKind:    "AuthorizationPolicy",
				ResourceName:    denyAuthorizationPolicyName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    AuthorizationPolicyShouldUpdate,
				UpdateFields:    AuthorizationPolicyUpdateFields,
			},
		},
	}
}

/*
ignoreAuthorizationPolicyResource reconciles ALLOW AuthorizationPolicy resources based on the configured
IgnoreAuthRules, allowing requests that satisfy the configured authentication requirements unless denied by any DENY
policy.
*/
func ignoreAuthorizationPolicyResource(
	scope *state.Scope,
) ControllerResourceAdapter[*istioclientsecurityv1.AuthorizationPolicy] {
	ignoreAuthAuthorizationPolicyName := names.IgnorePolicy(scope.AuthPolicy.Name)
	desiredResource := ignore.GetDesired(
		scope,
		buildObjectMeta(ignoreAuthAuthorizationPolicyName, scope.AuthPolicy.Namespace),
	)

	return ControllerResourceAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
		reconciliation.ReconcilerAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
			Func: reconciliation.ResourceReconciler[*istioclientsecurityv1.AuthorizationPolicy]{
				ResourceKind:    "AuthorizationPolicy",
				ResourceName:    ignoreAuthAuthorizationPolicyName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    AuthorizationPolicyShouldUpdate,
				UpdateFields:    AuthorizationPolicyUpdateFields,
			},
		},
	}
}

/*
requireAuthorizationPolicyResource reconciles ALLOW AuthorizationPolicy resources based on the configured
AuthRules, BaselineAuth and IgnoreAuthRules, allowing requests that satisfy the configured authentication requirements
unless denied by any DENY policy.
*/
func requireAuthorizationPolicyResource(
	scope *state.Scope,
) ControllerResourceAdapter[*istioclientsecurityv1.AuthorizationPolicy] {
	requireAuthAuthorizationPolicyName := names.RequirePolicy(scope.AuthPolicy.Name)
	desiredResource := require.GetDesired(
		scope,
		buildObjectMeta(requireAuthAuthorizationPolicyName, scope.AuthPolicy.Namespace),
	)

	return ControllerResourceAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
		reconciliation.ReconcilerAdapter[*istioclientsecurityv1.AuthorizationPolicy]{
			Func: reconciliation.ResourceReconciler[*istioclientsecurityv1.AuthorizationPolicy]{
				ResourceKind:    "AuthorizationPolicy",
				ResourceName:    requireAuthAuthorizationPolicyName,
				DesiredResource: helperfunctions.Ptr(desiredResource),
				Scope:           scope,
				ShouldUpdate:    AuthorizationPolicyShouldUpdate,
				UpdateFields:    AuthorizationPolicyUpdateFields,
			},
		},
	}
}

func AuthorizationPolicyShouldUpdate(current, desired *istioclientsecurityv1.AuthorizationPolicy) bool {
	return !reflect.DeepEqual(current.Spec.GetSelector(), desired.Spec.GetSelector()) ||
		!reflect.DeepEqual(current.Spec.GetRules(), desired.Spec.GetRules()) ||
		labelsNeedUpdate(current, desired)
}

func AuthorizationPolicyUpdateFields(current, desired *istioclientsecurityv1.AuthorizationPolicy) {
	current.Spec.Selector = desired.Spec.GetSelector()
	current.Spec.Rules = desired.Spec.GetRules()
	current.Labels = desired.Labels
}

type LabeledObject interface {
	GetLabels() map[string]string
}

// labelsNeedUpdate reports whether any of the desired labels are missing from current or have a
// different value. Labels on current that are not part of the desired set are ignored.
func labelsNeedUpdate(current, desired LabeledObject) bool {
	desiredLabels := desired.GetLabels()
	currentLabels := current.GetLabels()
	for key, value := range desiredLabels {
		if currentValue, ok := currentLabels[key]; !ok || currentValue != value {
			return true
		}
	}
	return false
}

func buildObjectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    labels.AuthPolicyStandardLabels(),
	}
}
