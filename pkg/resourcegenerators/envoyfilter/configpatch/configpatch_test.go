package configpatch_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/kartverket/ztoperator/pkg/model"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultClientID                 = "my-client"
	defaultTokenEndpointClusterName = "oauth"
)

func TestGetOAuth2ClusterConfig_NoTLSTransportSocketWhenHttp(t *testing.T) {
	result, _ := configpatch.GetOAuth2ClusterConfig("oauth2", "http://mock-oauth2.auth:8080/entraid/token")

	assert.Nil(t, (*result)["transport_socket"], "internal cluster should not have TLS transport socket")
}

func TestGetOAuth2ClusterConfig_HasTLSTransportSocketWhenHttps(t *testing.T) {
	result, _ := configpatch.GetOAuth2ClusterConfig("oauth2", "https://login.microsoftonline.com/token")

	ts, ok := (*result)["transport_socket"].(map[string]interface{})
	require.True(t, ok, "external cluster must have transport_socket")
	assert.Equal(t, "envoy.transport_sockets.tls", ts["name"])

	typed := ts["typed_config"].(map[string]interface{})
	assert.Equal(t, "login.microsoftonline.com", typed["sni"])
}

func TestGetOAuth2FilterConfig_EndSessionEndpoint_PresentWhenSet(t *testing.T) {
	autoLoginConfig := defaultAutoLoginConfig()
	identityProviderUris := defaultIdentityProviderUris()

	result := configpatch.GetOAuth2FilterConfig(
		defaultTokenEndpointClusterName,
		autoLoginConfig,
		identityProviderUris,
		defaultClientID,
	)

	inner := oauthInnerConfig(t, result)
	assert.Equal(t, "https://idp.example.com/endsession", inner["end_session_endpoint"])
}

func TestGetOAuth2FilterConfig_EndSessionEndpoint_AbsentWhenNil(t *testing.T) {
	autoLoginConfig := defaultAutoLoginConfig()
	identityProviderUris := defaultIdentityProviderUris()
	identityProviderUris.EndSessionURI = nil

	result := configpatch.GetOAuth2FilterConfig(
		defaultTokenEndpointClusterName,
		autoLoginConfig,
		identityProviderUris,
		defaultClientID,
	)

	inner := oauthInnerConfig(t, result)
	_, present := inner["end_session_endpoint"]
	assert.False(t, present, "end_session_endpoint must be absent when EndSessionURI is nil")
}

func TestGetOAuth2FilterConfig_Scopes_ForwardedAsIs(t *testing.T) {
	// Defaulting of "openid" is handled upstream in state.AutoLoginConfig.SetSaneDefaults,
	// so the config patch generator must forward whatever scopes it receives verbatim.
	autoLoginConfig := defaultAutoLoginConfig()
	autoLoginConfig.Scopes = []string{"offline_access"} // openid deliberately omitted
	identityProviderUris := defaultIdentityProviderUris()

	result := configpatch.GetOAuth2FilterConfig(
		defaultTokenEndpointClusterName,
		autoLoginConfig,
		identityProviderUris,
		defaultClientID,
	)

	inner := oauthInnerConfig(t, result)
	scopes := inner["auth_scopes"].([]interface{})
	scopeStrs := make([]string, 0, len(scopes))
	for _, s := range scopes {
		scopeStrs = append(scopeStrs, s.(string))
	}
	assert.Equal(t, []string{"offline_access"}, scopeStrs)
}

func TestGetOAuth2FilterConfig_Scopes_CustomScopesPreserved(t *testing.T) {
	autoLoginConfig := defaultAutoLoginConfig()
	autoLoginConfig.Scopes = []string{"openid", "profile", "email"}
	identityProviderUris := defaultIdentityProviderUris()

	result := configpatch.GetOAuth2FilterConfig(
		defaultTokenEndpointClusterName,
		autoLoginConfig,
		identityProviderUris,
		defaultClientID,
	)

	inner := oauthInnerConfig(t, result)
	scopes := inner["auth_scopes"].([]interface{})
	scopeStrs := make([]string, 0, len(scopes))
	for _, s := range scopes {
		scopeStrs = append(scopeStrs, s.(string))
	}
	assert.Equal(t, []string{"openid", "profile", "email"}, scopeStrs)
}

