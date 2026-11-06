package httpfilter

import (
	"fmt"

	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	httpconnectionmanagerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter/jwtauthn"
	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter/lua"
	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter/oauth2"
	"github.com/kartverket/ztoperator/pkg/model"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	EnvoyExtensionHttpFilterJwtAuthentication = "envoy.filters.http.jwt_authn"
	EnvoyExtensionHttpFilterLua               = "envoy.filters.http.lua"
	EnvoyExtensionHttpFilterOAuth2            = "envoy.filters.http.oauth2"
	EnvoyExtensionHttpFilterRouter            = "envoy.filters.http.router"
)

func JwtAuthenticationHttpFilter(
	issuerURI, jwksURI, jwksClusterName string, audiences []string,
) (*httpconnectionmanagerv3.HttpFilter, error) {
	jwtAuthentication, err := jwtauthn.JwtAuthentication(
		issuerURI,
		jwksURI,
		jwksClusterName,
		audiences,
	)
	if err != nil {
		return nil, err
	}
	jwtAuthenticationTypedFilterConfig, err := anypb.New(jwtAuthentication)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JWT authentication filter: %w", err)
	}
	jwtAuthenticationHttpFilter := &httpconnectionmanagerv3.HttpFilter{
		Name:       EnvoyExtensionHttpFilterJwtAuthentication,
		ConfigType: &httpconnectionmanagerv3.HttpFilter_TypedConfig{TypedConfig: jwtAuthenticationTypedFilterConfig},
	}

	var errJwtAuthenticationHttpFilterValidateErr error
	if validateErr := jwtAuthenticationHttpFilter.ValidateAll(); validateErr != nil {
		errJwtAuthenticationHttpFilterValidateErr = fmt.Errorf(
			"error validating JWT authentication HTTP filter: %w",
			validateErr,
		)
	}
	return jwtAuthenticationHttpFilter, errJwtAuthenticationHttpFilterValidateErr
}

func LuaHttpFilter(luaScriptAsString string) (*httpconnectionmanagerv3.HttpFilter, error) {
	luaFilter, err := lua.Lua(luaScriptAsString)
	if err != nil {
		return nil, err
	}
	luaTypedFilterConfig, err := anypb.New(luaFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Lua filter: %w", err)
	}

	luaHttpFilter := &httpconnectionmanagerv3.HttpFilter{
		Name:       EnvoyExtensionHttpFilterLua,
		ConfigType: &httpconnectionmanagerv3.HttpFilter_TypedConfig{TypedConfig: luaTypedFilterConfig},
	}

	var errLuaHttpFilterValidateErr error
	if validateErr := luaHttpFilter.ValidateAll(); validateErr != nil {
		errLuaHttpFilterValidateErr = fmt.Errorf(
			"error validating Lua HTTP filter: %w",
			validateErr,
		)
	}
	return luaHttpFilter, errLuaHttpFilterValidateErr
}

func OAuth2HttpFilter(
	autoLoginConfig model.AutoLoginConfig,
	identityProviderUris model.IdentityProviderUris,
	clientID string,
	tokenEndpointClusterName string,
) (*httpconnectionmanagerv3.HttpFilter, error) {
	oAuth2, err := oauth2.OAuth2(
		identityProviderUris,
		clientID,
		autoLoginConfig.ResourceIndicators,
		autoLoginConfig.Scopes,
		autoLoginConfig.LogoutPath,
		autoLoginConfig.RedirectPath,
		tokenEndpointClusterName,
		autoLoginConfig.PostLogoutRedirectURI,
	)
	if err != nil {
		return nil, err
	}
	oAuth2TypedFilterConfig, err := anypb.New(oAuth2)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OAuth2 filter: %w", err)
	}
	oauth2HttpFilter := &httpconnectionmanagerv3.HttpFilter{
		Name:       EnvoyExtensionHttpFilterOAuth2,
		ConfigType: &httpconnectionmanagerv3.HttpFilter_TypedConfig{TypedConfig: oAuth2TypedFilterConfig},
	}

	var errOAuth2HttpFilterValidateErr error
	if validateErr := oauth2HttpFilter.ValidateAll(); validateErr != nil {
		errOAuth2HttpFilterValidateErr = fmt.Errorf("error validating OAuth2 HTTP filter: %w", validateErr)
	}
	return oauth2HttpFilter, errOAuth2HttpFilterValidateErr
}

func RouterFilter() (*httpconnectionmanagerv3.HttpFilter, error) {
	typedConfig, err := anypb.New(&routerv3.Router{})
	if err != nil {
		return nil, fmt.Errorf("marshal router filter: %w", err)
	}
	return &httpconnectionmanagerv3.HttpFilter{
		Name:       EnvoyExtensionHttpFilterRouter,
		ConfigType: &httpconnectionmanagerv3.HttpFilter_TypedConfig{TypedConfig: typedConfig},
	}, nil
}
