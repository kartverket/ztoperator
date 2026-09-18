package config

import (
	"errors"
	"fmt"

	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/rest"
	"github.com/kelseyhightower/envconfig"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Config struct {
	GitRef                 string                  `split_words:"true" default:"main"`
	AllowedWellKnownURIs   []string                `envconfig:"ALLOWED_WELL_KNOWN_URIS" delimiter:","`
	DiscoveryDocumentCache *DiscoveryDocumentCache `ignored:"true"`
}

// DiscoveryDocumentCache contains the configured well-known URI allowlist and
// the discovery documents loaded for those URIs. Both are immutable after
// construction.
type DiscoveryDocumentCache struct {
	allowedURIs []string
	allowed     map[string]struct{}
	documents   map[string]rest.DiscoveryDocument
}

var cfg Config

func Load() error {
	var loaded Config
	if err := envconfig.Process("ztoperator", &loaded); err != nil {
		return err
	}
	return load(loaded, rest.NewHTTPDiscoveryDocumentResolver())
}

// LoadWithResolver loads configuration and eagerly fetches every configured
// discovery document. The resolver argument exists to keep startup loading
// deterministic in tests; production uses the HTTP resolver through Load.
func LoadWithResolver(discoveryResolver rest.DiscoveryDocumentResolver) error {
	var loaded Config
	if err := envconfig.Process("ztoperator", &loaded); err != nil {
		return err
	}
	return load(loaded, discoveryResolver)
}

func load(loaded Config, discoveryResolver rest.DiscoveryDocumentResolver) error {
	discoveryCache, err := LoadDiscoveryDocumentCache(
		loaded.AllowedWellKnownURIs,
		discoveryResolver,
		log.Logger{Logger: ctrl.Log.WithName("config")},
	)
	if err != nil {
		return fmt.Errorf("failed to load discovery document cache: %w", err)
	}
	loaded.DiscoveryDocumentCache = discoveryCache

	cfg = loaded
	return nil
}

// NewDiscoveryDocumentCache creates a cache from already loaded documents.
// The input slices and documents are copied so callers cannot mutate the
// cache after it has been constructed.
func NewDiscoveryDocumentCache(
	allowedURIs []string,
	documents map[string]rest.DiscoveryDocument,
) *DiscoveryDocumentCache {
	allowed := make(map[string]struct{}, len(allowedURIs))
	configuredURIs := make([]string, 0, len(allowedURIs))
	for _, uri := range allowedURIs {
		if _, exists := allowed[uri]; exists {
			continue
		}
		allowed[uri] = struct{}{}
		configuredURIs = append(configuredURIs, uri)
	}

	cachedDocuments := make(map[string]rest.DiscoveryDocument, len(documents))
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
// document. It fails fast when configuration is empty or any endpoint cannot
// be fetched, so the operator cannot start with a partial cache.
func LoadDiscoveryDocumentCache(
	allowedURIs []string,
	discoveryResolver rest.DiscoveryDocumentResolver,
	rLog log.Logger,
) (*DiscoveryDocumentCache, error) {
	if discoveryResolver == nil {
		return nil, errors.New("discovery document resolver is not configured")
	}
	if len(allowedURIs) == 0 {
		return nil, errors.New("at least one well-known URI must be configured")
	}

	documents := make(map[string]rest.DiscoveryDocument, len(allowedURIs))
	for _, uri := range allowedURIs {
		if err := rest.ValidateWellKnownURI(uri); err != nil {
			return nil, fmt.Errorf("invalid configured well-known URI: %w", err)
		}
		if _, exists := documents[uri]; exists {
			continue
		}

		document, err := discoveryResolver.GetOAuthDiscoveryDocument(uri, rLog)
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
) (*rest.DiscoveryDocument, error) {
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

func cloneDiscoveryDocument(document rest.DiscoveryDocument) rest.DiscoveryDocument {
	return rest.DiscoveryDocument{
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

func Get() Config {
	loaded := cfg
	loaded.AllowedWellKnownURIs = append([]string(nil), cfg.AllowedWellKnownURIs...)
	return loaded
}
