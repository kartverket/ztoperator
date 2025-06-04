package deny

import (
	"fmt"
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type AuthRuleDeny struct {
	Path   string
	Method string
	When   []v1alpha1.Condition
}

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.AuthorizationPolicy {
	if !scope.AuthPolicy.Spec.Enabled {
		return nil
	}

	if !scope.HasValidPaths {
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

	baseConditions := authorizationpolicy.GetBaseConditions(*scope.AuthPolicy, true)
	if scope.AuthPolicy.Spec.AuthRules != nil {
		flattenedAuthRules := flattenAuthRules(*scope.AuthPolicy.Spec.AuthRules)
		for _, rule := range flattenedAuthRules {
			authPolicyConditionsAsIstioConditions := baseConditions
			for _, condition := range rule.When {
				authPolicyConditionsAsIstioConditions = append(authPolicyConditionsAsIstioConditions, &v1beta1.Condition{
					Key:       fmt.Sprintf("request.auth.claims[%s]", condition.Claim),
					NotValues: condition.Values,
				})
			}

			ruleToOperation := v1beta1.Operation{
				Paths:   []string{rule.Path},
				Methods: []string{rule.Method},
			}

			for _, otherRule := range flattenedAuthRules {
				if otherRule.IsSubSetOf(rule) {
					ruleToOperation.NotPaths = append(ruleToOperation.NotPaths, otherRule.Path)
				}
			}

			for _, istioCondition := range authPolicyConditionsAsIstioConditions {
				denyRules = append(denyRules, &v1beta1.Rule{
					To: []*v1beta1.Rule_To{
						{
							Operation: &ruleToOperation,
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

func flattenAuthRules(authRules []v1alpha1.RequestAuthRule) []AuthRuleDeny {
	var flattenedAuthRules []AuthRuleDeny
	for _, authRule := range authRules {
		methods := authRule.Methods
		if len(methods) == 0 {
			methods = v1alpha1.AcceptedHttpMethods
		}

		for _, method := range methods {
			for _, path := range authRule.Paths {
				flattenedAuthRules = append(flattenedAuthRules, AuthRuleDeny{
					Path:   path,
					Method: method,
					When:   authRule.When,
				})
			}
		}
	}
	return flattenedAuthRules
}

func (rule AuthRuleDeny) IsSubSetOf(otherRule AuthRuleDeny) bool {

	//TODO: Denne må oppdateres til å finne subset mellom to paths både med den gamle path syntaxen OG den n

	if rule.Method != otherRule.Method {
		return false
	}
	if rule.Path == otherRule.Path {
		return false
	}

	prefix := func(path string) string {
		if i := strings.Index(path, "*"); i >= 0 {
			return path[:i]
		}
		return path
	}

	rulePrefix := prefix(rule.Path)
	otherPrefix := prefix(otherRule.Path)

	if rulePrefix != otherPrefix {
		return false
	}

	return len(rule.Path) > len(otherRule.Path)
}
