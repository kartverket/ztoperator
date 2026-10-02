package config

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/kartverket/ztoperator/pkg/rest"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	GitRef               string   `split_words:"true" default:"main"`
	AllowedWellKnownURIs []string `envconfig:"allowed_well_known_uris"`
}

var cfg Config

func Load() error {
	var loaded Config
	if err := envconfig.Process("ztoperator", &loaded); err != nil {
		return err
	}
	return loadConfig(loaded)
}

func loadConfig(loaded Config) error {
	if err := validateConfig(loaded); err != nil {
		return err
	}

	cfg = loaded
	return nil
}

func validateConfig(loaded Config) error {
	if len(loaded.AllowedWellKnownURIs) == 0 {
		return errors.New("at least one well-known URI must be configured")
	}
	staticDocuments := rest.GetWellknownURIToDiscoveryDocument()
	for _, uri := range loaded.AllowedWellKnownURIs {
		parsedURI, err := url.Parse(uri)
		if err != nil || parsedURI.Host == "" || (parsedURI.Scheme != "http" && parsedURI.Scheme != "https") {
			return fmt.Errorf("well-known URI %q must be a valid http or https URL", uri)
		}
		if _, exists := staticDocuments[uri]; !exists {
			return fmt.Errorf(
				"well-known URI %q is not present in the static discovery-document list",
				uri,
			)
		}
	}
	return nil
}

func Get() Config {
	loaded := cfg
	loaded.AllowedWellKnownURIs = append([]string(nil), cfg.AllowedWellKnownURIs...)
	return loaded
}
