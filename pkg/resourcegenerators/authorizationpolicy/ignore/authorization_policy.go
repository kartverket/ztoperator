package ignore

import (
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy"
	"istio.io/api/security/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.AuthorizationPolicy {
	if scope.IsMisconfigured() {
		// We rely on deny.authorizationpolicy to create an auth policy which block all requests.
		return nil
	}

	ignoreAuthRequestMatchers := scope.AuthPolicy.GetIgnoreAuthRequestMatchers()

	if len(ignoreAuthRequestMatchers) == 0 {
		// No IgnoreAuthRules defined, thus no allow rules to create
		return nil
	}

	// Create allow rules based on the specified ignore auth rules

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
