package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

// OAuthProvider configures an allowed provider and its discovery document values.
type OAuthProvider struct {
	WellKnownURI                   string `json:"wellKnownURI"`
	RequireAudience                bool   `json:"requireAudience"`
	Issuer                         string `json:"issuer"`
	AuthorizationEndpoint          string `json:"authorizationEndpoint"`
	TokenEndpoint                  string `json:"tokenEndpoint"`
	JwksURI                        string `json:"jwksURI"`
	EndSessionEndpoint             string `json:"endSessionEndpoint"`
	PrivateKeyJwtAssertionAudience string `json:"privateKeyJwtAssertionAudience"`
}

// AllowedOAuthProviders maps provider names to their configuration.
type AllowedOAuthProviders map[string]OAuthProvider

// Decode parses the JSON representation used by envconfig.
func (providers *AllowedOAuthProviders) Decode(value string) error {
	return json.Unmarshal([]byte(value), providers)
}

type Config struct {
	GitRef string `split_words:"true" default:"main"`

	FeatureIstioAmbient bool `split_words:"true" default:"false"`

	EnvoyPort      int32 `split_words:"true" default:"18080"`
	EnvoyProbePort int32 `split_words:"true" default:"18081"`
	EnvoyAdminPort int32 `split_words:"true" default:"18082"`

	AllowedOAuthProviders AllowedOAuthProviders `envconfig:"allowed_oauth_providers" required:"true"`
}

var cfg Config

func Load() error {
	var loaded Config
	if err := envconfig.Process("ztoperator", &loaded); err != nil {
		return err
	}
	if err := validateProviders(loaded.AllowedOAuthProviders); err != nil {
		return err
	}

	cfg = loaded
	return nil
}

func validateProviders(providers AllowedOAuthProviders) error {
	if len(providers) == 0 {
		return errors.New("at least one OAuth provider must be configured")
	}
	configuredURIs := make(map[string]string, len(providers))
	for name, provider := range providers {
		if name == "" {
			return errors.New("OAuth provider name must not be empty")
		}
		if provider.Issuer == "" {
			return errors.New("OAuth provider issuer must not be empty")
		}
		for _, endpoint := range []struct {
			field    string
			uri      string
			required bool
		}{
			{field: "wellKnownURI", uri: provider.WellKnownURI, required: true},
			{field: "tokenEndpoint", uri: provider.TokenEndpoint, required: true},
			{field: "jwksURI", uri: provider.JwksURI, required: true},
			{field: "authorizationEndpoint", uri: provider.AuthorizationEndpoint},
			{field: "endSessionEndpoint", uri: provider.EndSessionEndpoint},
		} {
			if err := validateProviderURI(name, endpoint.field, endpoint.uri, endpoint.required); err != nil {
				return err
			}
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
	return nil
}

func Get() Config {
	return cfg
}

func GetOAuthProvider(wellKnownURI string) (OAuthProvider, bool) {
	for _, provider := range Get().AllowedOAuthProviders {
		if provider.WellKnownURI == wellKnownURI {
			return provider, true
		}
	}
	return OAuthProvider{}, false
}

func validateProviderURI(providerName, field, uri string, required bool) error {
	if uri == "" && !required {
		return nil
	}
	parsedURI, err := url.Parse(uri)
	validURI := err == nil && parsedURI.Host != "" &&
		(parsedURI.Scheme == "http" || parsedURI.Scheme == "https") &&
		!strings.ContainsAny(uri, "\"\\\n\r\x00")
	if !validURI {
		return fmt.Errorf(
			"%s %q for OAuth provider %q must be a valid http or https URL without unsafe characters",
			field,
			uri,
			providerName,
		)
	}
	return nil
}
