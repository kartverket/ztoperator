package crypto_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/kartverket/ztoperator/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func marshalJWK(t *testing.T, key any, kid string) []byte {
	t.Helper()
	raw, err := json.Marshal(jose.JSONWebKey{Key: key, KeyID: kid})
	require.NoError(t, err)
	return raw
}

func TestPrivateJWKToPEMAndKeyID_SupportedKeyTypes(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	tests := map[string]any{
		"EC":      ecKey,
		"RSA":     rsaKey,
		"Ed25519": edKey,
	}

	for name, key := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := crypto.ParsePrivateKeyFromJWK(marshalJWK(t, key, "my-kid"))

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "my-kid", result.KeyID)

			block, _ := pem.Decode(result.PEM)
			require.NotNil(t, block, "output should be PEM encoded")
			assert.Equal(t, "PRIVATE KEY", block.Type)

			parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			require.NoError(t, err)
			assert.IsType(t, key, parsed)
		})
	}
}

func TestPrivateJWKToPEMAndKeyID_WithoutKeyID_ReturnsEmptyKeyID(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	result, err := crypto.ParsePrivateKeyFromJWK(marshalJWK(t, key, ""))

	require.NoError(t, err)
	assert.Empty(t, result.KeyID)
	assert.NotEmpty(t, result.PEM)
}

func TestPrivateJWKToPEMAndKeyID_PEMRoundTripsToSameKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	result, err := crypto.ParsePrivateKeyFromJWK(marshalJWK(t, key, "kid"))
	require.NoError(t, err)

	block, _ := pem.Decode(result.PEM)
	require.NotNil(t, block)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	assert.True(t, key.Equal(parsed))
}

func TestParsePrivateJWK_InvalidInput_ReturnsError(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	publicJWK, err := json.Marshal(jose.JSONWebKey{Key: &key.PublicKey, KeyID: "kid"})
	require.NoError(t, err)

	tests := map[string]struct {
		input       []byte
		errContains string
	}{
		"not json":       {[]byte("not-a-jwk"), "failed to parse JWK"},
		"empty":          {[]byte(""), "failed to parse JWK"},
		"unknown kty":    {[]byte(`{"kty":"nope"}`), "failed to parse JWK"},
		"public key JWK": {publicJWK, "public key"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			jwk, err := crypto.ParsePrivateJWK(tc.input)

			require.Error(t, err)
			assert.Nil(t, jwk)
			assert.Contains(t, err.Error(), tc.errContains)

			result, err := crypto.ParsePrivateKeyFromJWK(tc.input)
			require.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func TestKeyID(t *testing.T) {
	assert.Equal(t, "abc", crypto.KeyID(&jose.JSONWebKey{KeyID: "abc"}))
	assert.Empty(t, crypto.KeyID(&jose.JSONWebKey{}))
}
