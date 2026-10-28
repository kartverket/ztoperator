package config_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/stretchr/testify/require"
)

const (
	configTestURI1   = "http://mock-oauth2.auth:8080/entraid/.well-known/openid-configuration"
	configTestURI2   = "https://ansattporten.no/.well-known/openid-configuration"
	configTestIssuer = "https://issuer.example.com"
)

func TestLoadAcceptsConfiguredOAuthProviders(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{
		"entraid": {
			"wellKnownURI": "`+configTestURI1+`",
			"requireAudience": true,
			"issuer": "`+configTestIssuer+`",
			"authorizationEndpoint": "https://issuer.example.com/authorize",
			"tokenEndpoint": "https://issuer.example.com/token",
			"jwksURI": "https://issuer.example.com/jwks",
			"endSessionEndpoint": "https://issuer.example.com/logout"
		},
		"ansattporten": {
			"wellKnownURI": "`+configTestURI2+`",
			"requireAudience": false,
			"issuer": "https://ansattporten.no",
			"tokenEndpoint": "https://ansattporten.no/token",
			"jwksURI": "https://ansattporten.no/jwks"
		}
	}`)
	t.Setenv("ZTOPERATOR_GIT_REF", "test")

	require.NoError(t, config.Load())
	loaded := config.Get()
	require.Equal(t, config.OAuthProvider{
		WellKnownURI:          configTestURI1,
		RequireAudience:       true,
		Issuer:                configTestIssuer,
		AuthorizationEndpoint: "https://issuer.example.com/authorize",
		TokenEndpoint:         "https://issuer.example.com/token",
		JwksURI:               "https://issuer.example.com/jwks",
		EndSessionEndpoint:    "https://issuer.example.com/logout",
	}, loaded.AllowedOAuthProviders["entraid"])
	require.Equal(t, config.OAuthProvider{
		WellKnownURI:  configTestURI2,
		Issuer:        "https://ansattporten.no",
		TokenEndpoint: "https://ansattporten.no/token",
		JwksURI:       "https://ansattporten.no/jwks",
	}, loaded.AllowedOAuthProviders["ansattporten"])
	require.Equal(t, "test", loaded.GitRef)
}

func TestLoadRequiresConfiguredOAuthProviders(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{}`)

	err := config.Load()

	require.ErrorContains(t, err, "at least one OAuth provider must be configured")
}

func TestLoadRejectsInvalidProviderJSON(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{invalid}`)

	err := config.Load()

	require.ErrorContains(t, err, "invalid character")
}

func TestLoadRejectsInvalidWellKnownURI(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{
		"invalid": {"wellKnownURI": "not-a-uri", "requireAudience": true}
	}`)

	err := config.Load()

	require.ErrorContains(t, err, "must be a valid http or https URL")
}

func TestLoadRejectsEmptyProviderName(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{
		"": {"wellKnownURI": "`+configTestURI1+`", "requireAudience": true}
	}`)

	err := config.Load()

	require.ErrorContains(t, err, "OAuth provider name must not be empty")
}

func TestLoadRejectsDuplicateWellKnownURI(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{
		"first": {
			"wellKnownURI": "`+configTestURI1+`",
			"requireAudience": true,
			"issuer": "`+configTestIssuer+`",
			"tokenEndpoint": "https://issuer.example.com/token",
			"jwksURI": "https://issuer.example.com/jwks"
		},
		"second": {
			"wellKnownURI": "`+configTestURI1+`",
			"requireAudience": false,
			"issuer": "`+configTestIssuer+`",
			"tokenEndpoint": "https://issuer.example.com/token",
			"jwksURI": "https://issuer.example.com/jwks"
		}
	}`)

	err := config.Load()

	require.ErrorContains(t, err, "is configured for both OAuth provider")
}

func TestLoadRejectsMissingRequiredProviderEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		providerJSON string
		missingField string
	}{
		{
			name: "issuer",
			providerJSON: `"tokenEndpoint": "https://issuer.example.com/token",
				"jwksURI": "https://issuer.example.com/jwks"`,
			missingField: "issuer",
		},
		{
			name: "token endpoint",
			providerJSON: `"issuer": "` + configTestIssuer + `",
				"jwksURI": "https://issuer.example.com/jwks"`,
			missingField: "tokenEndpoint",
		},
		{
			name: "JWKS URI",
			providerJSON: `"issuer": "` + configTestIssuer + `",
				"tokenEndpoint": "https://issuer.example.com/token"`,
			missingField: "jwksURI",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{
				"missing-endpoint": {
					"wellKnownURI": "`+configTestURI1+`",
					`+test.providerJSON+`
				}
			}`)

			err := config.Load()

			require.ErrorContains(t, err, test.missingField)
		})
	}
}

func TestLoadRejectsInvalidOptionalProviderEndpoint(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{
		"invalid-authorization": {
			"wellKnownURI": "`+configTestURI1+`",
			"issuer": "`+configTestIssuer+`",
			"authorizationEndpoint": "not-a-uri",
			"tokenEndpoint": "https://issuer.example.com/token",
			"jwksURI": "https://issuer.example.com/jwks"
		}
	}`)

	err := config.Load()

	require.ErrorContains(t, err, "authorizationEndpoint")
}

func TestLoadRejectsUnsafeProviderEndpoint(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{
		"unsafe": {
			"wellKnownURI": "`+configTestURI1+`",
			"issuer": "`+configTestIssuer+`",
			"tokenEndpoint": "https://issuer.example.com/token",
			"jwksURI": "https://issuer.example.com/jwks\\bad"
		}
	}`)

	err := config.Load()

	require.ErrorContains(t, err, "jwksURI")
}
