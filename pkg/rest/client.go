package rest

import (
	"errors"
	"fmt"
	"time"

	"resty.dev/v3"

	"github.com/kartverket/ztoperator/pkg/log"
)

type DiscoveryDocumentResolver interface {
	GetOAuthDiscoveryDocument(uri string, rLog log.Logger) (*DiscoveryDocument, error)
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

// NewDiscoveryDocumentMapResolver creates a resolver that only reads from the
// provided, already loaded discovery documents.
func NewDiscoveryDocumentMapResolver(documents map[string]DiscoveryDocument) *DefaultDiscoveryDocumentResolver {
	return &DefaultDiscoveryDocumentResolver{documents: documents}
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
