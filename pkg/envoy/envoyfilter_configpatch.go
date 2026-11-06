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

// HttpFilterToEnvoyFilterConfigPatch returns the corresponding EnvoyFilter_EnvoyConfigObjectPatch
// for the given HTTP filter. The generated EnvoyFilter config patch inserts the HTTP filter into
// the Envoy sidecar filter chain in the correct order relative to the JWT authentication filter.
//
// Two filters are supported and are executed in the following order:
// 1. Lua filter (envoy.filters.http.lua)
// 2. OAuth2 filter (envoy.filters.http.oauth2)
func HttpFilterToEnvoyFilterConfigPatch(
	httpFilter *httpconnectionmanagerv3.HttpFilter,
) (*istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch, error) {
	switch httpFilter.GetName() {
	case httpfilter.EnvoyExtensionHttpFilterLua:
		luaHttpFilterAsProtobufStruct, err := toPbStruct(httpFilter)
		if err != nil {
			return nil, fmt.Errorf("convert Lua HTTP filter to protobuf Struct: %w", err)
		}
		return &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
			ApplyTo: istionetworkingv1alpha3.EnvoyFilter_HTTP_FILTER,
			Match: &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
				Context: istionetworkingv1alpha3.EnvoyFilter_SIDECAR_INBOUND,
				ObjectTypes: &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
					Listener: &istionetworkingv1alpha3.EnvoyFilter_ListenerMatch{
						FilterChain: &istionetworkingv1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
							Filter: &istionetworkingv1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
								Name: "envoy.filters.network.http_connection_manager",
							},
						},
					},
				},
			},
			Patch: &istionetworkingv1alpha3.EnvoyFilter_Patch{
				Operation: istionetworkingv1alpha3.EnvoyFilter_Patch_INSERT_BEFORE,
				Value:     luaHttpFilterAsProtobufStruct,
			},
		}, nil
	case httpfilter.EnvoyExtensionHttpFilterOAuth2:
		oAuth2HttpFilterAsProtobufStruct, err := toPbStruct(httpFilter)
		if err != nil {
			return nil, fmt.Errorf("convert OAuth2 HTTP filter to protobuf Struct: %w", err)
		}
		return &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
			ApplyTo: istionetworkingv1alpha3.EnvoyFilter_HTTP_FILTER,
			Match: &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
				Context: istionetworkingv1alpha3.EnvoyFilter_SIDECAR_INBOUND,
				ObjectTypes: &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
					Listener: &istionetworkingv1alpha3.EnvoyFilter_ListenerMatch{
						FilterChain: &istionetworkingv1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
							Filter: &istionetworkingv1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
								Name: "envoy.filters.network.http_connection_manager",
								SubFilter: &istionetworkingv1alpha3.EnvoyFilter_ListenerMatch_SubFilterMatch{
									Name: "envoy.filters.http.jwt_authn",
								},
							},
						},
					},
				},
			},
			Patch: &istionetworkingv1alpha3.EnvoyFilter_Patch{
				Operation: istionetworkingv1alpha3.EnvoyFilter_Patch_INSERT_BEFORE,
				Value:     oAuth2HttpFilterAsProtobufStruct,
			},
		}, nil
	default:
		return nil, fmt.Errorf(
			"HTTP filter name %s does not support transforming to EnvoyFilter config patch",
			httpFilter.GetName(),
		)
	}
}

func ClusterToEnvoyFilterConfigPatch(
	cluster *clusterv3.Cluster,
) (*istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch, error) {
	clusterAsProtobufStruct, err := toPbStruct(cluster)
	if err != nil {
		return nil, fmt.Errorf("convert cluster to protobuf Struct: %w", err)
	}
	return &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: istionetworkingv1alpha3.EnvoyFilter_CLUSTER,
		Match: &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			ObjectTypes: &istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Cluster{
				Cluster: &istionetworkingv1alpha3.EnvoyFilter_ClusterMatch{
					Service: cluster.GetName(),
				},
			},
		},
		Patch: &istionetworkingv1alpha3.EnvoyFilter_Patch{
			Operation: istionetworkingv1alpha3.EnvoyFilter_Patch_ADD,
			Value:     clusterAsProtobufStruct,
		},
	}, nil
}

func toPbStruct(msg proto.Message) (*structpb.Struct, error) {
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal %T to JSON: %w", msg, err)
	}
	s := &structpb.Struct{}
	if protobufJsonErr := protojson.Unmarshal(b, s); protobufJsonErr != nil {
		return nil, fmt.Errorf("unmarshal %T JSON into protobuf Struct: %w", msg, protobufJsonErr)
	}
	return s, nil
}
