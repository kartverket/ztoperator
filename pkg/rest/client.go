package rest

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"resty.dev/v3"

	"github.com/kartverket/ztoperator/pkg/log"
)

type DiscoveryDocumentResolver interface {
	GetOAuthDiscoveryDocument(uri string, rLog log.Logger) (*DiscoveryDocument, error)
}

// DiscoveryDocumentCache contains the configured well-known URI allowlist and
// the discovery documents loaded for those URIs. Both are immutable after
// construction.
type DiscoveryDocumentCache struct {
	allowedURIs []string
	allowed     map[string]struct{}
	documents   map[string]DiscoveryDocument
}

// NewDiscoveryDocumentCache creates a cache from already loaded documents.
// The input slices and documents are copied so callers cannot mutate the
// cache after it has been constructed.
func NewDiscoveryDocumentCache(allowedURIs []string, documents map[string]DiscoveryDocument) *DiscoveryDocumentCache {
	allowed := make(map[string]struct{}, len(allowedURIs))
	configuredURIs := make([]string, 0, len(allowedURIs))
	for _, uri := range allowedURIs {
		if _, exists := allowed[uri]; exists {
			continue
		}
		allowed[uri] = struct{}{}
		configuredURIs = append(configuredURIs, uri)
	}

	cachedDocuments := make(map[string]DiscoveryDocument, len(documents))
	for uri, document := range documents {
		cachedDocuments[uri] = cloneDiscoveryDocument(document)
	}

	return &DiscoveryDocumentCache{
		allowedURIs: configuredURIs,
		allowed:     allowed,
		documents:   cachedDocuments,
	}
}

// LoadDiscoveryDocumentCache fetches and caches every configured discovery
// document. It fails fast when any endpoint cannot be fetched or returns an
// empty document, so the operator cannot start with a partial cache.
func LoadDiscoveryDocumentCache(
	allowedURIs []string,
	resolver DiscoveryDocumentResolver,
	rLog log.Logger,
) (*DiscoveryDocumentCache, error) {
	if resolver == nil {
		return nil, errors.New("discovery document resolver is not configured")
	}
	if len(allowedURIs) == 0 {
		return nil, errors.New("at least one well-known URI must be configured")
	}

	documents := make(map[string]DiscoveryDocument, len(allowedURIs))
	for _, uri := range allowedURIs {
		if err := ValidateWellKnownURI(uri); err != nil {
			return nil, fmt.Errorf("invalid configured well-known URI: %w", err)
		}
		if _, exists := documents[uri]; exists {
			continue
		}

		document, err := resolver.GetOAuthDiscoveryDocument(uri, rLog)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch discovery document for well-known URI %q: %w", uri, err)
		}
		if document == nil {
			return nil, fmt.Errorf(
				"failed to fetch discovery document for well-known URI %q: resolver returned an empty document",
				uri,
			)
		}
		documents[uri] = *document
	}

	return NewDiscoveryDocumentCache(allowedURIs, documents), nil
}

// ValidateWellKnownURI verifies that uri is a well-formed http or https URL
// suitable for use as an OpenID Connect / OAuth discovery endpoint.
func ValidateWellKnownURI(uri string) error {
	if uri == "" {
		return errors.New("wellKnownURI must not be empty")
	}

	parsed, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("wellKnownURI %q is not a valid URL: %w", uri, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("wellKnownURI %q must use http or https scheme", uri)
	}
	if parsed.Host == "" {
		return fmt.Errorf("wellKnownURI %q must include a host", uri)
	}
	if parsed.RawQuery != "" {
		return fmt.Errorf("wellKnownURI %q must not contain a query string", uri)
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("wellKnownURI %q must not contain a fragment", uri)
	}

	return nil
}

// GetAllowedWellKnownURIs returns a copy of the configured allowlist.
func (c *DiscoveryDocumentCache) GetAllowedWellKnownURIs() []string {
	if c == nil {
		return nil
	}
	return append([]string(nil), c.allowedURIs...)
}

