package rest

import "github.com/kartverket/ztoperator/pkg/helperfunctions"

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
			Issuer:                helperfunctions.Ptr("http://mock-oauth2.auth:8080/entraid"),
			AuthorizationEndpoint: helperfunctions.Ptr("http://mock-oauth2.auth:8080/entraid/authorize"),
			TokenEndpoint:         helperfunctions.Ptr("http://mock-oauth2.auth:8080/entraid/token"),
			JwksURI:               helperfunctions.Ptr("http://mock-oauth2.auth:8080/entraid/jwks"),
			EndSessionEndpoint:    helperfunctions.Ptr("http://mock-oauth2.auth:8080/entraid/endsession"),
		},
		"http://mock-oauth2.auth:8080/smapi/.well-known/openid-configuration": {
			Issuer:                helperfunctions.Ptr("http://mock-oauth2.auth:8080/smapi"),
			AuthorizationEndpoint: helperfunctions.Ptr("http://mock-oauth2.auth:8080/smapi/authorize"),
			TokenEndpoint:         helperfunctions.Ptr("http://mock-oauth2.auth:8080/smapi/token"),
			JwksURI:               helperfunctions.Ptr("http://mock-oauth2.auth:8080/smapi/jwks"),
			EndSessionEndpoint:    helperfunctions.Ptr("http://mock-oauth2.auth:8080/smapi/endsession"),
		},
		"http://mock-oauth2.auth:8080/maskinporten/.well-known/openid-configuration": {
			Issuer:                helperfunctions.Ptr("http://mock-oauth2.auth:8080/maskinporten"),
			AuthorizationEndpoint: helperfunctions.Ptr("http://mock-oauth2.auth:8080/maskinporten/authorize"),
			TokenEndpoint:         helperfunctions.Ptr("http://mock-oauth2.auth:8080/maskinporten/token"),
			JwksURI:               helperfunctions.Ptr("http://mock-oauth2.auth:8080/maskinporten/jwks"),
			EndSessionEndpoint:    helperfunctions.Ptr("http://mock-oauth2.auth:8080/maskinporten/endsession"),
		},
		"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/v2.0/.well-known/openid-configuration": {
			Issuer: helperfunctions.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/v2.0",
			),
			AuthorizationEndpoint: helperfunctions.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/oauth2/v2.0/authorize",
			),
			TokenEndpoint: helperfunctions.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/oauth2/v2.0/token",
			),
			JwksURI: helperfunctions.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/discovery/v2.0/keys",
			),
			EndSessionEndpoint: helperfunctions.Ptr(
				"https://login.microsoftonline.com/7f74c8a2-43ce-46b2-b0e8-b6306cba73a3/oauth2/v2.0/logout",
			),
		},
		"https://idporten.no/.well-known/openid-configuration": {
			Issuer:                helperfunctions.Ptr("https://idporten.no"),
			AuthorizationEndpoint: helperfunctions.Ptr("https://login.idporten.no/authorize"),
			TokenEndpoint:         helperfunctions.Ptr("https://idporten.no/token"),
			JwksURI:               helperfunctions.Ptr("https://idporten.no/jwks.json"),
			EndSessionEndpoint:    helperfunctions.Ptr("https://login.idporten.no/logout"),
		},
		"https://maskinporten.no/.well-known/oauth-authorization-server": {
			Issuer:        helperfunctions.Ptr("https://maskinporten.no/"),
			TokenEndpoint: helperfunctions.Ptr("https://maskinporten.no/token"),
			JwksURI:       helperfunctions.Ptr("https://maskinporten.no/jwk"),
		},
	}
}
