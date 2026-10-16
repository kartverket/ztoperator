package config

import (
	"errors"
	"fmt"
	"net/url"
	"slices"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	GitRef                        string   `split_words:"true" default:"main"`
	AllowedWellKnownURIs          []string `envconfig:"allowed_well_known_uris"`
	AudienceRequiredWellKnownURIs []string `envconfig:"audience_required_well_known_uris"`
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
	if err := validateWellKnownURIs(loaded.AllowedWellKnownURIs); err != nil {
		return err
	}
	if err := validateWellKnownURIs(loaded.AudienceRequiredWellKnownURIs); err != nil {
		return err
	}
	for _, uri := range loaded.AudienceRequiredWellKnownURIs {
		if !slices.Contains(loaded.AllowedWellKnownURIs, uri) {
			return fmt.Errorf("audience-required well-known URI %q is not in the configured allowlist", uri)
		}
	}
	return nil
}

func validateWellKnownURIs(uris []string) error {
	for _, uri := range uris {
		parsedURI, err := url.Parse(uri)
		if err != nil || parsedURI.Host == "" || (parsedURI.Scheme != "http" && parsedURI.Scheme != "https") {
			return fmt.Errorf("well-known URI %q must be a valid http or https URL", uri)
		}
	}
	return nil
}

func Get() Config {
	loaded := cfg
	loaded.AllowedWellKnownURIs = append([]string(nil), cfg.AllowedWellKnownURIs...)
	loaded.AudienceRequiredWellKnownURIs = append([]string(nil), cfg.AudienceRequiredWellKnownURIs...)
	return loaded
}
