package authorizationpolicy

import (
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"istio.io/api/security/v1beta1"
)

func GetBaseConditions(authPolicy ztoperatorv1alpha1.AuthPolicy, issuer string, notValues bool) []*v1beta1.Condition {
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
		makeCondition("request.auth.claims[aud]", authPolicy.Spec.Audience),
	}

	if authPolicy.Spec.AcceptedResources != nil {
		conditions = append(conditions, makeCondition("request.auth.claims[aud]", *authPolicy.Spec.AcceptedResources))
	}
	return conditions
}
