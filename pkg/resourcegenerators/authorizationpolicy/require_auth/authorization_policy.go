package require_auth

import (
	"fmt"
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.AuthorizationPolicy {
	var authorizationPolicyRules []*v1beta1.Rule

	for _, jwtRule := range scope.AuthPolicy.Spec.Rules {
		baseConditions := authorizationpolicy.GetBaseConditions(jwtRule, false)

		var toList []*v1beta1.Rule_To
		var mentionedPaths []string
		for _, matcher := range append(
			scope.AuthPolicy.GetRequireAuthRequestMatchers(),
			scope.AuthPolicy.GetIgnoreAuthRequestMatchers()...,
		) {
			mentionedPaths = append(mentionedPaths, matcher.Paths...)
			methods := matcher.Methods
			if len(matcher.Methods) == 0 {
				methods = v1alpha1.AcceptedHttpMethods
			}
			toList = append(toList, &v1beta1.Rule_To{
				Operation: &v1beta1.Operation{
					Paths:      matcher.Paths,
					NotMethods: methods,
				},
			})
		}

		toList = append(toList, &v1beta1.Rule_To{
			Operation: &v1beta1.Operation{
				Paths:    []string{"*"},
				NotPaths: mentionedPaths,
			},
		})

		authorizationPolicyRules = append(authorizationPolicyRules, &v1beta1.Rule{
			To:   toList,
			When: baseConditions,
		})
	}

	for _, jwtRule := range scope.AuthPolicy.Spec.Rules {
		baseConditions := authorizationpolicy.GetBaseConditions(jwtRule, false)

		if (jwtRule.AuthRules == nil || len(*jwtRule.AuthRules) == 0) &&
			(jwtRule.IgnoreAuthRules == nil || len(*jwtRule.IgnoreAuthRules) == 0) {
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
			for _, authRule := range *jwtRule.AuthRules {
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
				MatchLabels: scope.AuthPolicy.Spec.Selector.MatchLabels,
			},
			Rules: authorizationPolicyRules,
		},
	}
}
