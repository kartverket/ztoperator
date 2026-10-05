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

func TestLoadAcceptsConfiguredWellKnownURIs(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_WELL_KNOWN_URIS", configTestURI1+","+configTestURI2)
	t.Setenv("ZTOPERATOR_GIT_REF", "test")

	require.NoError(t, config.Load())
	loaded := config.Get()
	require.Equal(t, []string{configTestURI1, configTestURI2}, loaded.AllowedWellKnownURIs)
	require.Equal(t, "test", loaded.GitRef)
}

func TestGetReturnsDefensiveAllowlistCopy(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_WELL_KNOWN_URIS", configTestURI1)

	require.NoError(t, config.Load())
	loaded := config.Get()
	loaded.AllowedWellKnownURIs[0] = "https://mutated.example.com/.well-known/openid-configuration"

	fresh := config.Get()
	require.Equal(t, []string{configTestURI1}, fresh.AllowedWellKnownURIs)
}

func TestLoadRequiresConfiguredWellKnownEndpoints(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_WELL_KNOWN_URIS", "")

	err := config.Load()
	require.Error(t, err)
}

func TestLoadRejectsInvalidWellKnownURIConfiguration(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_WELL_KNOWN_URIS", "not-a-uri")

	err := config.Load()

	require.ErrorContains(t, err, "must be a valid http or https URL")
}