func TestGetOAuth2FilterConfig_Resources_PresentWhenSet(t *testing.T) {
	autoLoginConfig := defaultAutoLoginConfig()
	autoLoginConfig.ResourceIndicators = []string{
		"https://example.com/api-1",
		"https://example.com/api-2",
	}
	identityProviderUris := defaultIdentityProviderUris()

	result := configpatch.GetOAuth2FilterConfig(
		defaultTokenEndpointClusterName,
		autoLoginConfig,
		identityProviderUris,
		defaultClientID,
	)

	inner := oauthInnerConfig(t, result)
	resources, ok := inner["resources"].([]interface{})
	require.True(t, ok, "resources must be present when AcceptedResources is set")
	require.Len(t, resources, 2)
	assert.Equal(t, "https://example.com/api-1", resources[0])
	assert.Equal(t, "https://example.com/api-2", resources[1])
}

func TestGetOAuth2FilterConfig_Resources_AbsentWhenNil(t *testing.T) {
	autoLoginConfig := defaultAutoLoginConfig()
	identityProviderUris := defaultIdentityProviderUris()

	result := configpatch.GetOAuth2FilterConfig(
		defaultTokenEndpointClusterName,
		autoLoginConfig,
		identityProviderUris,
		defaultClientID,
	)

	inner := oauthInnerConfig(t, result)
	_, present := inner["resources"]
	assert.False(t, present, "resources must be absent when AcceptedResources is nil")
}

func TestGetOAuth2FilterConfig_PassThroughAndDenyRedirectMatchers(t *testing.T) {
	autoLoginConfig := defaultAutoLoginConfig()
	identityProviderUris := defaultIdentityProviderUris()

	result := configpatch.GetOAuth2FilterConfig(
		defaultTokenEndpointClusterName,
		autoLoginConfig,
		identityProviderUris,
		defaultClientID,
	)

	inner := oauthInnerConfig(t, result)

	ptm := inner["pass_through_matcher"].([]interface{})
	require.Len(t, ptm, 2)
	authHeader := ptm[0].(map[string]interface{})
	assert.Equal(t, "authorization", authHeader["name"])
	bypassHeader := ptm[1].(map[string]interface{})
	assert.Equal(t, luascript.BypassOauthLoginHeaderName, bypassHeader["name"])

	drm := inner["deny_redirect_matcher"].([]interface{})
	require.Len(t, drm, 1)
	denyHeader := drm[0].(map[string]interface{})
	assert.Equal(t, luascript.DenyRedirectHeaderName, denyHeader["name"])
}

func TestGetOAuth2FilterConfig_CookieConfigs_SameSiteLax(t *testing.T) {
	autoLoginConfig := defaultAutoLoginConfig()
	identityProviderUris := defaultIdentityProviderUris()

	result := configpatch.GetOAuth2FilterConfig(
		defaultTokenEndpointClusterName,
		autoLoginConfig,
		identityProviderUris,
		defaultClientID,
	)

	inner := oauthInnerConfig(t, result)
	cookieConfigs, ok := inner["cookie_configs"].(map[string]interface{})
	require.True(t, ok, "cookie_configs must be present")

	expectedCookies := []string{
		"bearer_token_cookie_config",
		"oauth_hmac_cookie_config",
		"oauth_expires_cookie_config",
		"id_token_cookie_config",
		"refresh_token_cookie_config",
		"oauth_nonce_cookie_config",
		"code_verifier_cookie_config",
	}
	for _, key := range expectedCookies {
		cfg, present := cookieConfigs[key].(map[string]interface{})
		require.Truef(t, present, "%s must be present in cookie_configs", key)
		assert.Equalf(t, "LAX", cfg["same_site"], "%s.same_site must be LAX", key)
	}
}

func oauthInnerConfig(t *testing.T, patch map[string]interface{}) map[string]interface{} {
	t.Helper()
	typed, ok := patch["typed_config"].(map[string]interface{})
	require.True(t, ok, "typed_config not found or wrong type")
	cfg, ok := typed["config"].(map[string]interface{})
	require.True(t, ok, "config not found or wrong type")
	return cfg
}

func defaultIdentityProviderUris() model.IdentityProviderUris {
	endSession := "https://idp.example.com/endsession"
	return model.IdentityProviderUris{
		TokenURI:         "http://mock-oauth2.auth:8080/entraid/token",
		AuthorizationURI: "http://mock-oauth2.auth:8080/entraid/authorize",
		EndSessionURI:    &endSession,
	}
}

func defaultAutoLoginConfig() model.AutoLoginConfig {
	return model.AutoLoginConfig{
		Enabled:      true,
		RedirectPath: "/oauth2/callback",
		LogoutPath:   "/logout",
		Scopes:       []string{"openid"},
	}
}
