package oauth2_test

import (
	"testing"
	"time"

	oauth2v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/oauth2/v3"
	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter/oauth2"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	testEndSessionURI         = "https://idp.example.com/logout"
	testPostLogoutRedirectURI = "https://app.example.com/"
)

func TestOAuth2_PrivateKeyJwtConfig(t *testing.T) {
	privateKeyJwtConfig := &oauth2v3.PrivateKeyJwtConfig{
		SigningAlgorithm:  oauth2v3.PrivateKeyJwtConfig_ES256,
		AssertionLifetime: durationpb.New(60 * time.Second),
		AssertionAudience: "https://idp.example.com",
	}

	tests := []struct {
		name                string
		authType            oauth2v3.OAuth2Config_AuthType
		privateKeyJwtConfig *oauth2v3.PrivateKeyJwtConfig
		expected            *oauth2v3.PrivateKeyJwtConfig
	}{
		{
			name:                "set when auth type is PRIVATE_KEY_JWT",
			authType:            oauth2v3.OAuth2Config_PRIVATE_KEY_JWT,
			privateKeyJwtConfig: privateKeyJwtConfig,
			expected:            privateKeyJwtConfig,
		},
		{
			name:                "not set when auth type is URL_ENCODED_BODY",
			authType:            oauth2v3.OAuth2Config_URL_ENCODED_BODY,
			privateKeyJwtConfig: privateKeyJwtConfig,
			expected:            nil,
		},
		{
			name:                "not set when auth type is BASIC_AUTH",
			authType:            oauth2v3.OAuth2Config_BASIC_AUTH,
			privateKeyJwtConfig: privateKeyJwtConfig,
			expected:            nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter, err := buildOAuth2(test.authType, test.privateKeyJwtConfig, testIdentityProviderUris(nil), nil)

			require.NoError(t, err)
			assert.Equal(t, test.authType, filter.GetConfig().GetAuthType())
			assert.True(
				t,
				proto.Equal(test.expected, filter.GetConfig().GetPrivateKeyJwtConfig()),
				"expected PrivateKeyJwtConfig %v, got %v",
				test.expected,
				filter.GetConfig().GetPrivateKeyJwtConfig(),
			)
		})
	}
}

func TestOAuth2_EndSessionEndpointAndPostLogoutRedirectUri(t *testing.T) {
	tests := []struct {
		name                          string
		endSessionURI                 *string
		postLogoutRedirectURI         *string
		expectedEndSessionEndpoint    string
		expectedPostLogoutRedirectURI *string
	}{
		{
			name:                          "both set when end session URI and post logout redirect URI are provided",
			endSessionURI:                 helperfunctions.Ptr(testEndSessionURI),
			postLogoutRedirectURI:         helperfunctions.Ptr(testPostLogoutRedirectURI),
			expectedEndSessionEndpoint:    testEndSessionURI,
			expectedPostLogoutRedirectURI: helperfunctions.Ptr(testPostLogoutRedirectURI),
		},
		{
			name:                          "only end session endpoint set when post logout redirect URI is nil",
			endSessionURI:                 helperfunctions.Ptr(testEndSessionURI),
			postLogoutRedirectURI:         nil,
			expectedEndSessionEndpoint:    testEndSessionURI,
			expectedPostLogoutRedirectURI: nil,
		},
		{
			name:                          "neither set when end session URI is nil",
			endSessionURI:                 nil,
			postLogoutRedirectURI:         helperfunctions.Ptr(testPostLogoutRedirectURI),
			expectedEndSessionEndpoint:    "",
			expectedPostLogoutRedirectURI: nil,
		},
		{
			name:                          "neither set when both are nil",
			endSessionURI:                 nil,
			postLogoutRedirectURI:         nil,
			expectedEndSessionEndpoint:    "",
			expectedPostLogoutRedirectURI: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter, err := buildOAuth2(
				oauth2v3.OAuth2Config_URL_ENCODED_BODY,
				nil,
				testIdentityProviderUris(test.endSessionURI),
				test.postLogoutRedirectURI,
			)

			require.NoError(t, err)
			assert.Equal(t, test.expectedEndSessionEndpoint, filter.GetConfig().GetEndSessionEndpoint())
			if test.expectedPostLogoutRedirectURI == nil {
				assert.Nil(t, filter.GetConfig().GetPostLogoutRedirectUri())
				return
			}
			require.NotNil(t, filter.GetConfig().GetPostLogoutRedirectUri())
			assert.Equal(t, *test.expectedPostLogoutRedirectURI, filter.GetConfig().GetPostLogoutRedirectUri().GetUri())
		})
	}
}

func testIdentityProviderUris(endSessionURI *string) model.IdentityProviderUris {
	return model.IdentityProviderUris{
		IssuerURI:        "https://idp.example.com",
		JwksURI:          "https://idp.example.com/jwks",
		TokenURI:         "https://idp.example.com/token",
		AuthorizationURI: "https://idp.example.com/authorize",
		EndSessionURI:    endSessionURI,
	}
}

func buildOAuth2(
	authType oauth2v3.OAuth2Config_AuthType,
	privateKeyJwtConfig *oauth2v3.PrivateKeyJwtConfig,
	identityProviderUris model.IdentityProviderUris,
	postLogoutRedirectURI *string,
) (*oauth2v3.OAuth2, error) {
	return oauth2.OAuth2(
		identityProviderUris,
		"client-id",
		authType,
		privateKeyJwtConfig,
		nil,
		[]string{"openid"},
		"/logout",
		"/oauth2/callback",
		"token-cluster",
		postLogoutRedirectURI,
	)
}
