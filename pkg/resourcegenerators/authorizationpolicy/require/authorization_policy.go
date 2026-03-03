package require

import (
	"fmt"
	"slices"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy"
	"github.com/kartverket/ztoperator/pkg/validation"
	"istio.io/api/security/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.AuthorizationPolicy {
	if !scope.AuthPolicy.Spec.Enabled {
		return nil
	}

	if scope.InvalidConfig {
		// We rely on deny.authorizationpolicy to create an auth policy which block all requests.
		return nil
	}

	baseConditions := constructBaseConditions(scope)

	hasAuthRules := scope.AuthPolicy.Spec.AuthRules != nil && len(*scope.AuthPolicy.Spec.AuthRules) > 0
	hasIgnoreAuthRules := scope.AuthPolicy.Spec.IgnoreAuthRules != nil &&
		len(*scope.AuthPolicy.Spec.IgnoreAuthRules) > 0

	if !hasAuthRules && !hasIgnoreAuthRules {
		// If there are no specific auth rules or ignore rules, we create an allow-rule
		// matching all paths and all methods, with validation of audience and issuer.
		allPathsRule := []*v1beta1.Rule{
			{
				To: []*v1beta1.Rule_To{
					{
						Operation: &v1beta1.Operation{
							Paths: []string{"*"},
						},
					},
				},
				When: baseConditions,
			},
		}
		return authorizationpolicy.AllowAuthorizationPolicy(scope, objectMeta, allPathsRule)
	}

	specifiedPathsRules := constructSpecifiedPathsAllowRules(scope, baseConditions)

	unspecifiedPathsRule := constructUnspecifiedPathsAllowRule(scope, baseConditions)

	allAllowRules := append([]*v1beta1.Rule{unspecifiedPathsRule}, specifiedPathsRules...)
	return authorizationpolicy.AllowAuthorizationPolicy(
		scope,
		objectMeta,
		allAllowRules,
	)
}

/*
Audience and issuer conditions are always included as base conditions.
Additionally, any conditions specified as baseline auth are also included.
*/
func constructBaseConditions(scope *state.Scope) []*v1beta1.Condition {
	audienceAndIssuerConditions := authorizationpolicy.GetAudienceAndIssuerConditionsForAllowPolicy(
		authorizationpolicy.ConstructAcceptedResources(*scope),
		scope.IdentityProviderUris.IssuerURI,
	)
	baselineAuthConditions := authorizationpolicy.GetBaselineAuthConditionsForAllowPolicy(
		scope.AuthPolicy.Spec.BaselineAuth,
	)

	allBaseConditions := slices.Concat(audienceAndIssuerConditions, baselineAuthConditions)
	return allBaseConditions
}

/*
Each auth rule should result in an allow rule for the specified paths, methods and conditions.
Additionally, the audience and issuer conditions are always included.
*/
func constructSpecifiedPathsAllowRules(
	scope *state.Scope,
	audienceAndIssuerConditions []*v1beta1.Condition,
) []*v1beta1.Rule {
	var specifiedPathsAllowRules []*v1beta1.Rule
	if scope.AuthPolicy.Spec.AuthRules != nil {
		for _, authRule := range *scope.AuthPolicy.Spec.AuthRules {
			authPolicyConditionsAsIstioConditions := audienceAndIssuerConditions
			if authRule.When != nil {
				for _, condition := range *authRule.When {
					authPolicyConditionsAsIstioConditions = append(
						authPolicyConditionsAsIstioConditions,
						&v1beta1.Condition{
							Key:    fmt.Sprintf("request.auth.claims[%s]", condition.Claim),
							Values: condition.Values,
						},
					)
				}
			}
			specifiedPathsAllowRules = append(specifiedPathsAllowRules, &v1beta1.Rule{
				To: []*v1beta1.Rule_To{
					{
						Operation: &v1beta1.Operation{
							Paths:   validation.TransformPathsForIstio(authRule.Paths),
							Methods: authRule.Methods,
						},
					},
				},
				When: authPolicyConditionsAsIstioConditions,
			})
		}
	}
	return specifiedPathsAllowRules
}

/*
All paths and methods not explicitly specified in any auth rule or ignore auth rule
should result in an allow rule with only audience and issuer conditions.
*/
func constructUnspecifiedPathsAllowRule(
	scope *state.Scope,
	audienceAndIssuerConditions []*v1beta1.Condition,
) *v1beta1.Rule {
	allRequestMatchers := append(
		scope.AuthPolicy.GetRequireAuthRequestMatchers(),
		scope.AuthPolicy.GetIgnoreAuthRequestMatchers()...,
	)

	// +1 for the rule that allows all paths and methods not defined in any matcher
	unspecifiedPathsRuleList := make([]*v1beta1.Rule_To, 0, len(allRequestMatchers)+1)

	// For all request matchers, create to-rules for all methods not defined in the matcher
	for _, matcher := range allRequestMatchers {
		methods := matcher.Methods
		if len(matcher.Methods) == 0 {
			methods = v1alpha1.GetAcceptedHTTPMethods()
		}
		unspecifiedPathsRuleList = append(unspecifiedPathsRuleList, &v1beta1.Rule_To{
			Operation: &v1beta1.Operation{
				Paths:      validation.TransformPathsForIstio(matcher.Paths),
				NotMethods: methods, // NB: NotMethods used to create to-rules for all methods not defined in a matcher
			},
		})
	}

	// For all request matchers, create to-rules for all paths not defined in a matcher
	mentionedPaths := make([]string, 0, len(allRequestMatchers))
	for _, matcher := range allRequestMatchers {
		mentionedPaths = append(mentionedPaths, validation.TransformPathsForIstio(matcher.Paths)...)
	}
	unspecifiedPathsRuleList = append(unspecifiedPathsRuleList, &v1beta1.Rule_To{
		Operation: &v1beta1.Operation{
			Paths:    []string{"*"},
			NotPaths: mentionedPaths, // NB! NotPaths used to create to-rule for all paths not defined in any matcher
		},
	})

	// Create allow rule for all unspecified paths and methods, with audience and issuer conditions
	unspecifiedPathsAllowRule := &v1beta1.Rule{
		To:   unspecifiedPathsRuleList,
		When: audienceAndIssuerConditions,
	}
	return unspecifiedPathsAllowRule
}
