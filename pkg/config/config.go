package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/kelseyhightower/envconfig"
)

// OAuthProvider configures an allowed provider and its audience requirement.
type OAuthProvider struct {
	WellKnownURI    string `json:"wellKnownURI"`
	RequireAudience bool   `json:"requireAudience"`
}

// AllowedOAuthProviders maps provider names to their configuration.
type AllowedOAuthProviders map[string]OAuthProvider

// Decode parses the JSON representation used by envconfig.
func (providers *AllowedOAuthProviders) Decode(value string) error {
	return json.Unmarshal([]byte(value), providers)
}

type Config struct {
	GitRef                string                `split_words:"true" default:"main"`
	AllowedOAuthProviders AllowedOAuthProviders `envconfig:"allowed_oauth_providers" required:"true"`
}

var cfg Config

func Load() error {
	var loaded Config
	if err := envconfig.Process("ztoperator", &loaded); err != nil {
		return err
	}
	if len(loaded.AllowedOAuthProviders) == 0 {
		return errors.New("at least one OAuth provider must be configured")
	}
	configuredURIs := make(map[string]string, len(loaded.AllowedOAuthProviders))
	for name, provider := range loaded.AllowedOAuthProviders {
		if name == "" {
			return errors.New("OAuth provider name must not be empty")
		}
		parsedURI, err := url.Parse(provider.WellKnownURI)
		validURI := err == nil && parsedURI.Host != "" &&
			(parsedURI.Scheme == "http" || parsedURI.Scheme == "https")
		if !validURI {
			return fmt.Errorf(
				"well-known URI %q for OAuth provider %q must be a valid http or https URL",
				provider.WellKnownURI,
				name,
			)
		}
		if configuredProvider, found := configuredURIs[provider.WellKnownURI]; found {
			return fmt.Errorf(
				"well-known URI %q is configured for both OAuth provider %q and %q",
				provider.WellKnownURI,
				configuredProvider,
				name,
			)
		}
		configuredURIs[provider.WellKnownURI] = name
	}

	cfg = loaded
	return nil
}

func Get() Config {
	return cfg
}
