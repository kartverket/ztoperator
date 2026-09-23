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
	GitRef                 string                            `split_words:"true" default:"main"`
	AllowedWellKnownURIs   []string                          `envconfig:"allowed_well_known_uris"`
	DiscoveryDocumentCache map[string]rest.DiscoveryDocument `ignored:"true"`
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
	if discoveryResolver == nil {
		return errors.New("discovery document resolver is not configured")
	}
	if len(loaded.AllowedWellKnownURIs) == 0 {
		return errors.New("at least one well-known URI must be configured")
	}

	discoveryDocuments := make(map[string]rest.DiscoveryDocument, len(loaded.AllowedWellKnownURIs))
	rLog := log.Logger{Logger: ctrl.Log.WithName("config")}
	for _, uri := range loaded.AllowedWellKnownURIs {
		if _, exists := discoveryDocuments[uri]; exists {
			continue
		}

		document, err := discoveryResolver.GetOAuthDiscoveryDocument(uri, rLog)
		if err != nil {
			return fmt.Errorf("failed to fetch discovery document for well-known URI %q: %w", uri, err)
		}
		if document == nil {
			return fmt.Errorf(
				"failed to fetch discovery document for well-known URI %q: resolver returned an empty document",
				uri,
			)
		}
		discoveryDocuments[uri] = *document
	}

	loaded.DiscoveryDocumentCache = discoveryDocuments

	cfg = loaded
	return nil
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
	loaded.DiscoveryDocumentCache = cloneDiscoveryDocuments(cfg.DiscoveryDocumentCache)
	return loaded
}

func cloneDiscoveryDocuments(documents map[string]rest.DiscoveryDocument) map[string]rest.DiscoveryDocument {
	if documents == nil {
		return nil
	}

	cloned := make(map[string]rest.DiscoveryDocument, len(documents))
	for uri, document := range documents {
		cloned[uri] = cloneDiscoveryDocument(document)
	}
	return cloned
}
