package configpatch_test

import (
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetInternalOAuthClusterConfigPatch_NoTLSTransportSocket(t *testing.T) {
	result := configpatch.GetInternalOAuthClusterConfigPatchValue("mock-oauth2.auth", 8080)

	assert.Nil(t, result["transport_socket"], "internal cluster should not have TLS transport socket")
}

func TestGetExternalOAuthClusterPatch_HasTLSTransportSocket(t *testing.T) {
	result := configpatch.GetExternalOAuthClusterPatchValue("login.microsoftonline.com")

	ts, ok := result["transport_socket"].(map[string]interface{})
	require.True(t, ok, "external cluster must have transport_socket")
	assert.Equal(t, "envoy.transport_sockets.tls", ts["name"])

	typed := ts["typed_config"].(map[string]interface{})
	assert.Equal(t, "login.microsoftonline.com", typed["sni"])
}

func TestGetOAuthSidecarConfigPatch_EndSessionEndpoint_PresentWhenSet(t *testing.T) {
	scope := defaultScope()

	result := configpatch.GetOAuthSidecarConfigPatchValue(scope)

	inner := oauthInnerConfig(t, result)
	assert.Equal(t, "https://idp.example.com/endsession", inner["end_session_endpoint"])
}

func TestGetOAuthSidecarConfigPatch_EndSessionEndpoint_AbsentWhenNil(t *testing.T) {
	scope := defaultScope()
	scope.IdentityProviderUris.EndSessionURI = nil

	result := configpatch.GetOAuthSidecarConfigPatchValue(scope)

	inner := oauthInnerConfig(t, result)
	_, present := inner["end_session_endpoint"]
	assert.False(t, present, "end_session_endpoint must be absent when EndSessionURI is nil")
}

func TestGetOAuthSidecarConfigPatch_Scopes_OpenIDAlwaysPresent(t *testing.T) {
	scope := defaultScope()
	scope.AutoLoginConfig.Scopes = []string{"offline_access"} // openid deliberately omitted

	result := configpatch.GetOAuthSidecarConfigPatchValue(scope)

	inner := oauthInnerConfig(t, result)
	scopes := inner["auth_scopes"].([]interface{})
	scopeStrs := make([]string, 0, len(scopes))
	for _, s := range scopes {
		scopeStrs = append(scopeStrs, s.(string))
	}
	assert.Contains(t, scopeStrs, "openid")
}

func TestGetOAuthSidecarConfigPatch_Scopes_CustomScopesPreserved(t *testing.T) {
	scope := defaultScope()
	scope.AutoLoginConfig.Scopes = []string{"openid", "profile", "email"}

	result := configpatch.GetOAuthSidecarConfigPatchValue(scope)

	inner := oauthInnerConfig(t, result)
	scopes := inner["auth_scopes"].([]interface{})
	scopeStrs := make([]string, 0, len(scopes))
	for _, s := range scopes {
		scopeStrs = append(scopeStrs, s.(string))
	}
	assert.Equal(t, []string{"openid", "profile", "email"}, scopeStrs)
}

func TestGetOAuthSidecarConfigPatch_Resources_PresentWhenSet(t *testing.T) {
	scope := defaultScope()
	scope.AuthPolicy.Spec.AcceptedResources = &[]string{
		"https://example.com/api-1",
		"https://example.com/api-2",
	}

	result := configpatch.GetOAuthSidecarConfigPatchValue(scope)

	inner := oauthInnerConfig(t, result)
	resources, ok := inner["resources"].([]interface{})
	require.True(t, ok, "resources must be present when AcceptedResources is set")
	require.Len(t, resources, 2)
	assert.Equal(t, "https://example.com/api-1", resources[0])
	assert.Equal(t, "https://example.com/api-2", resources[1])
}

func TestGetOAuthSidecarConfigPatch_Resources_AbsentWhenNil(t *testing.T) {
	scope := defaultScope()

	result := configpatch.GetOAuthSidecarConfigPatchValue(scope)

	inner := oauthInnerConfig(t, result)
	_, present := inner["resources"]
	assert.False(t, present, "resources must be absent when AcceptedResources is nil")
}

func TestGetOAuthSidecarConfigPatch_PassThroughAndDenyRedirectMatchers(t *testing.T) {
	scope := defaultScope()

	result := configpatch.GetOAuthSidecarConfigPatchValue(scope)

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

func oauthInnerConfig(t *testing.T, patch map[string]interface{}) map[string]interface{} {
	t.Helper()
	typed, ok := patch["typed_config"].(map[string]interface{})
	require.True(t, ok, "typed_config not found or wrong type")
	cfg, ok := typed["config"].(map[string]interface{})
	require.True(t, ok, "config not found or wrong type")
	return cfg
}

func defaultScope() state.Scope {
	clientID := "my-client"
	endSession := "https://idp.example.com/endsession"
	return state.Scope{
		AuthPolicy: ztoperatorv1alpha1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "auth-policy", Namespace: "default"},
			Spec: ztoperatorv1alpha1.AuthPolicySpec{
				Enabled: true,
				Selector: ztoperatorv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{"app": "myapp"},
				},
			},
		},
		OAuthCredentials: state.OAuthCredentials{
			ClientID: &clientID,
		},
		IdentityProviderUris: state.IdentityProviderUris{
			TokenURI:         "http://mock-oauth2.auth:8080/entraid/token",
			AuthorizationURI: "http://mock-oauth2.auth:8080/entraid/authorize",
			EndSessionURI:    &endSession,
		},
		AutoLoginConfig: state.AutoLoginConfig{
			Enabled:      true,
			RedirectPath: "/oauth2/callback",
			LogoutPath:   "/logout",
			Scopes:       []string{"openid"},
		},
	}
}
