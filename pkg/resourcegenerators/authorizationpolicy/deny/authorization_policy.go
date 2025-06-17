package deny

import (
	"fmt"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.AuthorizationPolicy {
	if !scope.AuthPolicy.Spec.Enabled {
		return nil
	}

	if scope.InvalidConfig {
		return &istioclientsecurityv1.AuthorizationPolicy{
			ObjectMeta: objectMeta,
			Spec: v1beta1.AuthorizationPolicy{
				Action: v1beta1.AuthorizationPolicy_DENY,
				Selector: &v1beta2.WorkloadSelector{
					MatchLabels: scope.AuthPolicy.Spec.Selector.MatchLabels,
				},
				Rules: []*v1beta1.Rule{
					{
						To: []*v1beta1.Rule_To{
							{
								Operation: &v1beta1.Operation{
									Paths: []string{"*"},
								},
							},
						},
					},
				},
			},
		}
	}

	var denyRules []*v1beta1.Rule

	baseConditions := authorizationpolicy.GetBaseConditions(
		scope.AuthPolicy,
		scope.IdentityProviderUris.IssuerURI,
		true,
	)
	if scope.AuthPolicy.Spec.AuthRules != nil {
		for _, rule := range *scope.AuthPolicy.Spec.AuthRules {
			authPolicyConditionsAsIstioConditions := baseConditions
			for _, condition := range rule.When {
				authPolicyConditionsAsIstioConditions = append(
					authPolicyConditionsAsIstioConditions,
					&v1beta1.Condition{
						Key:       fmt.Sprintf("request.auth.claims[%s]", condition.Claim),
						NotValues: condition.Values,
					},
				)
			}

			for _, istioCondition := range authPolicyConditionsAsIstioConditions {
				denyRules = append(denyRules, &v1beta1.Rule{
					To: []*v1beta1.Rule_To{
						{
							Operation: &v1beta1.Operation{
								Paths:   rule.Paths,
								Methods: rule.Methods,
							},
						},
					},
					When: []*v1beta1.Condition{istioCondition},
				})
			}
		}
	}

	if len(denyRules) > 0 {
		return &istioclientsecurityv1.AuthorizationPolicy{
			ObjectMeta: objectMeta,
			Spec: v1beta1.AuthorizationPolicy{
				Action: v1beta1.AuthorizationPolicy_DENY,
				Selector: &v1beta2.WorkloadSelector{
					MatchLabels: scope.AuthPolicy.Spec.Selector.MatchLabels,
				},
				Rules: denyRules,
			},
		}
	}
	return nil
}
