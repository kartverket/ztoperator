package oauth2

import (
	"fmt"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	oauth2v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/oauth2/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/kartverket/ztoperator/pkg/constants"
	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/kartverket/ztoperator/pkg/model"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	AuthorizationHeaderName        = "authorization"
	AuthorizationHeaderValuePrefix = "Bearer "

	HostUriFromEnvoyHeaders  = "%REQ(x-forwarded-proto)%://%REQ(:authority)%"
	SdsConfigTokenSecretName = "token"
	SdsConfigHmacSecretName  = "hmac"
)

func OAuth2(
	identityProviderUris model.IdentityProviderUris,
	clientID string,
	resources, scopes []string,
	signoutPath, redirectPath, tokenClusterName string,
	postLogoutRedirectUri *string,
) (*oauth2v3.OAuth2, error) {
	oauth2Filter := oauth2v3.OAuth2{
		Config: &oauth2v3.OAuth2Config{
			TokenEndpoint: &corev3.HttpUri{
				Uri: identityProviderUris.TokenURI,
				HttpUpstreamType: &corev3.HttpUri_Cluster{
					Cluster: tokenClusterName,
				},
				Timeout: durationpb.New(5 * time.Second),
			},
			AuthorizationEndpoint: identityProviderUris.AuthorizationURI,
			RetryPolicy:           nil,
			Credentials: &oauth2v3.OAuth2Credentials{
				ClientId: clientID,
				TokenSecret: sdsFileSecret(
					SdsConfigTokenSecretName,
					constants.EnvoyOAuth2TokenSecretFilePath,
					constants.EnvoyOAuth2CredentialsDirectory,
				),
				TokenFormation: &oauth2v3.OAuth2Credentials_HmacSecret{
					HmacSecret: sdsFileSecret(
						SdsConfigHmacSecretName,
						constants.EnvoyOAuth2HmacSecretFilePath,
						constants.EnvoyOAuth2CredentialsDirectory,
					),
				},
			},
			RedirectUri: HostUriFromEnvoyHeaders + redirectPath,
			RedirectPathMatcher: &matcherv3.PathMatcher{
				Rule: &matcherv3.PathMatcher_Path{
					Path: &matcherv3.StringMatcher{
						MatchPattern: &matcherv3.StringMatcher_Exact{
							Exact: redirectPath,
						},
					},
				},
			},
			SignoutPath: &matcherv3.PathMatcher{
				Rule: &matcherv3.PathMatcher_Path{
					Path: &matcherv3.StringMatcher{
						MatchPattern: &matcherv3.StringMatcher_Exact{
							Exact: signoutPath,
						},
					},
				},
			},
			ForwardBearerToken: true,
			PassThroughMatcher: []*routev3.HeaderMatcher{
				{
					Name: AuthorizationHeaderName,
					HeaderMatchSpecifier: &routev3.HeaderMatcher_StringMatch{
						StringMatch: &matcherv3.StringMatcher{
							MatchPattern: &matcherv3.StringMatcher_Prefix{
								Prefix: AuthorizationHeaderValuePrefix,
							},
						},
					},
				},
				{
					Name: luascript.OAuth2FilterPassThroughHeaderName,
					HeaderMatchSpecifier: &routev3.HeaderMatcher_StringMatch{
						StringMatch: &matcherv3.StringMatcher{
							MatchPattern: &matcherv3.StringMatcher_Exact{
								Exact: luascript.OAuth2FilterPassThroughHeaderValue,
							},
						},
					},
				},
			},
			AuthScopes:      scopes,
			Resources:       resources,
			AuthType:        oauth2v3.OAuth2Config_URL_ENCODED_BODY,
			UseRefreshToken: &wrapperspb.BoolValue{Value: true},
			DenyRedirectMatcher: []*routev3.HeaderMatcher{
				{
					Name: luascript.OAuth2FilterDenyRedirectHeaderName,
					HeaderMatchSpecifier: &routev3.HeaderMatcher_StringMatch{
						StringMatch: &matcherv3.StringMatcher{
							MatchPattern: &matcherv3.StringMatcher_Exact{
								Exact: luascript.OAuth2FilterDenyRedirectHeaderValue,
							},
						},
					},
				},
			},
			CookieConfigs: &oauth2v3.CookieConfigs{
				BearerTokenCookieConfig:  &oauth2v3.CookieConfig{SameSite: oauth2v3.CookieConfig_LAX},
				OauthHmacCookieConfig:    &oauth2v3.CookieConfig{SameSite: oauth2v3.CookieConfig_LAX},
				OauthExpiresCookieConfig: &oauth2v3.CookieConfig{SameSite: oauth2v3.CookieConfig_LAX},
				IdTokenCookieConfig:      &oauth2v3.CookieConfig{SameSite: oauth2v3.CookieConfig_LAX},
				RefreshTokenCookieConfig: &oauth2v3.CookieConfig{SameSite: oauth2v3.CookieConfig_LAX},
				OauthNonceCookieConfig:   &oauth2v3.CookieConfig{SameSite: oauth2v3.CookieConfig_LAX},
				CodeVerifierCookieConfig: &oauth2v3.CookieConfig{SameSite: oauth2v3.CookieConfig_LAX},
			},
			DisableTokenEncryption: false,
		},
	}

	if identityProviderUris.EndSessionURI != nil {
		oauth2Filter.Config.EndSessionEndpoint = *identityProviderUris.EndSessionURI
		if postLogoutRedirectUri != nil {
			oauth2Filter.Config.PostLogoutRedirectUri = &oauth2v3.PostLogoutRedirectUri{
				Config: &oauth2v3.PostLogoutRedirectUri_Uri{
					Uri: *postLogoutRedirectUri,
				},
			}
		}
	}

	var err error
	if validateErr := oauth2Filter.ValidateAll(); validateErr != nil {
		err = fmt.Errorf("error validating OAuth2 filter: %w", validateErr)
	}

	return &oauth2Filter, err
}

func sdsFileSecret(name, filepath, watchedDir string) *tlsv3.SdsSecretConfig {
	return &tlsv3.SdsSecretConfig{
		Name: name,
		SdsConfig: &corev3.ConfigSource{
			ConfigSourceSpecifier: &corev3.ConfigSource_PathConfigSource{
				PathConfigSource: &corev3.PathConfigSource{
					Path: filepath,
					WatchedDirectory: &corev3.WatchedDirectory{
						Path: watchedDir,
					},
				},
			},
		},
	}
}
