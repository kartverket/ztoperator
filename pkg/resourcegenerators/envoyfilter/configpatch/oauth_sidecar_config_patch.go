package configpatch

import (
	"fmt"

	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter"
	"github.com/kartverket/ztoperator/pkg/model"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func GetOAuth2ConfigPatchValue(
	tokenEndpointClusterName string,
	autoLoginConfig model.AutoLoginConfig,
	identityProviderUris model.IdentityProviderUris,
	clientID string,
) (*structpb.Struct, error) {
	oAuth2, err := httpfilter.OAuth2HttpFilter(
		autoLoginConfig,
		identityProviderUris,
		clientID,
		tokenEndpointClusterName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth2 HTTP filter config patch value: %w", err)
	}
	oAuth2Any, err := anypb.New(oAuth2)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize OAuth2 HTTP filter to protobuf Any: %w", err)
	}
	patchValue, err := ToPbStruct(oAuth2Any)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize OAuth2 HTTP filter to protobuf Struct: %w", err)
	}
	return patchValue, nil
}
