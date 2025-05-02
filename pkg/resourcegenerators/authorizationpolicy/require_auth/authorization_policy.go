package require_auth

import (
	"fmt"
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *ztoperatorv1alpha1.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.AuthorizationPolicy {
	var authorizationPolicyRules []*v1beta1.Rule
	for _, resolvedRule := range scope.ResolvedAuthPolicy.ResolvedRules {
		baseConditions := getBaseConditions(resolvedRule)
		if len(*resolvedRule.Rule.AuthRules)+len(*resolvedRule.Rule.IgnoreAuthRules) == 0 {
			authorizationPolicyRules = append(authorizationPolicyRules, &v1beta1.Rule{
				To: []*v1beta1.Rule_To{
					{
						Operation: &v1beta1.Operation{
							Paths: []string{"*"},
						},
					},
				},
				When: baseConditions,
			})
		} else {
			// The first rule requires auth on all methods/paths combinations not covered by the provided rules by default.
			authorizationPolicyRules = append(authorizationPolicyRules, &v1beta1.Rule{
				To: authorizationpolicy.GetApiSurfaceDiffAsRuleToList(
					ztoperatorv1alpha1.RequestMatcherList{
						{
							Paths: []string{"*"},
						},
					},
					append(resolvedRule.Rule.AuthRules.GetRequestMatchers(), *resolvedRule.Rule.IgnoreAuthRules...),
				),
			})
			for _, authRule := range *resolvedRule.Rule.AuthRules {
				var authPolicyConditionsAsIstioConditions []*v1beta1.Condition
				for _, condition := range authRule.When {
					authPolicyConditionsAsIstioConditions = append(authPolicyConditionsAsIstioConditions, &v1beta1.Condition{
						Key:    fmt.Sprintf("request.auth.claims[%s]", condition.Claim),
						Values: condition.Values,
					})
				}
				authorizationPolicyRules = append(authorizationPolicyRules, &v1beta1.Rule{
					To: []*v1beta1.Rule_To{
						{
							Operation: &v1beta1.Operation{
								Paths:   authRule.Paths,
								Methods: authRule.Methods,
							},
						},
					},
					When: append(baseConditions, authPolicyConditionsAsIstioConditions...),
				})
			}
		}
	}
	return &istioclientsecurityv1.AuthorizationPolicy{
		ObjectMeta: objectMeta,
		Spec: v1beta1.AuthorizationPolicy{
			Selector: &v1beta2.WorkloadSelector{
				MatchLabels: scope.ResolvedAuthPolicy.AuthPolicy.Spec.Selector.MatchLabels,
			},
			Rules: authorizationPolicyRules,
		},
	}
}

func getBaseConditions(resolvedRule ztoperatorv1alpha1.ResolvedRule) []*v1beta1.Condition {
	return []*v1beta1.Condition{
		{
			Key:    "request.auth.claims[iss]",
			Values: []string{resolvedRule.IssuerUri},
		},
		{
			Key:    "request.auth.claims[aud]",
			Values: resolvedRule.Audiences,
		},
	}
}
