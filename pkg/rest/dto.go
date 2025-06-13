package rest

type DiscoveryDocument struct {
	Issuer                *string `json:"issuer"`
	AuthorizationEndpoint *string `json:"authorization_endpoint"`
	TokenEndpoint         *string `json:"token_endpoint"`
	JwksUri               *string `json:"jwks_uri"`
}
