package envoy

import (
	"fmt"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	httpconnectionmanagerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	istionetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
)

func HttpFilterToEnvoyFilterConfigPatch(
	httpFilter *httpconnectionmanagerv3.HttpFilter,
) (*istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch, error) {
	switch httpFilter.GetName() {
	case httpfilter.EnvoyExtensionHttpFilterLua:
		return ToPbStruct(httpFilter)
	case httpfilter.EnvoyExtensionHttpFilterOAuth2:

	default:
		return nil, fmt.Errorf(
			"HTTP filter name %s does not support tranforming to ENvoyFilter config patch",
			httpFilter.GetName(),
		)
	}
}

func ClusterToEnvoyFilterConfigPatch(
	cluster *clusterv3.Cluster,
) (*istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch, error) {

}

func ToPbStruct(msg proto.Message) (*structpb.Struct, error) {
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal %T to JSON: %w", msg, err)
	}
	s := &structpb.Struct{}
	if err := protojson.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("unmarshal %T JSON into protobuf Struct: %w", msg, err)
	}
	return s, nil
}
