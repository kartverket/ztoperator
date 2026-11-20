package model

import (
	"fmt"
	"slices"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	oauth2v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/oauth2/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const openIDScope = "openid"

type Scope struct {
	AuthPolicy             ztoperatorv1alpha1.AuthPolicy
	Audiences              []string
	AutoLoginConfig        AutoLoginConfig
	OAuthCredentials       OAuthCredentials
	IdentityProviderUris   IdentityProviderUris
	IstioAmbientConfig     IstioAmbientConfig
	Descendants            []Descendant[client.Object]
	InvalidConfig          bool
	ValidationErrorMessage *string
}

type OAuthCredentials struct {
	ClientID    *string
	HmacSecret  HmacSecret
	TokenSecret TokenSecret
}

type HmacSecret struct {
	Value *tlsv3.Secret
}

type TokenSecret struct {
	AuthType            oauth2v3.OAuth2Config_AuthType
	Value               *tlsv3.Secret
	PrivateKeyJwtConfig *oauth2v3.PrivateKeyJwtConfig
}

type IdentityProviderUris struct {
	IssuerURI        string
	JwksURI          string
	TokenURI         string
	AuthorizationURI string
	EndSessionURI    *string
}

type IstioAmbientConfig struct {
	Enabled        bool
	EnvoyBootstrap *bootstrapv3.Bootstrap
}

type Descendant[T client.Object] struct {
	ID             string
	Object         T
	ErrorMessage   *string
	SuccessMessage *string
}

func (s *Scope) GetErrors() []string {
	var errs []string
	if s != nil {
		for _, d := range s.Descendants {
			if d.ErrorMessage != nil {
				errs = append(errs, *d.ErrorMessage)
			}
		}
	}
	return errs
}

func (s *Scope) ReplaceDescendant(
	obj client.Object,
	errorMessage *string,
	successMessage *string,
	resourceKind, resourceName string,
) {
	if s != nil {
		expectedID := GetID(resourceKind, resourceName)
		for i, d := range s.Descendants {
			if d.ID == expectedID {
				s.Descendants[i] = Descendant[client.Object]{
					ID:             expectedID,
					Object:         obj,
					ErrorMessage:   errorMessage,
					SuccessMessage: successMessage,
				}
				return
			}
		}
		s.Descendants = append(s.Descendants, Descendant[client.Object]{
			ID:             GetID(resourceKind, resourceName),
			Object:         obj,
			ErrorMessage:   errorMessage,
			SuccessMessage: successMessage,
		})
	}
}

func GetID(resourceKind, resourceName string) string {
	return fmt.Sprintf("%s-%s", resourceKind, resourceName)
}

func (a *AutoLoginConfig) SetSaneDefaults(autoLogin ztoperatorv1alpha1.AutoLogin) {
	if autoLogin.RedirectPath == nil || *autoLogin.RedirectPath == "" {
		a.RedirectPath = "/oauth2/callback"
	} else {
		a.RedirectPath = *autoLogin.RedirectPath
	}
	if autoLogin.LogoutPath == nil || *autoLogin.LogoutPath == "" {
		a.LogoutPath = "/logout"
	} else {
		a.LogoutPath = *autoLogin.LogoutPath
	}
	if !slices.Contains(a.Scopes, openIDScope) {
		a.Scopes = append(a.Scopes, openIDScope)
	}
}
