package resolver_test

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/resolver"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInvalidWellKnownUriGivesError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := defaultZtoperatorAuthPolicy(
		"http://invalid-well-known-uri",
	)

	// 2. Act
	result, err := resolver.ResolveDiscoveryDocument(ctx, authPolicy, rest.NewDefaultDiscoveryDocumentResolver())

	// 3. Assert
	require.Error(t, err, "ResolveDiscoveryDocument should return an error for invalid well-known URI")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "resolve discovery document")
}

func TestMissingIssuerInDiscoveryDocumentGivesError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := defaultZtoperatorAuthPolicy("http://test-idp.example.com/.well-known/openid-configuration")
	mockResolver := &mockDiscoveryDocumentResolver{
		document: &rest.DiscoveryDocument{
			Issuer:                nil, // Missing issuer
			TokenEndpoint:         helperfunctions.Ptr("http://test-idp.example.com/token"),
			JwksURI:               helperfunctions.Ptr("http://test-idp.example.com/jwks"),
			AuthorizationEndpoint: helperfunctions.Ptr("http://test-idp.example.com/authorize"),
			EndSessionEndpoint:    helperfunctions.Ptr("http://test-idp.example.com/session"),
		},
	}

	// 2. Act
	result, err := resolver.ResolveDiscoveryDocument(ctx, authPolicy, mockResolver)

	// 3. Assert
	require.Error(t, err, "ResolveDiscoveryDocument should return an error when issuer is missing")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "failed to parse discovery document")
}

func TestMissingJwksURIInDiscoveryDocumentGivesError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := defaultZtoperatorAuthPolicy("http://test-idp.example.com/.well-known/openid-configuration")
	mockResolver := &mockDiscoveryDocumentResolver{
		document: &rest.DiscoveryDocument{
			Issuer:                helperfunctions.Ptr("http://test-idp.example.com"),
			TokenEndpoint:         helperfunctions.Ptr("http://test-idp.example.com/token"),
			JwksURI:               nil, // Missing jwks_uri
			AuthorizationEndpoint: helperfunctions.Ptr("http://test-idp.example.com/authorize"),
			EndSessionEndpoint:    helperfunctions.Ptr("http://test-idp.example.com/session"),
		},
	}

	// 2. Act
	result, err := resolver.ResolveDiscoveryDocument(ctx, authPolicy, mockResolver)

	// 3. Assert
	require.Error(t, err, "ResolveDiscoveryDocument should return an error when jwks_uri is missing")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "failed to parse discovery document")
}

func TestMissingTokenEndpointInDiscoveryDocumentGivesError(t *testing.T) {
	ctx := context.Background()

	// 1. Arrange
	authPolicy := defaultZtoperatorAuthPolicy("http://test-idp.example.com/.well-known/openid-configuration")
	mockResolver := &mockDiscoveryDocumentResolver{
		document: &rest.DiscoveryDocument{
			Issuer:                helperfunctions.Ptr("http://test-idp.example.com"),
			TokenEndpoint:         nil, // Missing token_endpoint
			JwksURI:               helperfunctions.Ptr("http://test-idp.example.com/jwks"),
			AuthorizationEndpoint: helperfunctions.Ptr("http://test-idp.example.com/authorize"),
			EndSessionEndpoint:    helperfunctions.Ptr("http://test-idp.example.com/session"),
		},
	}

	// 2. Act
	result, err := resolver.ResolveDiscoveryDocument(ctx, authPolicy, mockResolver)

	// 3. Assert
	require.Error(t, err, "ResolveDiscoveryDocument should return an error when token_endpoint is missing")
	assert.Nil(t, result, "Result should be nil on error")
	assert.Contains(t, err.Error(), "failed to parse discovery document")
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
	result, err := resolver.ResolveDiscoveryDocument(ctx, authPolicy, rest.NewDefaultDiscoveryDocumentResolver())

	// 3. Assert
	require.Error(
		t,
		err,
		"ResolveDiscoveryDocument should return an error when auto-login is enabled but authorization endpoint is missing",
	)
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
	result, err := resolver.ResolveDiscoveryDocument(ctx, authPolicy, rest.NewDefaultDiscoveryDocumentResolver())

	// 3. Assert
	require.NoError(t, err, "ResolveDiscoveryDocument should not return an error when autologin is disabled")
	require.NotNil(t, result, "Result should not be nil when autologin is disabled")

	assert.NotNil(t, result.IssuerURI, "IssuerURI should not be nil")
	assert.NotNil(t, result.JwksURI, "JwksURI should not be nil")
	assert.NotNil(t, result.TokenURI, "TokenURI should not be nil")

	assert.Empty(
		t,
		result.AuthorizationURI,
		"AuthorizationURI should be empty string when missing in discovery document",
	)
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
	result, err := resolver.ResolveDiscoveryDocument(ctx, authPolicy, rest.NewDefaultDiscoveryDocumentResolver())

	// 3. Assert
	require.NoError(t, err, "ResolveDiscoveryDocument should not return an error for valid well-known URI")
	require.NotNil(t, result, "Result should not be nil for valid well-known URI")

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

type mockDiscoveryDocumentResolver struct {
	document *rest.DiscoveryDocument
	err      error
}

func (m *mockDiscoveryDocumentResolver) GetOAuthDiscoveryDocument(
	_ string,
	_ log.Logger,
) (*rest.DiscoveryDocument, error) {
	return m.document, m.err
}
