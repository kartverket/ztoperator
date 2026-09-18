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

// ValidateWellKnownURI is kept for callers of the REST package. The canonical
// implementation is retained here for compatibility with the REST client.
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

// HTTPDiscoveryDocumentResolver is used only while loading the startup cache.
type HTTPDiscoveryDocumentResolver struct{}

// DefaultDiscoveryDocumentResolver is kept for callers using the repository's
// preconfigured discovery documents. It is cache-only; unknown URIs are not
// fetched over HTTP.
type DefaultDiscoveryDocumentResolver struct {
	documents map[string]DiscoveryDocument
}

// NewHTTPDiscoveryDocumentResolver creates the resolver used for startup
// discovery-document fetching.
func NewHTTPDiscoveryDocumentResolver() *HTTPDiscoveryDocumentResolver {
	return &HTTPDiscoveryDocumentResolver{}
}

func NewDefaultDiscoveryDocumentResolver() *DefaultDiscoveryDocumentResolver {
	return &DefaultDiscoveryDocumentResolver{
		documents: GetWellknownURIToDiscoveryDocument(),
	}
}

func (r *DefaultDiscoveryDocumentResolver) GetOAuthDiscoveryDocument(
	uri string,
	_ log.Logger,
) (*DiscoveryDocument, error) {
	if r == nil {
		return nil, errors.New("default discovery document resolver is not configured")
	}
	document, exists := r.documents[uri]
	if !exists {
		return nil, fmt.Errorf("well-known URI %q is not in the configured allowlist", uri)
	}
	returnDocument := cloneDiscoveryDocument(document)
	return &returnDocument, nil
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
