package resolver_test

import (
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/resolver"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolveAutoLoginConfig_WithAutoLoginDisabled_ReturnsDisabledConfig(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy("test-policy", &ztoperatorv1alpha1.AutoLogin{Enabled: false})
	identityProviderUris := createTestIdentityProviderUris()

	// 2. Act
	result := resolver.ResolveAutoLoginConfig(authPolicy, identityProviderUris)

	// 3. Assert
	assert.False(t, result.Enabled, "AutoLogin should be disabled")
	assert.Empty(t, result.EnvoySecretName, "EnvoySecretName should be empty when disabled")
	assert.Empty(t, result.LuaScriptConfig.LuaScript, "LuaScript should be empty when disabled")
}

func TestResolveAutoLoginConfig_WithAutoLoginNil_ReturnsDisabledConfig(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy("test-policy", nil)
	identityProviderUris := createTestIdentityProviderUris()

	// 2. Act
	result := resolver.ResolveAutoLoginConfig(authPolicy, identityProviderUris)

	// 3. Assert
	assert.False(t, result.Enabled, "AutoLogin should be disabled when nil")
}

func TestResolveAutoLoginConfig_WithBasicAutoLogin_ReturnsConfigWithDefaults(t *testing.T) {
	// 1. Arrange
	authPolicy := createTestAuthPolicy("test-policy", &ztoperatorv1alpha1.AutoLogin{Enabled: true})
	identityProviderUris := createTestIdentityProviderUris()

	// 2. Act
	result := resolver.ResolveAutoLoginConfig(authPolicy, identityProviderUris)

	// 3. Assert
	assert.True(t, result.Enabled, "AutoLogin should be enabled")
	assert.NotEmpty(t, result.RedirectPath, "RedirectPath should have default value")
	assert.NotEmpty(t, result.LogoutPath, "LogoutPath should have default value")
	assert.NotEmpty(t, result.LuaScriptConfig.LuaScript, "LuaScript should be generated")
	assert.Equal(t, "test-policy-envoy-secret", result.EnvoySecretName, "EnvoySecretName should be set")
}

func TestResolveAutoLoginConfig_WithCustomConfiguration_PreservesAllValues(t *testing.T) {
	// 1. Arrange
	customLoginPath := "/custom-login"
	customPostLogoutURI := "https://example.com/logged-out"
	customScopes := []string{"openid", "profile", "email"}
	customLoginParams := map[string]string{
		"prompt": "consent",
		"acr":    "Level4",
	}

	authPolicy := createTestAuthPolicy("my-policy", &ztoperatorv1alpha1.AutoLogin{
		Enabled:               true,
		LoginPath:             &customLoginPath,
		PostLogoutRedirectURI: &customPostLogoutURI,
		Scopes:                customScopes,
		LoginParams:           customLoginParams,
	})
	identityProviderUris := createTestIdentityProviderUris()

	// 2. Act
	result := resolver.ResolveAutoLoginConfig(authPolicy, identityProviderUris)

	// 3. Assert
	assert.True(t, result.Enabled, "AutoLogin should be enabled")

	// Verify custom values are preserved
	require.NotNil(t, result.LoginPath, "LoginPath should not be nil")
	assert.Equal(t, customLoginPath, *result.LoginPath, "Custom LoginPath should be preserved")
	require.NotNil(t, result.PostLogoutRedirectURI, "PostLogoutRedirectURI should not be nil")
	assert.Equal(
		t,
		customPostLogoutURI,
		*result.PostLogoutRedirectURI,
		"Custom PostLogoutRedirectURI should be preserved",
	)
	assert.Equal(t, customScopes, result.Scopes, "Custom scopes should be preserved")
	assert.Equal(t, customLoginParams, result.LoginParams, "Custom login params should be preserved")

	// Verify generated values
	assert.Equal(t, "my-policy-envoy-secret", result.EnvoySecretName, "EnvoySecretName should be set")
	assert.NotEmpty(t, result.LuaScriptConfig.LuaScript, "LuaScript should be generated")
	assert.Contains(
		t,
		result.LuaScriptConfig.LuaScript,
		identityProviderUris.AuthorizationURI,
		"Lua script should contain authorization URI",
	)
	assert.Contains(
		t,
		result.LuaScriptConfig.LuaScript,
		*identityProviderUris.EndSessionURI,
		"Lua script should contain end session URI",
	)
}

func createTestAuthPolicy(name string, autoLogin *ztoperatorv1alpha1.AutoLogin) *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			WellKnownURI: "http://test-idp.example.com/.well-known/openid-configuration",
			AutoLogin:    autoLogin,
		},
	}
}

func createTestIdentityProviderUris() state.IdentityProviderUris {
	return state.IdentityProviderUris{
		IssuerURI:        "http://test-idp.example.com",
		JwksURI:          "http://test-idp.example.com/jwks",
		TokenURI:         "http://test-idp.example.com/token",
		AuthorizationURI: "http://test-idp.example.com/authorize",
		EndSessionURI:    helperfunctions.Ptr("http://test-idp.example.com/logout"),
	}
}
