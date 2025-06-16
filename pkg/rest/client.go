package rest

import (
	"errors"
	"fmt"
	log2 "github.com/kartverket/ztoperator/pkg/log"
	"resty.dev/v3"
)

func GetOAuthDiscoveryDocument(uri string, rLog log2.Logger) (*DiscoveryDocument, error) {
	var discoveryDocument DiscoveryDocument

	if _, exists := wellknownUriToDiscoveryDocument[uri]; exists {
		rLog.Info(fmt.Sprintf("Using cached discovery document for well-known uri: %s", uri))
		cachedDiscoveryDocument := wellknownUriToDiscoveryDocument[uri]
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
