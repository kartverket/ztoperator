package authorizationpolicy

import (
	"github.com/kartverket/ztoperator/internal/state"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAudienceAndIssuerConditionsForAllowPolicy(acceptedResources []string, issuer string) []*v1beta1.Condition {
	makeCondition := func(key string, values []string) *v1beta1.Condition {
		return &v1beta1.Condition{
			Key:    key,
			Values: values,
		}
	}
	return getAudienceAndIssuerConditions(acceptedResources, issuer, makeCondition)
}

func GetAudienceAndIssuerConditionsForDenyPolicy(acceptedResources []string, issuer string) []*v1beta1.Condition {
	makeCondition := func(key string, values []string) *v1beta1.Condition {
		return &v1beta1.Condition{
			Key:       key,
			NotValues: values, // NB! NotValues used in combination with deny rule
		}
	}
	return getAudienceAndIssuerConditions(acceptedResources, issuer, makeCondition)
}

func getAudienceAndIssuerConditions(
	acceptedResources []string,
	issuer string,
	makeConditionFunc func(key string, values []string) *v1beta1.Condition,
) []*v1beta1.Condition {
	var conditions []*v1beta1.Condition
	conditions = append(conditions, makeConditionFunc("request.auth.claims[iss]", []string{issuer}))
	if len(acceptedResources) > 0 {
		conditions = append(conditions, makeConditionFunc("request.auth.claims[aud]", acceptedResources))
	}
	return conditions
}

func ConstructAcceptedResources(scope state.Scope) []string {
	var acceptedResources []string
	acceptedResources = append(acceptedResources, scope.Audiences...)

	if scope.AuthPolicy.Spec.AcceptedResources != nil {
		acceptedResources = append(acceptedResources, *scope.AuthPolicy.Spec.AcceptedResources...)
	}
	return acceptedResources
}

func AllowAuthorizationPolicy(
	scope *state.Scope,
	objectMeta v1.ObjectMeta,
	allowRules []*v1beta1.Rule,
) *istioclientsecurityv1.AuthorizationPolicy {
	return authorizationPolicy(
		scope,
		objectMeta,
		v1beta1.AuthorizationPolicy_ALLOW,
		allowRules,
	)
}

func DenyAuthorizationPolicy(
	scope *state.Scope,
	objectMeta v1.ObjectMeta,
	denyRules []*v1beta1.Rule,
) *istioclientsecurityv1.AuthorizationPolicy {
	return authorizationPolicy(
		scope,
		objectMeta,
		v1beta1.AuthorizationPolicy_DENY,
		denyRules,
	)
}

func authorizationPolicy(
	scope *state.Scope,
	objectMeta v1.ObjectMeta,
	action v1beta1.AuthorizationPolicy_Action,
	rules []*v1beta1.Rule,
) *istioclientsecurityv1.AuthorizationPolicy {
	return &istioclientsecurityv1.AuthorizationPolicy{
		ObjectMeta: objectMeta,
		Spec: v1beta1.AuthorizationPolicy{
			Action: action,
			Selector: &v1beta2.WorkloadSelector{
				MatchLabels: scope.AuthPolicy.Spec.Selector.MatchLabels,
			},
			Rules: rules,
		},
	}
}
