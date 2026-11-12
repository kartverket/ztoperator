package constants

const (
	SkiperatorApplicationRefLabel  = "application.skiperator.no/app-name"
	SkiperatorAppContainerPortName = "main"

	IstioAmbientLabelKey    = "istio.io/dataplane-mode"
	IstioAmbientLabelValue  = "ambient"
	IstioAmbientOptOutValue = "none"

	IstioInjectionLabelKey        = "istio-injection"
	IstioInjectionEnabledValue    = "enabled"
	IstioRevisionLabelKey         = "istio.io/rev"
	IstioSidecarInjectLabelKey    = "sidecar.istio.io/inject"
	IstioSidecarInjectTrueValue   = "true"
	IstioAmbientRedirectionKey    = "ambient.istio.io/redirection"
	IstioAmbientRedirectionOptOut = "disabled"

	EnvoyBootstrapConfigMapKey        = "envoy.json"
	EnvoyBootstrapUpstreamClusterName = "upstream"

	EnvoyOAuth2TokenSecretFileName  = "token-secret.yaml"
	EnvoyOAuth2HmacSecretFileName   = "hmac-secret.yaml"
	EnvoyOAuth2CredentialsDirectory = "/etc/istio/config"
	EnvoyOAuth2TokenSecretFilePath  = EnvoyOAuth2CredentialsDirectory + "/" + EnvoyOAuth2TokenSecretFileName
	EnvoyOAuth2HmacSecretFilePath   = EnvoyOAuth2CredentialsDirectory + "/" + EnvoyOAuth2HmacSecretFileName
)
