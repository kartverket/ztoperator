package ignore_auth

import (
	"github.com/kartverket/ztoperator/internal/state"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.AuthorizationPolicy {
	if !scope.AuthPolicy.Spec.Enabled || !scope.HasValidPaths {
		return nil
	}

	ignoreAuthRequestMatchers := scope.AuthPolicy.GetIgnoreAuthRequestMatchers()

	var ruleToList []*v1beta1.Rule_To

	for _, ignoreAuthRequestMatcher := range ignoreAuthRequestMatchers {
		ruleTo := &v1beta1.Rule_To{
			Operation: &v1beta1.Operation{
				Paths:   ignoreAuthRequestMatcher.Paths,
				Methods: ignoreAuthRequestMatcher.Methods,
			},
		}
		ruleToList = append(ruleToList, ruleTo)
	}

	if len(ruleToList) > 0 {
		return &istioclientsecurityv1.AuthorizationPolicy{
			ObjectMeta: objectMeta,
			Spec: v1beta1.AuthorizationPolicy{
				Selector: &v1beta2.WorkloadSelector{
					MatchLabels: scope.AuthPolicy.Spec.Selector.MatchLabels,
				},
				Rules: []*v1beta1.Rule{
					{
						To: ruleToList,
					},
				},
			},
		}
	}
	return nil
}
