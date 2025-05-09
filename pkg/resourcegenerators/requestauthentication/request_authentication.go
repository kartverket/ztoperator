package requestauthentication

import (
	"github.com/kartverket/ztoperator/internal/state"
	securityv1 "istio.io/api/security/v1"
	"istio.io/api/security/v1beta1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.RequestAuthentication {
	var jwtRules []*securityv1.JWTRule

	for _, rule := range scope.AuthPolicy.Spec.Rules {
		jwtRule := &securityv1.JWTRule{
			Issuer:               rule.IssuerUri,
			Audiences:            rule.Audience,
			JwksUri:              rule.JwksUri,
			ForwardOriginalToken: rule.ForwardJwt,
		}

		if rule.FromCookies != nil && len(*rule.FromCookies) > 0 {
			jwtRule.FromCookies = *rule.FromCookies
		}
		if rule.OutputClaimToHeaders != nil && len(*rule.OutputClaimToHeaders) > 0 {
			claimsToHeaders := make([]*v1beta1.ClaimToHeader, len(*rule.OutputClaimToHeaders))
			for i, claimToHeader := range *rule.OutputClaimToHeaders {
				claimsToHeaders[i] = &v1beta1.ClaimToHeader{
					Header: claimToHeader.Header,
					Claim:  claimToHeader.Claim,
				}
			}
			jwtRule.OutputClaimToHeaders = claimsToHeaders
		}
		jwtRules = append(jwtRules, jwtRule)
	}

	return &istioclientsecurityv1.RequestAuthentication{
		ObjectMeta: objectMeta,
		Spec: securityv1.RequestAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{MatchLabels: scope.AuthPolicy.Spec.Selector.MatchLabels},
			JwtRules: jwtRules,
		},
	}
}
