package model_test

import (
	"os"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/model"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToIdentityProviderUrisUsesConfiguredProvider(t *testing.T) {
	configureDiscoveryTestProviders(t)

	result, err := model.ToIdentityProviderUris(testAuthPolicy("https://idp.example.com/.well-known/openid-configuration"))

	require.NoError(t, err)
	require.Equal(t, "https://idp.example.com", result.IssuerURI)
	require.Equal(t, "https://idp.example.com/jwks", result.JwksURI)
	require.Equal(t, "https://idp.example.com/token", result.TokenURI)
	require.Equal(t, "https://idp.example.com/authorize", result.AuthorizationURI)
	require.NotNil(t, result.EndSessionURI)
	require.Equal(t, "https://idp.example.com/endsession", *result.EndSessionURI)
}

func TestToIdentityProviderUrisAllowsProviderWithoutOptionalEndpoints(t *testing.T) {
	configureDiscoveryTestProviders(t)

	result, err := model.ToIdentityProviderUris(
		testAuthPolicy("https://optional.example.com/.well-known/openid-configuration"),
	)

	require.NoError(t, err)
	require.Equal(t, "https://optional.example.com", result.IssuerURI)
	require.Empty(t, result.AuthorizationURI)
	require.Nil(t, result.EndSessionURI)
}

func TestToIdentityProviderUrisRejectsUnknownProvider(t *testing.T) {
	configureDiscoveryTestProviders(t)

	result, err := model.ToIdentityProviderUris(
		testAuthPolicy("https://unknown.example.com/.well-known/openid-configuration"),
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.ErrorContains(t, err, "is not in the configured allowlist")
}

func configureDiscoveryTestProviders(t *testing.T) {
	t.Helper()
	previous, wasSet := os.LookupEnv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS")
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", previous)
		} else {
			_ = os.Unsetenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS")
		}
	})
	providers := `{
		"required": {
			"wellKnownURI": "https://idp.example.com/.well-known/openid-configuration",
			"issuer": "https://idp.example.com",
			"tokenEndpoint": "https://idp.example.com/token",
			"jwksURI": "https://idp.example.com/jwks",
			"authorizationEndpoint": "https://idp.example.com/authorize",
			"endSessionEndpoint": "https://idp.example.com/endsession"
		},
		"optional": {
			"wellKnownURI": "https://optional.example.com/.well-known/openid-configuration",
			"issuer": "https://optional.example.com",
			"tokenEndpoint": "https://optional.example.com/token",
			"jwksURI": "https://optional.example.com/jwks"
		}
	}`
	require.NoError(t, os.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", providers))
	require.NoError(t, config.Load())
}

func testAuthPolicy(wellKnownURI string) ztoperatorv1alpha1.AuthPolicy {
	return ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
		Spec:       ztoperatorv1alpha1.AuthPolicySpec{WellKnownURI: wellKnownURI, Enabled: true},
	}
}
