package rest

import "github.com/kartverket/ztoperator/pkg/utilities"

type DiscoveryDocument struct {
	Issuer                *string `json:"issuer"`
	AuthorizationEndpoint *string `json:"authorization_endpoint"`
	TokenEndpoint         *string `json:"token_endpoint"`
	JwksURI               *string `json:"jwks_uri"`
	EndSessionEndpoint    *string `json:"end_session_endpoint"`
}

func GetWellknownURIToDiscoveryDocument() map[string]DiscoveryDocument {
	return map[string]DiscoveryDocument{
		"http://mock-oauth2.auth:8080/entraid/.well-known/openid-configuration": {
			Issuer:                utilities.Ptr("http://mock-oauth2.auth:8080/entraid"),
			AuthorizationEndpoint: utilities.Ptr("http://mock-oauth2.auth:8080/entraid/authorize"),
			TokenEndpoint:         utilities.Ptr("http://mock-oauth2.auth:8080/entraid/token"),
			JwksURI:               utilities.Ptr("http://mock-oauth2.auth:8080/entraid/jwks"),
			EndSessionEndpoint:    utilities.Ptr("http://mock-oauth2.auth:8080/entraid/endsession"),
		},
		"http://mock-oauth2.auth:8080/smapi/.well-known/openid-configuration": {
			Issuer:                utilities.Ptr("http://mock-oauth2.auth:8080/smapi"),
			AuthorizationEndpoint: utilities.Ptr("http://mock-oauth2.auth:8080/smapi/authorize"),
			TokenEndpoint:         utilities.Ptr("http://mock-oauth2.auth:8080/smapi/token"),
			JwksURI:               utilities.Ptr("http://mock-oauth2.auth:8080/smapi/jwks"),
			EndSessionEndpoint:    utilities.Ptr("http://mock-oauth2.auth:8080/smapi/endsession"),
		},
		"http://mock-oauth2.auth:8080/maskinporten/.well-known/openid-configuration": {
			Issuer:                utilities.Ptr("http://mock-oauth2.auth:8080/maskinporten"),
			AuthorizationEndpoint: utilities.Ptr("http://mock-oauth2.auth:8080/maskinporten/authorize"),
			TokenEndpoint:         utilities.Ptr("http://mock-oauth2.auth:8080/maskinporten/token"),
			JwksURI:               utilities.Ptr("http://mock-oauth2.auth:8080/maskinporten/jwks"),
			EndSessionEndpoint:    utilities.Ptr("http://mock-oauth2.auth:8080/maskinporten/endsession"),
		},
		"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/v2.0/.well-known/openid-configuration": {
			Issuer: utilities.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/v2.0",
			),
			AuthorizationEndpoint: utilities.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/oauth2/v2.0/authorize",
			),
			TokenEndpoint: utilities.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/oauth2/v2.0/token",
			),
			JwksURI: utilities.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/discovery/v2.0/keys",
			),
			EndSessionEndpoint: utilities.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/oauth2/v2.0/logout",
			),
		},
		"https://idporten.no/.well-known/openid-configuration": {
			Issuer:                utilities.Ptr("https://idporten.no"),
			AuthorizationEndpoint: utilities.Ptr("https://login.idporten.no/authorize"),
			TokenEndpoint:         utilities.Ptr("https://idporten.no/token"),
			JwksURI:               utilities.Ptr("https://idporten.no/jwks.json"),
			EndSessionEndpoint:    utilities.Ptr("https://login.idporten.no/logout"),
		},
		"https://maskinporten.no/.well-known/oauth-authorization-server": {
			Issuer:        utilities.Ptr("https://maskinporten.no/"),
			TokenEndpoint: utilities.Ptr("https://maskinporten.no/token"),
			JwksURI:       utilities.Ptr("https://maskinporten.no/jwk"),
		},
	}
}