// IsAllowed reports whether uri is an exactly configured well-known endpoint.
func (c *DiscoveryDocumentCache) IsAllowed(uri string) bool {
	if c == nil {
		return false
	}
	_, exists := c.allowed[uri]
	return exists
}

// GetOAuthDiscoveryDocument only returns documents already present in the
// cache. It deliberately never performs an HTTP request.
func (c *DiscoveryDocumentCache) GetOAuthDiscoveryDocument(
	uri string,
	_ log.Logger,
) (*DiscoveryDocument, error) {
	if c == nil {
		return nil, errors.New("discovery document cache is not configured")
	}
	if !c.IsAllowed(uri) {
		return nil, fmt.Errorf("well-known URI %q is not in the configured allowlist", uri)
	}

	document, exists := c.documents[uri]
	if !exists {
		return nil, fmt.Errorf("discovery document for well-known URI %q is not cached", uri)
	}
	returnDocument := cloneDiscoveryDocument(document)
	return &returnDocument, nil
}

// HTTPDiscoveryDocumentResolver is used only while loading the startup cache.
type HTTPDiscoveryDocumentResolver struct{}

// DefaultDiscoveryDocumentResolver is kept for callers using the repository's
// preconfigured discovery documents. It is cache-only; unknown URIs are not
// fetched over HTTP.
type DefaultDiscoveryDocumentResolver struct {
	cache *DiscoveryDocumentCache
}

// NewHTTPDiscoveryDocumentResolver creates the resolver used for startup
// discovery-document fetching.
func NewHTTPDiscoveryDocumentResolver() *HTTPDiscoveryDocumentResolver {
	return &HTTPDiscoveryDocumentResolver{}
}

func NewDefaultDiscoveryDocumentResolver() *DefaultDiscoveryDocumentResolver {
	documents := GetWellknownURIToDiscoveryDocument()
	allowedURIs := make([]string, 0, len(documents))
	for uri := range documents {
		allowedURIs = append(allowedURIs, uri)
	}
	return &DefaultDiscoveryDocumentResolver{
		cache: NewDiscoveryDocumentCache(allowedURIs, documents),
	}
}

func (r *DefaultDiscoveryDocumentResolver) GetOAuthDiscoveryDocument(
	uri string,
	rLog log.Logger,
) (*DiscoveryDocument, error) {
	if r == nil {
		return nil, errors.New("default discovery document resolver is not configured")
	}
	return r.cache.GetOAuthDiscoveryDocument(uri, rLog)
}

func (r *HTTPDiscoveryDocumentResolver) GetOAuthDiscoveryDocument(
	uri string,
	rLog log.Logger,
) (*DiscoveryDocument, error) {
	if err := ValidateWellKnownURI(uri); err != nil {
		return nil, err
	}

	var discoveryDocument DiscoveryDocument
	rLog.Info(fmt.Sprintf("Fetching discovery document for well-known uri: %s", uri))
	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) {
		closeErr := client.Close()
		if closeErr != nil {
			panic(closeErr)
		}
	}(client)

	res, err := client.R().SetResult(&discoveryDocument).Get(uri)
	if err != nil {
		return nil, err
	}
	if res.StatusCode() != 200 {
		return nil, errors.New(res.Status())
	}
	return &discoveryDocument, nil
}

func cloneDiscoveryDocument(document DiscoveryDocument) DiscoveryDocument {
	return DiscoveryDocument{
		Issuer:                cloneString(document.Issuer),
		AuthorizationEndpoint: cloneString(document.AuthorizationEndpoint),
		TokenEndpoint:         cloneString(document.TokenEndpoint),
		JwksURI:               cloneString(document.JwksURI),
		EndSessionEndpoint:    cloneString(document.EndSessionEndpoint),
	}
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
