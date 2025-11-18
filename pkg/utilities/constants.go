package utilities

const (
	TokenProxyImageName               = "ztoperator-token-proxy"
	TokenProxyImageTag                = "latest"
	TokenProxyPort                    = 8080
	TokenProxyTokenEndpointEnvVarName = "ZTOPERATOR_TOKEN_PROXY_TOKEN_ENDPOINT"
	TokenProxyIssuerEnvVarName        = "ZTOPERATOR_TOKEN_PROXY_ISSUER"
	TokenProxyPrivateJWKEnvVarName    = "ZTOPERATOR_TOKEN_PROXY_PRIVATE_JWK"
	TokenProxyServerModeEnvVarName    = "GIN_MODE"
	TokenProxyServerModeEnvVarValue   = "release"

	IstioProxyPort          = 15020
	IstioDataplaneNamespace = "istio-system"
	IstioGatewaysNamespace  = "istio-gateways"

	EnvoyFilterTokenSecretFileName = "token-secret.yaml"
	EnvoyFilterHmacSecretFileName  = "hmac-secret.yaml"
	IstioTokenSecretVolumePath     = "/etc/istio/config/" + EnvoyFilterTokenSecretFileName
	IstioHmacSecretVolumePath      = "/etc/istio/config/" + EnvoyFilterHmacSecretFileName
	IstioCredentialsVolumePath     = "/etc/istio/config"
)
