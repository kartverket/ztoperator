package skiperatorapplication

import (
	"github.com/kartverket/skiperator/api/v1alpha1"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/utilities"
	v3 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TokenProxyTokenEndpointEnvVarName = "ZTOPERATOR_TOKEN_PROXY_TOKEN_ENDPOINT"
	TokenProxyIssuerEnvVarName        = "ZTOPERATOR_TOKEN_PROXY_ISSUER"
	TokenProxyPrivateJWKEnvVarName    = "ZTOPERATOR_TOKEN_PROXY_PRIVATE_JWK"
	TokenProxyServerModeEnvVarName    = "GIN_MODE"
	TokenProxyServerModeEnvVarValue   = "RELEASE"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v1alpha1.Application {
	if scope.IsMisconfigured() ||
		scope.OAuthCredentials.ClientAuthMethod != state.PrivateKeyJWT ||
		scope.AppLabel == nil {
		return nil
	}

	idpAsParsedURL, err := utilities.GetParsedURL(scope.IdentityProviderUris.TokenURI)
	if err != nil {
		panic(
			"failed to get issuer hostname from token URI " + scope.IdentityProviderUris.TokenURI + " due to the following error: " + err.Error(),
		)
	}

	return &v1alpha1.Application{
		ObjectMeta: objectMeta,
		Spec: v1alpha1.ApplicationSpec{
			PodSettings: &podtypes.PodSettings{
				Annotations: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
			},
			Image: "ztoperator-token-proxy:latest",
			Port:  8080,
			AccessPolicy: &podtypes.AccessPolicy{
				Inbound: &podtypes.InboundPolicy{
					Rules: []podtypes.InternalRule{
						{
							Application: *scope.AppLabel,
						},
					},
				},
				Outbound: podtypes.OutboundPolicy{
					External: []podtypes.ExternalRule{
						{
							Host: idpAsParsedURL.Hostname(),
						},
					},
				},
			},
			Env: []v3.EnvVar{
				{
					Name:  TokenProxyServerModeEnvVarName,
					Value: TokenProxyServerModeEnvVarValue,
				},
				{
					Name:  TokenProxyTokenEndpointEnvVarName,
					Value: scope.IdentityProviderUris.TokenURI,
				},
				{
					Name:  TokenProxyIssuerEnvVarName,
					Value: scope.IdentityProviderUris.IssuerURI,
				},
				{
					Name: TokenProxyPrivateJWKEnvVarName,
					ValueFrom: &v3.EnvVarSource{
						SecretKeyRef: &v3.SecretKeySelector{
							LocalObjectReference: v3.LocalObjectReference{},
							Key:                  scope.AuthPolicy.Spec.OAuthCredentials.PrivateJWKKey,
							Optional:             utilities.Ptr(false),
						},
					},
				},
			},
		},
	}
}
