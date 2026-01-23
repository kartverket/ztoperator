package rest

import (
	"errors"
	"fmt"

	"resty.dev/v3"

	"github.com/kartverket/ztoperator/pkg/log"
)

type DiscoveryDocumentResolver interface {
	GetOAuthDiscoveryDocument(uri string, rLog log.Logger) (*DiscoveryDocument, error)
}

type DefaultDiscoveryDocumentResolver struct{}

func NewDefaultDiscoveryDocumentResolver() *DefaultDiscoveryDocumentResolver {
	return &DefaultDiscoveryDocumentResolver{}
}

func (r *DefaultDiscoveryDocumentResolver) GetOAuthDiscoveryDocument(
	uri string,
	rLog log.Logger,
) (*DiscoveryDocument, error) {
	var discoveryDocument DiscoveryDocument

	wellknownURIToDiscoveryDocument := GetWellknownURIToDiscoveryDocument()

	if _, exists := wellknownURIToDiscoveryDocument[uri]; exists {
		rLog.Info(fmt.Sprintf("Using cached discovery document for well-known uri: %s", uri))
		cachedDiscoveryDocument := wellknownURIToDiscoveryDocument[uri]
		return &cachedDiscoveryDocument, nil
	}
	rLog.Info(fmt.Sprintf("Fetching discovery document for well-known uri: %s", uri))
	client := resty.New()
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
