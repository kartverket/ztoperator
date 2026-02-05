package deny

import (
	"fmt"
	"slices"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy"
	"istio.io/api/security/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.AuthorizationPolicy {
	if !scope.AuthPolicy.Spec.Enabled {
		// AuthPolicy disabled, no deny rules to create
		return nil
	}

	if scope.InvalidConfig {
		// We deny requests for all paths if the configuration is invalid
		allPathsRule := []*v1beta1.Rule{
			{
				To: []*v1beta1.Rule_To{
					{
						Operation: &v1beta1.Operation{
							Paths: []string{"*"},
						},
					},
				},
			},
		}
		return authorizationpolicy.DenyAuthorizationPolicy(scope, objectMeta, allPathsRule)
	}

	if scope.AuthPolicy.Spec.AuthRules == nil || len(*scope.AuthPolicy.Spec.AuthRules) == 0 {
		// No AuthRules defined, thus no deny rules to create
		return nil
	}

	// Create deny rules based on the specified auth rules

	baseDenyConditions := constructBaseConditionsForDenyPolicy(scope)

	var denyRules []*v1beta1.Rule
	for _, rule := range *scope.AuthPolicy.Spec.AuthRules {
		// Audience and issuer conditions are always included
		authPolicyDenyConditions := baseDenyConditions
		// Additional conditions from the "when" clause
		if rule.When != nil {
			for _, condition := range *rule.When {
				authPolicyDenyConditions = append(
					authPolicyDenyConditions,
					&v1beta1.Condition{
						Key:       fmt.Sprintf("request.auth.claims[%s]", condition.Claim),
						NotValues: condition.Values, // NB! NotValues used in combination with deny rule
					},
				)
			}
		}
		// Create one rule per condition
		for _, istioCondition := range authPolicyDenyConditions {
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

	return authorizationpolicy.DenyAuthorizationPolicy(scope, objectMeta, denyRules)
}

/*
Audience and issuer conditions are always included as base conditions.
Additionally, any conditions specified as baseline auth are also included.
*/
func constructBaseConditionsForDenyPolicy(scope *state.Scope) []*v1beta1.Condition {
	audienceAndIssuerConditions := authorizationpolicy.GetAudienceAndIssuerConditionsForDenyPolicy(
		authorizationpolicy.ConstructAcceptedResources(*scope),
		scope.IdentityProviderUris.IssuerURI,
	)
	baselineAuthConditions := authorizationpolicy.GetBaselineAuthConditionsForDenyPolicy(
		scope.AuthPolicy.Spec.BaselineAuth,
	)
	allBaseConditions := slices.Concat(audienceAndIssuerConditions, baselineAuthConditions)
	return allBaseConditions
}
