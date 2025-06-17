package rest

import "github.com/kartverket/ztoperator/pkg/utils"

type DiscoveryDocument struct {
	Issuer                *string `json:"issuer"`
	AuthorizationEndpoint *string `json:"authorization_endpoint"`
	TokenEndpoint         *string `json:"token_endpoint"`
	JwksURI               *string `json:"jwks_uri"`
}

func GetWellknownURIToDiscoveryDocument() map[string]DiscoveryDocument {
	return map[string]DiscoveryDocument{
		"http://mock-oauth2.auth:8080/entraid/.well-known/openid-configuration": {
			Issuer:                utils.Ptr("https://fake.auth/entraid"),
			AuthorizationEndpoint: utils.Ptr("https://fake.auth/entraid/authorize"),
			TokenEndpoint:         utils.Ptr("https://fake.auth/entraid/token"),
			JwksURI:               utils.Ptr("http://mock-oauth2.auth:8080/entraid/jwks"),
		},
		"http://mock-oauth2.auth:8080/smapi/.well-known/openid-configuration": {
			Issuer:                utils.Ptr("https://fake.auth/smapi"),
			AuthorizationEndpoint: utils.Ptr("http://mock-oauth2.auth:8080/smapi/authorize"),
			TokenEndpoint:         utils.Ptr("http://mock-oauth2.auth:8080/smapi/token"),
			JwksURI:               utils.Ptr("http://mock-oauth2.auth:8080/smapi/jwks"),
		},
		"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/v2.0/.well-known/openid-configuration": {
			Issuer: utils.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/v2.0",
			),
			AuthorizationEndpoint: utils.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/oauth2/v2.0/authorize",
			),
			TokenEndpoint: utils.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/oauth2/v2.0/token",
			),
			JwksURI: utils.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/discovery/v2.0/keys",
			),
		},
		"https://idporten.no/.well-known/openid-configuration": {
			Issuer:                utils.Ptr("https://idporten.no"),
			AuthorizationEndpoint: utils.Ptr("https://login.idporten.no/authorize"),
			TokenEndpoint:         utils.Ptr("https://idporten.no/token"),
			JwksURI:               utils.Ptr("https://idporten.no/jwks.json"),
		},
		"https://maskinporten.no/.well-known/oauth-authorization-server": {
			Issuer:        utils.Ptr("https://maskinporten.no/"),
			TokenEndpoint: utils.Ptr("https://maskinporten.no/token"),
			JwksURI:       utils.Ptr("https://maskinporten.no/jwk"),
		},
	}
}
