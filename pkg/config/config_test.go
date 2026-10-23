package config_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/stretchr/testify/require"
)

const (
	configTestURI1 = "http://mock-oauth2.auth:8080/entraid/.well-known/openid-configuration"
	configTestURI2 = "https://ansattporten.no/.well-known/openid-configuration"
)

func TestLoadAcceptsConfiguredOAuthProviders(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{
		"entraid": {"wellKnownURI": "`+configTestURI1+`", "requireAudience": true},
		"ansattporten": {"wellKnownURI": "`+configTestURI2+`", "requireAudience": false}
	}`)
	t.Setenv("ZTOPERATOR_GIT_REF", "test")

	require.NoError(t, config.Load())
	loaded := config.Get()
	require.Equal(t, config.OAuthProvider{
		WellKnownURI:    configTestURI1,
		RequireAudience: true,
	}, loaded.AllowedOAuthProviders["entraid"])
	require.Equal(t, config.OAuthProvider{
		WellKnownURI:    configTestURI2,
		RequireAudience: false,
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
		"first": {"wellKnownURI": "`+configTestURI1+`", "requireAudience": true},
		"second": {"wellKnownURI": "`+configTestURI1+`", "requireAudience": false}
	}`)

	err := config.Load()

	require.ErrorContains(t, err, "is configured for both OAuth provider")
}
