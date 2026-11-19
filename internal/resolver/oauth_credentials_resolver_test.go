package resolver_test

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/resolver"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveOAuthCredentials_WithAutoLoginDisabled_ReturnsEmptyCredentials(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createAuthPolicyWithOAuth("oauth-secret", false)
	k8sClient := createFakeClientForOauthCredentials()

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, *authPolicy)

	// 3. Assert
	require.NoError(t, err, "ResolveOAuthCredentials should not return an error when auto-login is disabled")
	require.NotNil(t, result, "Result should not be nil")
	assert.Nil(t, result.ClientID, "ClientID should be nil when auto-login is disabled")
	assert.Nil(t, result.TokenSecret.Value, "TokenSecret should be nil when auto-login is disabled")
}

func TestResolveOAuthCredentials_WithNoOAuthCredentialsSpec_ReturnsEmptyCredentials(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createAuthPolicyWithoutOAuth("default", true)
	k8sClient := createFakeClientForOauthCredentials()

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, *authPolicy)

	// 3. Assert
	require.NoError(t, err, "ResolveOAuthCredentials should not return an error when OAuthCredentials is nil")
	require.NotNil(t, result, "Result should not be nil")
	assert.Nil(t, result.ClientID, "ClientID should be nil when OAuthCredentials is not specified")
	assert.Nil(t, result.TokenSecret.Value, "TokenSecret should be nil when OAuthCredentials is not specified")
}

func TestResolveOAuthCredentials_WithNilAutoLogin_ReturnsEmptyCredentials(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createAuthPolicyWithNilAutoLogin("default", "oauth-secret")
	k8sClient := createFakeClientForOauthCredentials()

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, *authPolicy)

	// 3. Assert
	require.NoError(t, err, "ResolveOAuthCredentials should not return an error when AutoLogin is nil")
	require.NotNil(t, result, "Result should not be nil")
	assert.Nil(t, result.ClientID, "ClientID should be nil when AutoLogin is not configured")
	assert.Nil(t, result.TokenSecret.Value, "TokenSecret should be nil when AutoLogin is not configured")
}

func TestResolveOAuthCredentials_WithMissingSecret_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createAuthPolicyWithOAuth("non-existent-secret", true)
	k8sClient := createFakeClientForOauthCredentials()

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, *authPolicy)

	// 3. Assert
	require.Error(t, err, "ResolveOAuthCredentials should return an error when secret is missing")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "failed to get OAuth credentials secret")
	assert.Contains(t, err.Error(), "default/non-existent-secret")
}

func TestResolveOAuthCredentials_WithUnknownWellKnownURI_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	configureOAuthCredentialsTestProvider(t)
	secret := &v1.Secret{
		Name: "oauth-secret", Namespace: "default",
		Data: map[string][]byte{
			"client-id":     []byte("my-client-id"),
			"client-secret": []byte("my-client-secret"),
		},
	}
	authPolicy := createAuthPolicyWithOAuth("oauth-secret", true)
	authPolicy.Spec.WellKnownURI = "http://unknown-idp.example.com/.well-known/openid-configuration"
	k8sClient := createFakeClientForOauthCredentials(secret)

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, *authPolicy)

	// 3. Assert
	require.Error(t, err, "ResolveOAuthCredentials should return an error when wellKnownURI is not in the allowlist")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), `wellKnownURI "http://unknown-idp.example.com/.well-known/openid-configuration" is not in the configured allowlist`)
}

func TestResolveOAuthCredentials_WithValidSecret_ReturnsCredentials(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	configureOAuthCredentialsTestProvider(t)
	expectedClientID := "my-client-id"
	expectedClientSecret := "my-client-secret"

	secret := &v1.Secret{
		Name:      "oauth-secret",
		Namespace: "default",
		Data: map[string][]byte{
			"client-id":     []byte(expectedClientID),
			"client-secret": []byte(expectedClientSecret),
		},
	}

	authPolicy := createAuthPolicyWithOAuth("oauth-secret", true)
	k8sClient := createFakeClientForOauthCredentials(secret)

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, *authPolicy)

	// 3. Assert
	require.NoError(t, err, "ResolveOAuthCredentials should not return an error with valid secret")
	require.NotNil(t, result, "Result should not be nil")
	require.NotNil(t, result.ClientID, "ClientID should not be nil")
	assert.Equal(t, expectedClientID, *result.ClientID, "ClientID should match expected value")
	require.NotNil(t, result.TokenSecret.Value, "TokenSecret value should not be nil")
	assert.Equal(t, expectedClientSecret, result.TokenSecret.Value.GetGenericSecret().GetSecret().GetInlineString(), "ClientSecret should match expected value")
	require.NotNil(t, result.HmacSecret.Value, "HmacSecret value should not be nil")
	assert.NotEmpty(t, result.HmacSecret.Value.GetGenericSecret().GetSecret().GetInlineBytes())
}

const testWellKnownURI = "http://test-idp.example.com/.well-known/openid-configuration"

func configureOAuthCredentialsTestProvider(t *testing.T) {
	t.Helper()
	t.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{
		"test-idp": {
			"wellKnownURI": "`+testWellKnownURI+`",
			"issuer": "http://test-idp.example.com",
			"tokenEndpoint": "http://test-idp.example.com/token",
			"jwksURI": "http://test-idp.example.com/jwks",
			"privateKeyJwtAssertionAudience": "http://test-idp.example.com"
		}
	}`)
	require.NoError(t, config.Load())
}

func createAuthPolicyWithOAuth(
	secretRef string,
	autoLoginEnabled bool,
) *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		Name:      "test-policy",
		Namespace: "default",
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			WellKnownURI: testWellKnownURI,
			AutoLogin: &ztoperatorv1alpha1.AutoLogin{
				Enabled: autoLoginEnabled,
			},
			OAuthCredentials: &ztoperatorv1alpha1.OAuthCredentials{
				SecretRef:       secretRef,
				ClientIDKey:     "client-id",
				ClientSecretKey: "client-secret",
			},
		},
	}
}

func createAuthPolicyWithoutOAuth(namespace string, autoLoginEnabled bool) *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		Name:      "test-policy",
		Namespace: namespace,
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			WellKnownURI: testWellKnownURI,
			AutoLogin: &ztoperatorv1alpha1.AutoLogin{
				Enabled: autoLoginEnabled,
			},
			OAuthCredentials: nil,
		},
	}
}

func createAuthPolicyWithNilAutoLogin(
	namespace, secretRef string,
) *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		Name:      "test-policy",
		Namespace: namespace,
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			WellKnownURI: testWellKnownURI,
			AutoLogin:    nil,
			OAuthCredentials: &ztoperatorv1alpha1.OAuthCredentials{
				SecretRef:       secretRef,
				ClientIDKey:     "client-id",
				ClientSecretKey: "client-secret",
			},
		},
	}
}

func createFakeClientForOauthCredentials(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = ztoperatorv1alpha1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}
