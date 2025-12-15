package authorizationpolicy

import (
	"github.com/kartverket/ztoperator/internal/state"
	"istio.io/api/security/v1beta1"
)

func GetBaseConditions(acceptedResources []string, issuer string, notValues bool) []*v1beta1.Condition {
	makeCondition := func(key string, values []string) *v1beta1.Condition {
		if notValues {
			return &v1beta1.Condition{
				Key:       key,
				NotValues: values,
			}
		}
		return &v1beta1.Condition{
			Key:    key,
			Values: values,
		}
	}

	conditions := []*v1beta1.Condition{
		makeCondition("request.auth.claims[iss]", []string{issuer}),
	}

	if len(acceptedResources) > 0 {
		conditions = append(conditions, makeCondition("request.auth.claims[aud]", acceptedResources))
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
