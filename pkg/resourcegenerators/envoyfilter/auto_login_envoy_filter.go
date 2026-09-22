package envoyfilter

import (
	"github.com/kartverket/ztoperator/pkg/model"
	"google.golang.org/protobuf/types/known/structpb"
	"istio.io/api/networking/v1alpha3"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
)

const oAuthClusterName = "oauth"

// GetDesired returns the desired EnvoyFilter resource for the given AuthPolicy scope
//
// The generated EnvoyFilter inserts three config patches into the Envoy sidecar filter chain in
// the following order:
//
//  1. A Lua HTTP filter (INSERT_BEFORE jwt_authn) that handles OAuth2 redirect detection, logout
//     (RP-initiated logout via the IdP's end_session_endpoint), deny-redirect for API endpoints,
//     and cookie-based session management. On successful login it injects an
//     "Authorization: Bearer <token>" header for downstream JWT validation.
//
//  2. An OAuth2 cluster (ADD) that configures the upstream cluster Envoy uses to communicate with
//     the identity provider's token endpoint. Internal IdPs (with an explicit port) are configured
//     as STATIC clusters; external IdPs are configured as STRICT_DNS clusters.
//
//  3. An OAuth2 HTTP filter (INSERT_BEFORE jwt_authn) that drives the Authorization Code Flow and
//     exchanges the authorization code for tokens using the upstream OAuth2 cluster defined above.
func GetDesired(scope *model.Scope, objectMeta v1.ObjectMeta) *v1alpha4.EnvoyFilter {
	if !scope.AuthPolicy.Spec.Enabled || scope.InvalidConfig || scope.AuthPolicy.Spec.AutoLogin == nil ||
		!scope.AuthPolicy.Spec.AutoLogin.Enabled {
		return nil
	}

	oAuthClusterConfigPatchValue, err := configpatch.GetOAuth2ClusterConfig(
		oAuthClusterName, scope.IdentityProviderUris.TokenURI,
	)
	if err != nil {
		panic("failed to get oauth envoy cluster config patch: " + err.Error())
	}
	oAuthClusterConfigPatchValueAsPbStruct, err := structpb.NewStruct(
		*oAuthClusterConfigPatchValue,
	)
	if err != nil {
		panic(
			"failed to serialize OAuth Cluster Config Patch to protobuf struct due to the following error: " + err.Error(),
		)
	}

	luaScriptConfigPatchValue, err := structpb.NewStruct(configpatch.GetLuaScriptFilterConfig(
		scope.AutoLoginConfig.LuaScriptConfig.LuaScript,
	))
	if err != nil {
		panic(
			"failed to serialize Lua script config patch value due to the following error: " + err.Error(),
		)
	}

	oAuthSidecarConfigPatchValueAsPbStruct, err := structpb.NewStruct(
		configpatch.GetOAuth2FilterConfig(
			oAuthClusterName,
			scope.AutoLoginConfig,
			scope.IdentityProviderUris,
			*scope.OAuthCredentials.ClientID,
		),
	)
	if err != nil {
		panic(
			"failed to serialize OAuth Sidecar Config Patch to protobuf struct due to the following error: " + err.Error(),
		)
	}

	// Pre-allocating the slice with a length of 3 since we know there will be exactly 3 patches.
	configPatches := make([]*v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch, 0, 3)

	configPatches = append(configPatches, &v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: v1alpha3.EnvoyFilter_HTTP_FILTER,
		Match: &v1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: v1alpha3.EnvoyFilter_SIDECAR_INBOUND,
			ObjectTypes: &v1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
				Listener: &v1alpha3.EnvoyFilter_ListenerMatch{
					FilterChain: &v1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
						Filter: &v1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
							Name: "envoy.filters.network.http_connection_manager",
						},
					},
				},
			},
		},
		Patch: &v1alpha3.EnvoyFilter_Patch{
			Operation: v1alpha3.EnvoyFilter_Patch_INSERT_BEFORE,
			Value:     luaScriptConfigPatchValue,
		},
	})

	configPatches = append(configPatches, &v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: v1alpha3.EnvoyFilter_CLUSTER,
		Match: &v1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			ObjectTypes: &v1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Cluster{
				Cluster: &v1alpha3.EnvoyFilter_ClusterMatch{
					Service: "oauth",
				},
			},
		},
		Patch: &v1alpha3.EnvoyFilter_Patch{
			Operation: v1alpha3.EnvoyFilter_Patch_ADD,
			Value:     oAuthClusterConfigPatchValueAsPbStruct,
		},
	})

	configPatches = append(configPatches, &v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: v1alpha3.EnvoyFilter_HTTP_FILTER,
		Match: &v1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: v1alpha3.EnvoyFilter_SIDECAR_INBOUND,
			ObjectTypes: &v1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
				Listener: &v1alpha3.EnvoyFilter_ListenerMatch{
					FilterChain: &v1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
						Filter: &v1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
							Name: "envoy.filters.network.http_connection_manager",
							SubFilter: &v1alpha3.EnvoyFilter_ListenerMatch_SubFilterMatch{
								Name: "envoy.filters.http.jwt_authn",
							},
						},
					},
				},
			},
		},
		Patch: &v1alpha3.EnvoyFilter_Patch{
			Operation: v1alpha3.EnvoyFilter_Patch_INSERT_BEFORE,
			Value:     oAuthSidecarConfigPatchValueAsPbStruct,
		},
	})

	return &v1alpha4.EnvoyFilter{
		ObjectMeta: objectMeta,
		Spec: v1alpha3.EnvoyFilter{
			ConfigPatches: configPatches,
			WorkloadSelector: &v1alpha3.WorkloadSelector{
				Labels: scope.AuthPolicy.Spec.Selector.MatchLabels,
			},
		},
	}
}
