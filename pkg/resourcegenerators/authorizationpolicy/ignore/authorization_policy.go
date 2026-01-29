package ignore

import (
	"github.com/kartverket/ztoperator/internal/state"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.AuthorizationPolicy {
	if scope.IsMisconfigured() {
		// We rely on deny.authorizationpolicy to create an auth policy which block all requests.
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
	return authorizationpolicy.AllowAuthorizationPolicy(
		scope,
		objectMeta,
		[]*v1beta1.Rule{
			{
				To: ruleToList,
			},
		},
	)
}
