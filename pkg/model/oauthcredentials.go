package model

import (
	"errors"
	"fmt"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	oauth2v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/oauth2/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/crypto"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"google.golang.org/protobuf/types/known/durationpb"
)

func ToOAuthCredentials(
	authPolicy ztoperatorv1alpha1.AuthPolicy,
	secretData map[string][]byte,
	assertionAudience string,
) (*OAuthCredentials, error) {
	clientID := string(secretData[authPolicy.Spec.OAuthCredentials.ClientIDKey])
	if clientID == "" {
		return nil, fmt.Errorf(
			"client id with key: %s was nil or empty when retrieving it from Secret with name %s/%s",
			authPolicy.Spec.OAuthCredentials.ClientIDKey,
			authPolicy.Namespace,
			authPolicy.Spec.OAuthCredentials.SecretRef,
		)
	}
	hmacSecret, err := helperfunctions.GenerateHMACSecret(32)
	if err != nil {
		return nil, err
	}

	oAuthCredentials := OAuthCredentials{
		ClientID: &clientID,
		HmacSecret: HmacSecret{
			Value: &tlsv3.Secret{
				Name: "hmac",
				Type: &tlsv3.Secret_GenericSecret{
					GenericSecret: &tlsv3.GenericSecret{
						Secret: &corev3.DataSource{
							Specifier: &corev3.DataSource_InlineBytes{InlineBytes: hmacSecret},
						},
					},
				},
			},
		},
	}

	tokenSecret, err := GetTokenSecret(
		secretData,
		assertionAudience,
		authPolicy.Spec.OAuthCredentials.ClientSecretKey,
		authPolicy.Spec.OAuthCredentials.PrivateJwkKey,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get token secret from OAuth credentials secret %s/%s: %w",
			authPolicy.Namespace,
			authPolicy.Spec.OAuthCredentials.SecretRef,
			err,
		)
	}
	oAuthCredentials.TokenSecret = *tokenSecret

	return &oAuthCredentials, nil
}

func GetTokenSecret(
	secretData map[string][]byte,
	assertionAudience string,
	clientSecretKey, privateJwkKey string,
) (*TokenSecret, error) {
	if clientSecretKey != "" {
		return getClientSecretTokenSecret(secretData, clientSecretKey)
	}
	return getPrivateKeyJwtTokenSecret(secretData, assertionAudience, privateJwkKey)
}

func getClientSecretTokenSecret(secretData map[string][]byte, clientSecretKey string) (*TokenSecret, error) {
	clientSecret := string(secretData[clientSecretKey])
	if clientSecret == "" {
		return nil, errors.New("client secret was nil or empty")
	}
	return &TokenSecret{
		AuthType: oauth2v3.OAuth2Config_URL_ENCODED_BODY,
		Value: &tlsv3.Secret{
			Name: "token",
			Type: &tlsv3.Secret_GenericSecret{
				GenericSecret: &tlsv3.GenericSecret{
					Secret: &corev3.DataSource{
						Specifier: &corev3.DataSource_InlineString{InlineString: clientSecret},
					},
				},
			},
		},
	}, nil
}

func getPrivateKeyJwtTokenSecret(
	secretData map[string][]byte,
	assertionAudience string,
	privateJwkKey string,
) (*TokenSecret, error) {
	privateJWK := secretData[privateJwkKey]
	if len(privateJWK) == 0 {
		return nil, errors.New("private JWK was nil or empty")
	}

	privateKey, err := crypto.ParsePrivateKeyFromJWK(privateJWK)
	if err != nil {
		return nil, errors.New("failed to parse private JWK")
	}
	signingAlgorithm, ok := oauth2v3.PrivateKeyJwtConfig_SigningAlgorithm_value[privateKey.Algorithm]
	if !ok {
		return nil, fmt.Errorf("unsupported signing algorithm %q for private JWK, supported values are %v",
			privateKey.Algorithm,
			oauth2v3.PrivateKeyJwtConfig_SigningAlgorithm_name,
		)
	}

	return &TokenSecret{
		AuthType: oauth2v3.OAuth2Config_PRIVATE_KEY_JWT,
		PrivateKeyJwtConfig: &oauth2v3.PrivateKeyJwtConfig{
			SigningAlgorithm:  oauth2v3.PrivateKeyJwtConfig_SigningAlgorithm(signingAlgorithm),
			AssertionLifetime: durationpb.New(60 * time.Second),
			AssertionAudience: assertionAudience,
		},
		Value: &tlsv3.Secret{
			Name: "token",
			Type: &tlsv3.Secret_GenericSecret{GenericSecret: privateKeyGenericSecret(privateKey)},
		},
	}, nil
}

func privateKeyGenericSecret(privateKey *crypto.PrivateKey) *tlsv3.GenericSecret {
	if privateKey.KeyID == "" {
		return &tlsv3.GenericSecret{
			Secret: &corev3.DataSource{
				Specifier: &corev3.DataSource_InlineString{InlineString: string(privateKey.PEM)},
			},
		}
	}
	return &tlsv3.GenericSecret{
		Secrets: map[string]*corev3.DataSource{
			"private_key": {Specifier: &corev3.DataSource_InlineString{InlineString: string(privateKey.PEM)}},
			"key_id":      {Specifier: &corev3.DataSource_InlineString{InlineString: privateKey.KeyID}},
		},
	}
}
