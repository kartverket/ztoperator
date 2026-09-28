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
	require.NotNil(t, loaded.DiscoveryDocumentCache)
	require.Equal(t, []string{configTestURI1, configTestURI2}, loaded.AllowedWellKnownURIs)
	require.Equal(t, []string{configTestURI1, configTestURI2}, resolver.calls)
	_, uri1Allowed := loaded.DiscoveryDocumentCache[configTestURI1]
	require.True(t, uri1Allowed)
	_, uri2Allowed := loaded.DiscoveryDocumentCache[configTestURI2]
	require.True(t, uri2Allowed)

	document, documentCached := loaded.DiscoveryDocumentCache[configTestURI1]
	require.True(t, documentCached)
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
	document, documentCached := loaded.DiscoveryDocumentCache[uri]
	require.True(t, documentCached)
	require.Equal(t, "https://idp.example.com", *document.Issuer)
}

func TestGetReturnsDefensiveDiscoveryDocumentCopy(t *testing.T) {
	t.Setenv("ZTOPERATOR_ALLOWED_WELL_KNOWN_URIS", configTestURI1)

	resolver := &configTestResolver{
		documents: map[string]*rest.DiscoveryDocument{
			configTestURI1: {
				Issuer: helperfunctions.Ptr("https://idp-one.example.com"),
			},
		},
		errors: map[string]error{},
	}

	require.NoError(t, config.LoadWithResolver(resolver))
	loaded := config.Get()
	loadedDocument := loaded.DiscoveryDocumentCache[configTestURI1]
	*loadedDocument.Issuer = "https://mutated.example.com"
	loaded.DiscoveryDocumentCache[configTestURI1] = rest.DiscoveryDocument{}

	fresh := config.Get()
	freshDocument, exists := fresh.DiscoveryDocumentCache[configTestURI1]
	require.True(t, exists)
	require.Equal(t, "https://idp-one.example.com", *freshDocument.Issuer)
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

	err := config.Load()
	require.Error(t, err)
}
