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
	if !scope.AuthPolicy.Spec.Enabled || scope.InvalidConfig {
		return nil
	}

	var audiences []string
	if scope.AuthPolicy.Spec.Audience != nil {
		audiences = scope.AuthPolicy.Spec.Audience
	}
	if scope.OAuthCredentials.ClientID != nil {
		for _, audience := range audiences {
			if *scope.OAuthCredentials.ClientID != audience {
				audiences = append(audiences, *scope.OAuthCredentials.ClientID)
			}
		}
	}

	jwtRule := &securityv1.JWTRule{
		Issuer:               scope.IdentityProviderUris.IssuerURI,
		Audiences:            audiences,
		JwksUri:              scope.IdentityProviderUris.JwksURI,
		ForwardOriginalToken: scope.AuthPolicy.Spec.ForwardJwt,
	}

	if scope.AuthPolicy.Spec.OutputClaimToHeaders != nil && len(*scope.AuthPolicy.Spec.OutputClaimToHeaders) > 0 {
		claimsToHeaders := make([]*v1beta1.ClaimToHeader, len(*scope.AuthPolicy.Spec.OutputClaimToHeaders))
		for i, claimToHeader := range *scope.AuthPolicy.Spec.OutputClaimToHeaders {
			claimsToHeaders[i] = &v1beta1.ClaimToHeader{
				Header: claimToHeader.Header,
				Claim:  claimToHeader.Claim,
			}
		}
		jwtRule.OutputClaimToHeaders = claimsToHeaders
	}

	return &istioclientsecurityv1.RequestAuthentication{
		ObjectMeta: objectMeta,
		Spec: securityv1.RequestAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{MatchLabels: scope.AuthPolicy.Spec.Selector.MatchLabels},
			JwtRules: []*securityv1.JWTRule{jwtRule},
		},
	}
}
