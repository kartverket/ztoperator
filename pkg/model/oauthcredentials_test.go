package model_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
	"time"

	oauth2v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/oauth2/v3"
	"github.com/go-jose/go-jose/v4"
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAssertionAudience = "http://test-idp.example.com"
	testSecretRef         = "oauth-secret"
	testClientIdKey       = "client-id"
)

func TestToOAuthCredentials_WithEmptyClientID_ReturnsError(t *testing.T) {
	// 1. Arrange
	authPolicy := createOAuthAuthPolicy("client-secret", "")
	secretData := map[string][]byte{
		testClientIdKey: []byte(""), // Empty client ID
		"client-secret": []byte("my-client-secret"),
	}

	// 2. Act
	result, err := model.ToOAuthCredentials(*authPolicy, secretData, testAssertionAudience)

	// 3. Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "client id with key: client-id was nil or empty")
	assert.Contains(t, err.Error(), "default/oauth-secret")
}

func TestToOAuthCredentials_WithMissingClientIDKey_ReturnsError(t *testing.T) {
	// 1. Arrange
	authPolicy := createOAuthAuthPolicy("client-secret", "")
	secretData := map[string][]byte{
		// client-id key is missing
		"client-secret": []byte("my-client-secret"),
	}

	// 2. Act
	result, err := model.ToOAuthCredentials(*authPolicy, secretData, testAssertionAudience)

	// 3. Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "client id with key: client-id was nil or empty")
	assert.Contains(t, err.Error(), "default/oauth-secret")
}

func TestToOAuthCredentials_WithValidClientSecret_ReturnsCredentials(t *testing.T) {
	// 1. Arrange
	expectedClientID := "my-client-id"
	expectedClientSecret := "my-client-secret"
	authPolicy := createOAuthAuthPolicy("client-secret", "")
	secretData := map[string][]byte{
		testClientIdKey: []byte(expectedClientID),
		"client-secret": []byte(expectedClientSecret),
	}

	// 2. Act
	result, err := model.ToOAuthCredentials(*authPolicy, secretData, testAssertionAudience)

	// 3. Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ClientID)
	assert.Equal(t, expectedClientID, *result.ClientID)
	assert.Equal(t, oauth2v3.OAuth2Config_URL_ENCODED_BODY, result.TokenSecret.AuthType)
	require.NotNil(t, result.TokenSecret.Value)
	assert.Equal(t, "token", result.TokenSecret.Value.GetName())
	assert.Equal(
		t,
		expectedClientSecret,
		result.TokenSecret.Value.GetGenericSecret().GetSecret().GetInlineString(),
	)
	require.NotNil(t, result.HmacSecret.Value)
	assert.Equal(t, "hmac", result.HmacSecret.Value.GetName())
	assert.NotEmpty(t, result.HmacSecret.Value.GetGenericSecret().GetSecret().GetInlineBytes())
}

func TestToOAuthCredentials_WithEmptyClientSecret_ReturnsError(t *testing.T) {
	// 1. Arrange
	authPolicy := createOAuthAuthPolicy("client-secret", "")
	secretData := map[string][]byte{
		testClientIdKey: []byte("my-client-id"),
		"client-secret": []byte(""), // Empty client secret
	}

	// 2. Act
	result, err := model.ToOAuthCredentials(*authPolicy, secretData, testAssertionAudience)

	// 3. Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get token secret from OAuth credentials secret default/oauth-secret")
	assert.Contains(t, err.Error(), "client secret was nil or empty")
}

func TestToOAuthCredentials_WithPrivateJWK_ReturnsPEMAndKeyID(t *testing.T) {
	// 1. Arrange
	authPolicy := createOAuthAuthPolicy("", "private-jwk")
	secretData := map[string][]byte{
		testClientIdKey: []byte("my-client-id"),
		"private-jwk":   newECPrivateJWK(t, "my-kid", "ES256"),
	}

	// 2. Act
	result, err := model.ToOAuthCredentials(*authPolicy, secretData, testAssertionAudience)

	// 3. Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ClientID)
	assert.Equal(t, "my-client-id", *result.ClientID)
	assert.Equal(t, oauth2v3.OAuth2Config_PRIVATE_KEY_JWT, result.TokenSecret.AuthType)
	require.NotNil(t, result.TokenSecret.Value)
	assert.Equal(t, "token", result.TokenSecret.Value.GetName())
	generic := result.TokenSecret.Value.GetGenericSecret()
	assert.Nil(t, generic.GetSecret(), "multi-entry form should be used when kid is present")
	assert.Equal(t, "my-kid", generic.GetSecrets()["key_id"].GetInlineString())
	assert.Contains(t, generic.GetSecrets()["private_key"].GetInlineString(), "BEGIN PRIVATE KEY")

	jwtConfig := result.TokenSecret.PrivateKeyJwtConfig
	require.NotNil(t, jwtConfig)
	assert.Equal(t, oauth2v3.PrivateKeyJwtConfig_ES256, jwtConfig.GetSigningAlgorithm())
	assert.Equal(t, 60*time.Second, jwtConfig.GetAssertionLifetime().AsDuration())
	assert.Equal(t, testAssertionAudience, jwtConfig.GetAssertionAudience())
}

