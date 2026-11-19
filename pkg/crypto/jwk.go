package crypto

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/go-jose/go-jose/v4"
)

// ParsePrivateJWK parses a JSON-encoded JWK and verifies that it is a valid private key.
func ParsePrivateJWK(rawJWK []byte) (*jose.JSONWebKey, error) {
	var jwk jose.JSONWebKey
	if err := jwk.UnmarshalJSON(rawJWK); err != nil {
		return nil, fmt.Errorf("failed to parse JWK: %w", err)
	}
	if jwk.IsPublic() {
		return nil, errors.New("JWK is a public key, expected a private key")
	}
	if !jwk.Valid() {
		return nil, errors.New("JWK is not valid")
	}
	return &jwk, nil
}

// PrivateJWKToPEM converts a private JWK to a PKCS#8 PEM-encoded private key ("PRIVATE KEY" block).
func PrivateJWKToPEM(jwk *jose.JSONWebKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(jwk.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key to PKCS#8: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// KeyID returns the JWK's "kid" (empty if the JWK has none).
func KeyID(jwk *jose.JSONWebKey) string {
	return jwk.KeyID
}

// PrivateKey holds a PEM-encoded private key and the optional key id and algorithm from the JWK.
type PrivateKey struct {
	// PEM is the PKCS#8 PEM-encoded private key.
	PEM []byte
	// KeyID is the JWK "kid"; empty when the JWK has none.
	KeyID string
	// Algorithm is the JWK "alg"; empty when the JWK has none.
	Algorithm string
}

// ParsePrivateKeyFromJWK parses a raw private JWK into a PrivateKey (PEM + kid + alg).
func ParsePrivateKeyFromJWK(rawJWK []byte) (*PrivateKey, error) {
	jwk, err := ParsePrivateJWK(rawJWK)
	if err != nil {
		return nil, err
	}
	pemKey, err := PrivateJWKToPEM(jwk)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{PEM: pemKey, KeyID: KeyID(jwk), Algorithm: jwk.Algorithm}, nil
}
