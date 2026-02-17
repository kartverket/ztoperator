package resolver_test

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveOAuthCredentials_WithAutoLoginDisabled_ReturnsEmptyCredentials(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createAuthPolicyWithOAuth("default", "oauth-secret", "client-id", "client-secret", false)
	k8sClient := createFakeClientForOauthCredentials()

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, authPolicy)

	// 3. Assert
	require.NoError(t, err, "ResolveOAuthCredentials should not return an error when auto-login is disabled")
	require.NotNil(t, result, "Result should not be nil")
	assert.Nil(t, result.ClientID, "ClientID should be nil when auto-login is disabled")
	assert.Nil(t, result.ClientSecret, "ClientSecret should be nil when auto-login is disabled")
}

func TestResolveOAuthCredentials_WithNoOAuthCredentialsSpec_ReturnsEmptyCredentials(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createAuthPolicyWithoutOAuth("default", true)
	k8sClient := createFakeClientForOauthCredentials()

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, authPolicy)

	// 3. Assert
	require.NoError(t, err, "ResolveOAuthCredentials should not return an error when OAuthCredentials is nil")
	require.NotNil(t, result, "Result should not be nil")
	assert.Nil(t, result.ClientID, "ClientID should be nil when OAuthCredentials is not specified")
	assert.Nil(t, result.ClientSecret, "ClientSecret should be nil when OAuthCredentials is not specified")
}

func TestResolveOAuthCredentials_WithMissingSecret_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createAuthPolicyWithOAuth("default", "non-existent-secret", "client-id", "client-secret", true)
	k8sClient := createFakeClientForOauthCredentials()

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, authPolicy)

	// 3. Assert
	require.Error(t, err, "ResolveOAuthCredentials should return an error when secret is missing")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "failed to get OAuth credentials secret")
	assert.Contains(t, err.Error(), "default/non-existent-secret")
}

func TestResolveOAuthCredentials_WithEmptyClientID_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oauth-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"client-id":     []byte(""), // Empty client ID
			"client-secret": []byte("my-client-secret"),
		},
	}

	authPolicy := createAuthPolicyWithOAuth("default", "oauth-secret", "client-id", "client-secret", true)
	k8sClient := createFakeClientForOauthCredentials(secret)

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, authPolicy)

	// 3. Assert
	require.Error(t, err, "ResolveOAuthCredentials should return an error when client ID is empty")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "client id with key: client-id was nil or empty")
	assert.Contains(t, err.Error(), "default/oauth-secret")
}

func TestResolveOAuthCredentials_WithEmptyClientSecret_ReturnsError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oauth-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"client-id":     []byte("my-client-id"),
			"client-secret": []byte(""), // Empty client secret
		},
	}

	authPolicy := createAuthPolicyWithOAuth("default", "oauth-secret", "client-id", "client-secret", true)
	k8sClient := createFakeClientForOauthCredentials(secret)

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, authPolicy)

	// 3. Assert
	require.Error(t, err, "ResolveOAuthCredentials should return an error when client secret is empty")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "client secret with key: client-secret was nil or empty")
	assert.Contains(t, err.Error(), "default/oauth-secret")
}

func TestResolveOAuthCredentials_WithNilAutoLogin_ReturnsEmptyCredentials(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := createAuthPolicyWithNilAutoLogin("default", "oauth-secret", "client-id", "client-secret")
	k8sClient := createFakeClientForOauthCredentials()

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, authPolicy)

	// 3. Assert
	require.NoError(t, err, "ResolveOAuthCredentials should not return an error when AutoLogin is nil")
	require.NotNil(t, result, "Result should not be nil")
	assert.Nil(t, result.ClientID, "ClientID should be nil when AutoLogin is not configured")
	assert.Nil(t, result.ClientSecret, "ClientSecret should be nil when AutoLogin is not configured")
}

func TestResolveOAuthCredentials_WithValidSecret_ReturnsCredentials(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	expectedClientID := "my-client-id"
	expectedClientSecret := "my-client-secret"

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oauth-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"client-id":     []byte(expectedClientID),
			"client-secret": []byte(expectedClientSecret),
		},
	}

	authPolicy := createAuthPolicyWithOAuth("default", "oauth-secret", "client-id", "client-secret", true)
	k8sClient := createFakeClientForOauthCredentials(secret)

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, authPolicy)

	// 3. Assert
	require.NoError(t, err, "ResolveOAuthCredentials should not return an error with valid secret")
	require.NotNil(t, result, "Result should not be nil")
	require.NotNil(t, result.ClientID, "ClientID should not be nil")
	require.NotNil(t, result.ClientSecret, "ClientSecret should not be nil")
	assert.Equal(t, expectedClientID, *result.ClientID, "ClientID should match expected value")
	assert.Equal(t, expectedClientSecret, *result.ClientSecret, "ClientSecret should match expected value")
}

func TestResolveOAuthCredentials_WithCustomKeys_ReturnsCredentials(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	expectedClientID := "custom-client-id"
	expectedClientSecret := "custom-client-secret"

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oauth-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"custom-id-key":     []byte(expectedClientID),
			"custom-secret-key": []byte(expectedClientSecret),
		},
	}

	authPolicy := createAuthPolicyWithOAuth(
		"test-namespace",
		"oauth-secret",
		"custom-id-key",
		"custom-secret-key",
		true,
	)
	k8sClient := createFakeClientForOauthCredentials(secret)

	// 2. Act
	result, err := resolver.ResolveOAuthCredentials(ctx, k8sClient, authPolicy)

	// 3. Assert
	require.NoError(t, err, "ResolveOAuthCredentials should not return an error with custom keys")
	require.NotNil(t, result, "Result should not be nil")
	require.NotNil(t, result.ClientID, "ClientID should not be nil")
	require.NotNil(t, result.ClientSecret, "ClientSecret should not be nil")
	assert.Equal(t, expectedClientID, *result.ClientID, "ClientID should match expected value")
	assert.Equal(t, expectedClientSecret, *result.ClientSecret, "ClientSecret should match expected value")
}

func createAuthPolicyWithOAuth(
	namespace, secretRef, clientIDKey, clientSecretKey string,
	autoLoginEnabled bool,
) *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: namespace,
		},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			WellKnownURI: "http://test-idp.example.com/.well-known/openid-configuration",
			AutoLogin: &ztoperatorv1alpha1.AutoLogin{
				Enabled: autoLoginEnabled,
			},
			OAuthCredentials: &ztoperatorv1alpha1.OAuthCredentials{
				SecretRef:       secretRef,
				ClientIDKey:     clientIDKey,
				ClientSecretKey: clientSecretKey,
			},
		},
	}
}

func createAuthPolicyWithoutOAuth(namespace string, autoLoginEnabled bool) *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: namespace,
		},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			WellKnownURI: "http://test-idp.example.com/.well-known/openid-configuration",
			AutoLogin: &ztoperatorv1alpha1.AutoLogin{
				Enabled: autoLoginEnabled,
			},
			OAuthCredentials: nil,
		},
	}
}

func createAuthPolicyWithNilAutoLogin(
	namespace, secretRef, clientIDKey, clientSecretKey string,
) *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: namespace,
		},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			WellKnownURI: "http://test-idp.example.com/.well-known/openid-configuration",
			AutoLogin:    nil,
			OAuthCredentials: &ztoperatorv1alpha1.OAuthCredentials{
				SecretRef:       secretRef,
				ClientIDKey:     clientIDKey,
				ClientSecretKey: clientSecretKey,
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
