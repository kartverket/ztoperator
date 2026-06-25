package requestauthentication_test

import (
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/requestauthentication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetDesired_ReturnsNil_WhenDisabled(t *testing.T) {
	scope := defaultScope()
	scope.AuthPolicy.Spec.Enabled = false
	assert.Nil(t, requestauthentication.GetDesired(&scope, defaultObjectMeta()))
}

func TestGetDesired_ReturnsNil_WhenInvalidConfig(t *testing.T) {
	scope := defaultScope()
	scope.InvalidConfig = true
	assert.Nil(t, requestauthentication.GetDesired(&scope, defaultObjectMeta()))
}

func TestGetDesired_ObjectMetaIsPreserved(t *testing.T) {
	scope := defaultScope()
	name := "my-request-auth"
	namespace := "my-namespace"
	labels := map[string]string{"team": "security"}
	om := metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}

	ra := requestauthentication.GetDesired(&scope, om)

	require.NotNil(t, ra)
	assert.Equal(t, name, ra.Name)
	assert.Equal(t, namespace, ra.Namespace)
	assert.Equal(t, labels, ra.Labels)
}

func TestGetDesired_WorkloadSelectorMatchesAuthPolicySelector(t *testing.T) {
	scope := defaultScope()

	ra := requestauthentication.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ra)
	assert.Equal(t, scope.AuthPolicy.Spec.Selector.MatchLabels, ra.Spec.Selector.MatchLabels)
}

func TestGetDesired_JWTRuleHasCorrectIssuerAndJWKSURI(t *testing.T) {
	scope := defaultScope()

	ra := requestauthentication.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ra)
	require.Len(t, ra.Spec.JwtRules, 1)
	assert.Equal(t, scope.IdentityProviderUris.IssuerURI, ra.Spec.JwtRules[0].Issuer)
	assert.Equal(t, scope.IdentityProviderUris.JwksURI, ra.Spec.JwtRules[0].JwksUri)
}

func TestGetDesired_AudiencesAreIncluded_WhenPopulated(t *testing.T) {
	scope := defaultScope()
	scope.Audiences = []string{"api://my-api", "https://example.com"}

	ra := requestauthentication.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ra)
	require.Len(t, ra.Spec.JwtRules, 1)
	assert.Equal(t, []string{"api://my-api", "https://example.com"}, ra.Spec.JwtRules[0].Audiences)
}

func TestGetDesired_AudiencesAreEmpty_WhenNotPopulated(t *testing.T) {
	scope := defaultScope()
	scope.Audiences = []string{}

	ra := requestauthentication.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ra)
	require.Len(t, ra.Spec.JwtRules, 1)
	assert.Empty(t, ra.Spec.JwtRules[0].Audiences)
}

func TestGetDesired_ForwardJWTDefaultsToTrue(t *testing.T) {
	scope := defaultScope()
	scope.AuthPolicy.Spec.ForwardJwt = nil

	ra := requestauthentication.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ra)
	require.Len(t, ra.Spec.JwtRules, 1)
	assert.True(t, ra.Spec.JwtRules[0].ForwardOriginalToken)
}

func TestGetDesired_ForwardJWTRespectsFalseValue(t *testing.T) {
	scope := defaultScope()
	falseValue := false
	scope.AuthPolicy.Spec.ForwardJwt = &falseValue

	ra := requestauthentication.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ra)
	require.Len(t, ra.Spec.JwtRules, 1)
	assert.False(t, ra.Spec.JwtRules[0].ForwardOriginalToken)
}

func TestGetDesired_ForwardJWTRespectsTrueValue(t *testing.T) {
	scope := defaultScope()
	trueValue := true
	scope.AuthPolicy.Spec.ForwardJwt = &trueValue

	ra := requestauthentication.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ra)
	require.Len(t, ra.Spec.JwtRules, 1)
	assert.True(t, ra.Spec.JwtRules[0].ForwardOriginalToken)
}

func TestGetDesired_OutputClaimToHeadersAreIncluded_WhenPopulated(t *testing.T) {
	scope := defaultScope()
	scope.AuthPolicy.Spec.OutputClaimToHeaders = &[]ztoperatorv1alpha1.ClaimToHeader{
		{Claim: "sub", Header: "X-User-ID"},
		{Claim: "email", Header: "X-User-Email"},
	}

	ra := requestauthentication.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ra)
	require.Len(t, ra.Spec.JwtRules, 1)
	require.Len(t, ra.Spec.JwtRules[0].OutputClaimToHeaders, 2)

	assert.Equal(t, "sub", ra.Spec.JwtRules[0].OutputClaimToHeaders[0].Claim)
	assert.Equal(t, "X-User-ID", ra.Spec.JwtRules[0].OutputClaimToHeaders[0].Header)

	assert.Equal(t, "email", ra.Spec.JwtRules[0].OutputClaimToHeaders[1].Claim)
	assert.Equal(t, "X-User-Email", ra.Spec.JwtRules[0].OutputClaimToHeaders[1].Header)
}

func TestGetDesired_OutputClaimToHeadersAreEmpty_WhenNil(t *testing.T) {
	scope := defaultScope()
	scope.AuthPolicy.Spec.OutputClaimToHeaders = nil

	ra := requestauthentication.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ra)
	require.Len(t, ra.Spec.JwtRules, 1)
	assert.Nil(t, ra.Spec.JwtRules[0].OutputClaimToHeaders)
}

func TestGetDesired_OutputClaimToHeadersAreEmpty_WhenEmptySlice(t *testing.T) {
	scope := defaultScope()
	emptySlice := []ztoperatorv1alpha1.ClaimToHeader{}
	scope.AuthPolicy.Spec.OutputClaimToHeaders = &emptySlice

	ra := requestauthentication.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ra)
	require.Len(t, ra.Spec.JwtRules, 1)
	assert.Nil(t, ra.Spec.JwtRules[0].OutputClaimToHeaders)
}

func defaultScope() state.Scope {
	return state.Scope{
		AuthPolicy: ztoperatorv1alpha1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "my-policy", Namespace: "default"},
			Spec: ztoperatorv1alpha1.AuthPolicySpec{
				Enabled: true,
				Selector: ztoperatorv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{"app": "my-app"},
				},
			},
		},
		Audiences: []string{},
		IdentityProviderUris: state.IdentityProviderUris{
			IssuerURI: "https://login.example.com",
			JwksURI:   "https://login.example.com/.well-known/jwks.json",
		},
		InvalidConfig: false,
	}
}

func defaultObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: "my-policy", Namespace: "default"}
}
