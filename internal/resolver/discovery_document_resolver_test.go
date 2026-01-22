package resolver

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInvalidWellKnownUriGivesError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := defaultZtoperatorAuthPolicy(
		"http://invalid-well-known-uri",
	)

	// 2. Act
	result, err := ResolveDiscoveryDocument(ctx, authPolicy)

	// 3. Assert
	assert.Error(t, err, "ResolveDiscoveryDocument should return an error for invalid well-known URI")
	assert.Nil(t, result, "Result should be nil on error")

	assert.Contains(t, err.Error(), "resolve discovery document")
}

func TestMissingIssuerInDiscoveryDocumentGivesError(t *testing.T) {
	// TODO: Implement a mock REST client to simulate a discovery document missing the issuer field
}

func TestMissingJwksURIInDiscoveryDocumentGivesError(t *testing.T) {
	// TODO: Implement a mock REST client to simulate a discovery document missing the jwks_uri field
}

func TestMissingTokenEndpointInDiscoveryDocumentGivesError(t *testing.T) {
	// TODO: Implement a mock REST client to simulate a discovery document missing the token_endpoint field
}

func TestMissingAuthorizationEndpointWithAutoLoginGivesError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := defaultZtoperatorAuthPolicy(
		// Maskinporten does not have session management endpoints
		// Result for this uri is defined in pkg/rest/dto.go
		"https://maskinporten.no/.well-known/oauth-authorization-server",
	)
	authPolicy.Spec.AutoLogin = &ztoperatorv1alpha1.AutoLogin{
		Enabled: true,
	}

	// 2. Act
	result, err := ResolveDiscoveryDocument(ctx, authPolicy)

	// 3. Assert
	assert.Error(t, err, "ResolveDiscoveryDocument should return an error when auto-login is enabled but authorization endpoint is missing")
	assert.Nil(t, result, "Result should be nil on error")

	assert.Contains(t, err.Error(), "does not support authorization endpoint")
}

func TestMissingAuthorizationEndpointWithoutAutoLoginResolvesSuccessfully(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := defaultZtoperatorAuthPolicy(
		// Maskinporten does not have session management endpoints
		// Result for this uri is defined in pkg/rest/dto.go
		"https://maskinporten.no/.well-known/oauth-authorization-server",
	)
	authPolicy.Spec.AutoLogin = &ztoperatorv1alpha1.AutoLogin{
		Enabled: false,
	}

	// 2. Act
	result, err := ResolveDiscoveryDocument(ctx, authPolicy)

	// 3. Assert
	assert.NoError(t, err, "ResolveDiscoveryDocument should not return an error when autologin is disabled")
	assert.NotNil(t, result, "Result should not be nil when autologin is disabled")

	assert.NotNil(t, result.IssuerURI, "IssuerURI should not be nil")
	assert.NotNil(t, result.JwksURI, "JwksURI should not be nil")
	assert.NotNil(t, result.TokenURI, "TokenURI should not be nil")

	assert.Nil(t, result.AuthorizationURI, "AuthorizationURI should be nil when missing in discovery document")
	assert.Nil(t, result.EndSessionURI, "EndSessionURI should be nil when missing in discovery document")
}

func TestValidWellKnownUriResolvesSuccessfully(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := defaultZtoperatorAuthPolicy(
		// Result for this uri is defined in pkg/rest/dto.go
		"http://mock-oauth2.auth:8080/entraid/.well-known/openid-configuration",
	)

	// 2. Act
	result, err := ResolveDiscoveryDocument(ctx, authPolicy)

	// 3. Assert
	assert.NoError(t, err, "ResolveDiscoveryDocument should not return an error for valid well-known URI")
	assert.NotNil(t, result, "Result should not be nil for valid well-known URI")

	assert.NotNil(t, result.IssuerURI, "IssuerURI should not be nil")
	assert.NotNil(t, result.JwksURI, "JwksURI should not be nil")
	assert.NotNil(t, result.TokenURI, "TokenURI should not be nil")
	assert.NotNil(t, result.AuthorizationURI, "AuthorizationURI should not be nil")
	assert.NotNil(t, result.EndSessionURI, "EndSessionURI should not be nil")
}

func defaultZtoperatorAuthPolicy(
	wellKnownURI string,
) *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			WellKnownURI: wellKnownURI,
			Enabled:      true,
		},
	}
}
