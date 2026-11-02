package configpatch_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/kartverket/ztoperator/pkg/model"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	defaultClientID                 = "my-client"
	defaultTokenEndpointClusterName = "oauth"
)

func TestGetOAuth2ClusterConfig_NoTLSTransportSocketWhenHttp(t *testing.T) {
	result, err := configpatch.GetTokenEndpointClusterConfigPatchValue(
		"oauth2",
		"http://mock-oauth2.auth:8080/entraid/token",
	)

	cluster := toMap(t, result, err)
	_, present := cluster["transport_socket"]
	assert.False(t, present, "internal cluster should not have TLS transport socket")
}

func TestGetOAuth2ClusterConfig_HasTLSTransportSocketWhenHttps(t *testing.T) {
	result, err := configpatch.GetTokenEndpointClusterConfigPatchValue(
		"oauth2",
		"https://login.microsoftonline.com/token",
	)

	cluster := toMap(t, result, err)
	ts, ok := cluster["transport_socket"].(map[string]any)
	require.True(t, ok, "external cluster must have transport_socket")
	assert.Equal(t, "envoy.transport_sockets.tls", ts["name"])

	typed, ok := ts["typed_config"].(map[string]any)
	require.True(t, ok, "transport_socket.typed_config not found or wrong type")
	assert.Equal(t, "login.microsoftonline.com", typed["sni"])
}

func TestGetOAuth2FilterConfig_EndSessionEndpoint_PresentWhenSet(t *testing.T) {
	inner := oauthInnerConfig(t, defaultAutoLoginConfig(), defaultIdentityProviderUris())

	assert.Equal(t, "https://idp.example.com/endsession", inner["end_session_endpoint"])
}

func TestGetOAuth2FilterConfig_EndSessionEndpoint_AbsentWhenNil(t *testing.T) {
	identityProviderUris := defaultIdentityProviderUris()
	identityProviderUris.EndSessionURI = nil

	inner := oauthInnerConfig(t, defaultAutoLoginConfig(), identityProviderUris)

	_, present := inner["end_session_endpoint"]
	assert.False(t, present, "end_session_endpoint must be absent when EndSessionURI is nil")
}

func TestGetOAuth2FilterConfig_Scopes_ForwardedAsIs(t *testing.T) {
	// Defaulting of "openid" is handled upstream in AutoLoginConfig.SetSaneDefaults,
	// so the config patch generator must forward whatever scopes it receives verbatim.
	autoLoginConfig := defaultAutoLoginConfig()
	autoLoginConfig.Scopes = []string{"offline_access"} // openid deliberately omitted

	inner := oauthInnerConfig(t, autoLoginConfig, defaultIdentityProviderUris())

	assert.Equal(t, []string{"offline_access"}, stringSlice(t, inner["auth_scopes"]))
}

func TestGetOAuth2FilterConfig_Scopes_CustomScopesPreserved(t *testing.T) {
	autoLoginConfig := defaultAutoLoginConfig()
	autoLoginConfig.Scopes = []string{"openid", "profile", "email"}

	inner := oauthInnerConfig(t, autoLoginConfig, defaultIdentityProviderUris())

	assert.Equal(t, []string{"openid", "profile", "email"}, stringSlice(t, inner["auth_scopes"]))
}

func TestGetOAuth2FilterConfig_Resources_PresentWhenSet(t *testing.T) {
	autoLoginConfig := defaultAutoLoginConfig()
	autoLoginConfig.ResourceIndicators = []string{
		"https://example.com/api-1",
		"https://example.com/api-2",
	}

	inner := oauthInnerConfig(t, autoLoginConfig, defaultIdentityProviderUris())

	_, present := inner["resources"]
	require.True(t, present, "resources must be present when AcceptedResources is set")
	assert.Equal(t,
		[]string{"https://example.com/api-1", "https://example.com/api-2"},
		stringSlice(t, inner["resources"]),
	)
}

func TestGetOAuth2FilterConfig_Resources_AbsentWhenNil(t *testing.T) {
	inner := oauthInnerConfig(t, defaultAutoLoginConfig(), defaultIdentityProviderUris())

	_, present := inner["resources"]
	assert.False(t, present, "resources must be absent when AcceptedResources is nil")
}

func TestGetOAuth2FilterConfig_PassThroughAndDenyRedirectMatchers(t *testing.T) {
	inner := oauthInnerConfig(t, defaultAutoLoginConfig(), defaultIdentityProviderUris())

	ptm, ok := inner["pass_through_matcher"].([]any)
	require.True(t, ok, "pass_through_matcher not found or wrong type")
	require.Len(t, ptm, 2)
	authHeader := ptm[0].(map[string]any)
	assert.Equal(t, "authorization", authHeader["name"])
	bypassHeader := ptm[1].(map[string]any)
	assert.Equal(t, luascript.OAuth2FilterPassThroughHeaderName, bypassHeader["name"])

	drm, ok := inner["deny_redirect_matcher"].([]any)
	require.True(t, ok, "deny_redirect_matcher not found or wrong type")
	require.Len(t, drm, 1)
	denyHeader := drm[0].(map[string]any)
	assert.Equal(t, luascript.OAuth2FilterDenyRedirectHeaderName, denyHeader["name"])
}

func TestGetOAuth2FilterConfig_CookieConfigs_SameSiteLax(t *testing.T) {
	inner := oauthInnerConfig(t, defaultAutoLoginConfig(), defaultIdentityProviderUris())

	cookieConfigs, ok := inner["cookie_configs"].(map[string]any)
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
		cfg, present := cookieConfigs[key].(map[string]any)
		require.Truef(t, present, "%s must be present in cookie_configs", key)
		assert.Equalf(t, "LAX", cfg["same_site"], "%s.same_site must be LAX", key)
	}
}

func TestGetOAuth2FilterConfig_ReturnsErrorOnInvalidConfig(t *testing.T) {
	result, err := configpatch.GetOAuth2ConfigPatchValue(
		defaultTokenEndpointClusterName,
		defaultAutoLoginConfig(),
		defaultIdentityProviderUris(),
		"",
	)

	require.Error(t, err)
	assert.Nil(t, result)
}

// toMap asserts that the config patch value was generated without error and returns it as a plain map.
func toMap(t *testing.T, value *structpb.Struct, err error) map[string]any {
	t.Helper()
	require.NoError(t, err)
	require.NotNil(t, value)
	return value.AsMap()
}

// oauthInnerConfig generates the OAuth2 config patch value and returns typed_config.config of the HTTP filter.
func oauthInnerConfig(
	t *testing.T,
	autoLoginConfig model.AutoLoginConfig,
	identityProviderUris model.IdentityProviderUris,
) map[string]any {
	t.Helper()
	result, err := configpatch.GetOAuth2ConfigPatchValue(
		defaultTokenEndpointClusterName,
		autoLoginConfig,
		identityProviderUris,
		defaultClientID,
	)
	patch := toMap(t, result, err)

	typed, ok := patch["typed_config"].(map[string]any)
	require.True(t, ok, "typed_config not found or wrong type")
	cfg, ok := typed["config"].(map[string]any)
	require.True(t, ok, "config not found or wrong type")
	return cfg
}

func stringSlice(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]any)
	require.True(t, ok, "expected a list, got %T", value)
	result := make([]string, 0, len(items))
	for _, item := range items {
		s, isString := item.(string)
		require.True(t, isString, "expected string list item, got %T", item)
		result = append(result, s)
	}
	return result
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
