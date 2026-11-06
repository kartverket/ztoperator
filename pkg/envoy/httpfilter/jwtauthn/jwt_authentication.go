package jwtauthn

import (
	"fmt"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	jwtauthnv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	JwtAuthenticationFilterStatPrefix         = "jwt_authn"
	JwtAuthenticationFilterPayloadMetadataKey = "jwt_payload"
	jwtProviderName                           = "origin-0"
)

func JwtAuthentication(
	issuerUri, jwksUri, jwksClusterName string,
	audiences []string,
) (*jwtauthnv3.JwtAuthentication, error) {
	jwtAuthentication := jwtauthnv3.JwtAuthentication{
		Providers: map[string]*jwtauthnv3.JwtProvider{
			jwtProviderName: {
				Issuer: issuerUri,
				JwksSourceSpecifier: &jwtauthnv3.JwtProvider_RemoteJwks{
					RemoteJwks: &jwtauthnv3.RemoteJwks{
						HttpUri: &corev3.HttpUri{
							Uri:              jwksUri,
							HttpUpstreamType: &corev3.HttpUri_Cluster{Cluster: jwksClusterName},
							Timeout:          durationpb.New(10 * time.Second),
						},
						CacheDuration: durationpb.New(60 * time.Minute),
						AsyncFetch: &jwtauthnv3.JwksAsyncFetch{
							FastListener:          false,
							FailedRefetchDuration: durationpb.New(5 * time.Second),
						},
						RetryPolicy: &corev3.RetryPolicy{
							RetryBackOff: &corev3.BackoffStrategy{
								BaseInterval: durationpb.New(100 * time.Millisecond),
								MaxInterval:  durationpb.New(20 * time.Second),
							},
							NumRetries: &wrapperspb.UInt32Value{Value: uint32(10)},
						},
					},
				},
				Audiences:         audiences,
				Forward:           true,
				PayloadInMetadata: JwtAuthenticationFilterPayloadMetadataKey,
				NormalizePayloadInMetadata: &jwtauthnv3.JwtProvider_NormalizePayload{
					SpaceDelimitedClaims: []string{"permission", "scope"},
				},
			},
		},
		Rules: []*jwtauthnv3.RequirementRule{
			{
				Match: &routev3.RouteMatch{
					PathSpecifier: &routev3.RouteMatch_Prefix{
						Prefix: "/",
					},
				},
				RequirementType: &jwtauthnv3.RequirementRule_Requires{
					Requires: &jwtauthnv3.JwtRequirement{
						RequiresType: &jwtauthnv3.JwtRequirement_RequiresAny{
							RequiresAny: &jwtauthnv3.JwtRequirementOrList{
								Requirements: []*jwtauthnv3.JwtRequirement{
									{
										RequiresType: &jwtauthnv3.JwtRequirement_ProviderName{
											ProviderName: jwtProviderName,
										},
									},
									{
										RequiresType: &jwtauthnv3.JwtRequirement_AllowMissing{
											AllowMissing: &emptypb.Empty{},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		BypassCorsPreflight: true,
		StatPrefix:          JwtAuthenticationFilterStatPrefix,
	}
	var err error
	if validateErr := jwtAuthentication.ValidateAll(); validateErr != nil {
		err = fmt.Errorf("error validating JwtAuthentication filter: %w", validateErr)
	}
	return &jwtAuthentication, err
}
