package authorizationpolicy

import (
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"istio.io/api/security/v1beta1"
)

func GetApiSurfaceDiffAsRuleToList(requestMatchers, otherRequestMatchers []v1alpha1.RequestMatcher) []*v1beta1.Rule_To {
	var diff []*v1beta1.Rule_To
	for _, requestMatcher := range requestMatchers {
		ruleTo := &v1beta1.Rule_To{
			Operation: &v1beta1.Operation{
				Paths:   requestMatcher.Paths,
				Methods: requestMatcher.Methods,
			},
		}
		for _, otherRequestMatcher := range otherRequestMatchers {
			ruleTo.Operation.NotPaths = append(ruleTo.Operation.NotPaths, otherRequestMatcher.Paths...)
		}
		diff = append(diff, ruleTo)
	}
	for _, otherRequestMatcher := range otherRequestMatchers {
		notMethods := otherRequestMatcher.Methods
		if len(notMethods) == 0 {
			notMethods = append(notMethods, v1alpha1.AcceptedHttpMethods...)
		}
		diff = append(diff, &v1beta1.Rule_To{
			Operation: &v1beta1.Operation{
				Paths:      otherRequestMatcher.Paths,
				NotMethods: notMethods,
			},
		})
	}
	return diff
}
