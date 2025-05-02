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
	resolvedAp := scope.ResolvedAuthPolicy

	var jwtRules []*securityv1.JWTRule

	for _, rule := range resolvedAp.ResolvedRules {
		jwtRule := &securityv1.JWTRule{
			Issuer:               rule.IssuerUri,
			Audiences:            rule.Audiences,
			JwksUri:              rule.JwksUri,
			ForwardOriginalToken: true,
		}

		if rule.Rule.FromCookies != nil && len(*rule.Rule.FromCookies) > 0 {
			jwtRule.FromCookies = *rule.Rule.FromCookies
		}
		if rule.Rule.ForwardJwt != nil {
			jwtRule.ForwardOriginalToken = *rule.Rule.ForwardJwt
		}
		if rule.Rule.OutputClaimToHeaders != nil && len(*rule.Rule.OutputClaimToHeaders) > 0 {
			claimsToHeaders := make([]*v1beta1.ClaimToHeader, len(*rule.Rule.OutputClaimToHeaders))
			for i, claimToHeader := range *rule.Rule.OutputClaimToHeaders {
				claimsToHeaders[i] = &v1beta1.ClaimToHeader{
					Header: claimToHeader.Header,
					Claim:  claimToHeader.Claim,
				}
			}
			jwtRule.OutputClaimToHeaders = claimsToHeaders
		}
	}

	return &istioclientsecurityv1.RequestAuthentication{
		ObjectMeta: objectMeta,
		Spec: securityv1.RequestAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{MatchLabels: scope.ResolvedAuthPolicy.AuthPolicy.Spec.Selector.MatchLabels},
			JwtRules: jwtRules,
		},
	}
}
