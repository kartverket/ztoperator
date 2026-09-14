package config_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/rest"
	"github.com/stretchr/testify/require"
)

const (
	configTestURI1 = "https://idp-one.example.com/.well-known/openid-configuration"
	configTestURI2 = "https://idp-two.example.com/.well-known/openid-configuration"
)

type configTestResolver struct {
	documents map[string]*rest.DiscoveryDocument
	errors    map[string]error
	calls     []string
}

func (r *configTestResolver) GetOAuthDiscoveryDocument(
	uri string,
	_ log.Logger,
) (*rest.DiscoveryDocument, error) {
	r.calls = append(r.calls, uri)
	if err := r.errors[uri]; err != nil {
		return nil, err
	}
	return r.documents[uri], nil
}

func TestLoadWithResolverCachesAllConfiguredDiscoveryDocuments(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_WELL_KNOWN_URIS", configTestURI1+","+configTestURI2)
	t.Setenv("ZTOPERATOR_GIT_REF", "test")

	resolver := &configTestResolver{
		documents: map[string]*rest.DiscoveryDocument{
			configTestURI1: {
				Issuer: helperfunctions.Ptr("https://idp-one.example.com"),
			},
			configTestURI2: {
				Issuer: helperfunctions.Ptr("https://idp-two.example.com"),
			},
		},
		errors: map[string]error{},
	}

	require.NoError(t, config.LoadWithResolver(resolver))
	loaded := config.Get()
	require.Equal(t, []string{configTestURI1, configTestURI2}, loaded.AllowedWellKnownURIs)
	require.Equal(t, []string{configTestURI1, configTestURI2}, resolver.calls)
	require.True(t, loaded.DiscoveryDocumentCache.IsAllowed(configTestURI1))
	require.True(t, loaded.DiscoveryDocumentCache.IsAllowed(configTestURI2))

	document, err := loaded.DiscoveryDocumentCache.GetOAuthDiscoveryDocument(configTestURI1, log.Logger{})
	require.NoError(t, err)
	require.Equal(t, "https://idp-one.example.com", *document.Issuer)
}

func TestLoadFetchesAndCachesDiscoveryDocumentFromConfiguredHTTPEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		discoveryDocument := []byte(`{
  "issuer": "https://idp.example.com",
  "jwks_uri": "https://idp.example.com/jwks",
  "token_endpoint": "https://idp.example.com/token"
}`)
		_, _ = w.Write(discoveryDocument)
	}))
	defer server.Close()

	uri := server.URL + "/.well-known/openid-configuration"
	t.Setenv("ZTOPERATOR_ALLOWED_WELL_KNOWN_URIS", uri)

	require.NoError(t, config.Load())
	loaded := config.Get()
	document, err := loaded.DiscoveryDocumentCache.GetOAuthDiscoveryDocument(uri, log.Logger{})
	require.NoError(t, err)
	require.Equal(t, "https://idp.example.com", *document.Issuer)
}

func TestLoadFailsWhenConfiguredHTTPEndpointIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	uri := server.URL + "/.well-known/openid-configuration"
	server.Close()
	t.Setenv("ZTOPERATOR_ALLOWED_WELL_KNOWN_URIS", uri)

	err := config.Load()

	require.Error(t, err)
	require.ErrorContains(t, err, uri)
}

func TestLoadWithResolverFailsWhenConfiguredEndpointCannotBeFetched(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_WELL_KNOWN_URIS", configTestURI1+","+configTestURI2)

	resolver := &configTestResolver{
		documents: map[string]*rest.DiscoveryDocument{
			configTestURI1: {
				Issuer: helperfunctions.Ptr("https://idp-one.example.com"),
			},
		},
		errors: map[string]error{
			configTestURI2: errors.New("endpoint unavailable"),
		},
	}

	err := config.LoadWithResolver(resolver)
	require.Error(t, err)
	require.ErrorContains(t, err, configTestURI2)
	require.ErrorContains(t, err, "endpoint unavailable")
	require.Equal(t, []string{configTestURI1, configTestURI2}, resolver.calls)
}

func TestLoadRequiresConfiguredWellKnownEndpoints(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_WELL_KNOWN_URIS", "")

	err := config.LoadWithResolver(&configTestResolver{})
	require.Error(t, err)
}
