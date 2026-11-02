package configpatch

import (
	"fmt"

	"github.com/kartverket/ztoperator/pkg/envoy/cluster"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func GetTokenEndpointClusterConfigPatchValue(
	clusterName string,
	tokenUrl string,
) (*structpb.Struct, error) {
	tokenEndpointCluster, err := cluster.FromURI(clusterName, tokenUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to create token endpoint cluster: %w", err)
	}
	tokenEndpointClusterAny, err := anypb.New(tokenEndpointCluster)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize token endpoint cluster to protobuf Any: %w", err)
	}
	patchValue, err := ToPbStruct(tokenEndpointClusterAny)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize token endpoint cluster patch value to protobuf struct: %w", err)
	}
	return patchValue, nil
}