func TestGetTokenSecret_WithPrivateJWKWithoutKeyID_ReturnsSingleValueSecret(t *testing.T) {
	// 1. Arrange
	secretData := map[string][]byte{"private-jwk": newECPrivateJWK(t, "", "ES256")}

	// 2. Act
	result, err := model.GetTokenSecret(secretData, testAssertionAudience, "", "private-jwk")

	// 3. Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	generic := result.Value.GetGenericSecret()
	assert.Empty(t, generic.GetSecrets())
	assert.Contains(t, generic.GetSecret().GetInlineString(), "BEGIN PRIVATE KEY")
	require.NotNil(t, result.PrivateKeyJwtConfig)
	assert.Equal(t, testAssertionAudience, result.PrivateKeyJwtConfig.GetAssertionAudience())
}

func TestGetTokenSecret_WithPrivateJWK_MapsSigningAlgorithm(t *testing.T) {
	tests := []struct {
		alg      string
		key      any
		expected oauth2v3.PrivateKeyJwtConfig_SigningAlgorithm
	}{
		{alg: "RS256", key: newRSAKey(t), expected: oauth2v3.PrivateKeyJwtConfig_RS256},
		{alg: "RS384", key: newRSAKey(t), expected: oauth2v3.PrivateKeyJwtConfig_RS384},
		{alg: "RS512", key: newRSAKey(t), expected: oauth2v3.PrivateKeyJwtConfig_RS512},
		{alg: "ES256", key: newECKey(t, elliptic.P256()), expected: oauth2v3.PrivateKeyJwtConfig_ES256},
		{alg: "ES384", key: newECKey(t, elliptic.P384()), expected: oauth2v3.PrivateKeyJwtConfig_ES384},
		{alg: "ES512", key: newECKey(t, elliptic.P521()), expected: oauth2v3.PrivateKeyJwtConfig_ES512},
	}

	for _, tc := range tests {
		t.Run(tc.alg, func(t *testing.T) {
			// 1. Arrange
			secretData := map[string][]byte{"private-jwk": marshalPrivateJWK(t, tc.key, "my-kid", tc.alg)}

			// 2. Act
			result, err := model.GetTokenSecret(secretData, testAssertionAudience, "", "private-jwk")

			// 3. Assert
			require.NoError(t, err)
			require.NotNil(t, result.PrivateKeyJwtConfig)
			assert.Equal(t, tc.expected, result.PrivateKeyJwtConfig.GetSigningAlgorithm())
		})
	}
}

func TestGetTokenSecret_WithPrivateJWKWithoutAlgorithm_ReturnsError(t *testing.T) {
	// 1. Arrange
	secretData := map[string][]byte{"private-jwk": newECPrivateJWK(t, "my-kid", "")}

	// 2. Act
	result, err := model.GetTokenSecret(secretData, testAssertionAudience, "", "private-jwk")

	// 3. Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), `unsupported signing algorithm ""`)
}

func TestGetTokenSecret_WithPrivateJWKWithUnsupportedAlgorithm_ReturnsError(t *testing.T) {
	// 1. Arrange
	secretData := map[string][]byte{"private-jwk": newECPrivateJWK(t, "my-kid", "PS256")}

	// 2. Act
	result, err := model.GetTokenSecret(secretData, testAssertionAudience, "", "private-jwk")

	// 3. Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), `unsupported signing algorithm "PS256"`)
}

func TestGetTokenSecret_WithEmptyPrivateJWK_ReturnsError(t *testing.T) {
	// 1. Arrange
	secretData := map[string][]byte{"private-jwk": []byte("")}

	// 2. Act
	result, err := model.GetTokenSecret(secretData, testAssertionAudience, "", "private-jwk")

	// 3. Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "private JWK was nil or empty")
}

func TestGetTokenSecret_WithInvalidPrivateJWK_ReturnsError(t *testing.T) {
	// 1. Arrange
	secretData := map[string][]byte{"private-jwk": []byte("not-a-jwk")}

	// 2. Act
	result, err := model.GetTokenSecret(secretData, testAssertionAudience, "", "private-jwk")

	// 3. Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse private JWK")
}

func TestGetTokenSecret_WithPublicJWK_ReturnsError(t *testing.T) {
	// 1. Arrange
	key := newECKey(t, elliptic.P256())
	secretData := map[string][]byte{
		"private-jwk": marshalPrivateJWK(t, &key.PublicKey, "my-kid", "ES256"),
	}

	// 2. Act
	result, err := model.GetTokenSecret(secretData, testAssertionAudience, "", "private-jwk")

	// 3. Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse private JWK")
}

func createOAuthAuthPolicy(
	clientSecretKey,
	privateJwkKey string,
) *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		Name:      "test-policy",
		Namespace: "default",
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			OAuthCredentials: &ztoperatorv1alpha1.OAuthCredentials{
				SecretRef:       testSecretRef,
				ClientIDKey:     testClientIdKey,
				ClientSecretKey: clientSecretKey,
				PrivateJwkKey:   privateJwkKey,
			},
		},
	}
}

func newECKey(t *testing.T, curve elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	return key
}

func newRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func marshalPrivateJWK(t *testing.T, key any, kid, alg string) []byte {
	t.Helper()
	rawJWK, err := json.Marshal(jose.JSONWebKey{Key: key, KeyID: kid, Algorithm: alg, Use: "sig"})
	require.NoError(t, err)
	return rawJWK
}

func newECPrivateJWK(t *testing.T, kid, alg string) []byte {
	t.Helper()
	return marshalPrivateJWK(t, newECKey(t, elliptic.P256()), kid, alg)
}
