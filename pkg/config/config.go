package config

import (
	"fmt"

	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/rest"
	"github.com/kelseyhightower/envconfig"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Config struct {
	GitRef                 string                       `split_words:"true" default:"main"`
	AllowedWellKnownURIs   []string                     `envconfig:"ALLOWED_WELL_KNOWN_URIS" delimiter:","`
	DiscoveryDocumentCache *rest.DiscoveryDocumentCache `ignored:"true"`
}

var cfg Config

func Load() error {
	var loaded Config
	if err := envconfig.Process("ztoperator", &loaded); err != nil {
		return err
	}
	if len(loaded.AllowedWellKnownURIs) == 0 {
		cfg = loaded
		return nil
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
	discoveryCache, err := rest.LoadDiscoveryDocumentCache(
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

func Get() Config {
	loaded := cfg
	loaded.AllowedWellKnownURIs = append([]string(nil), cfg.AllowedWellKnownURIs...)
	return loaded
}
