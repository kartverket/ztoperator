package rest

import (
	"errors"
	"resty.dev/v3"
)

func GetOAuthDiscoveryDocument(uri string) (*DiscoveryDocument, error) {
	var discoveryDocument DiscoveryDocument

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
