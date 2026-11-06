package constants

const (
	EnvoyOAuth2TokenSecretFileName  = "token-secret.yaml"
	EnvoyOAuth2HmacSecretFileName   = "hmac-secret.yaml"
	EnvoyOAuth2CredentialsDirectory = "/etc/istio/config"
	EnvoyOAuth2TokenSecretFilePath  = EnvoyOAuth2CredentialsDirectory + "/" + EnvoyOAuth2TokenSecretFileName
	EnvoyOAuth2HmacSecretFilePath   = EnvoyOAuth2CredentialsDirectory + "/" + EnvoyOAuth2HmacSecretFileName
)
