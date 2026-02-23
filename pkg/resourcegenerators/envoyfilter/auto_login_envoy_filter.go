package envoyfilter

import (
	"strconv"

	"google.golang.org/protobuf/types/known/structpb"
	"istio.io/api/networking/v1alpha3"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v1alpha4.EnvoyFilter {
	if !scope.AuthPolicy.Spec.Enabled || scope.InvalidConfig || scope.AuthPolicy.Spec.AutoLogin == nil ||
		!scope.AuthPolicy.Spec.AutoLogin.Enabled {
		return nil
	}

	idpAsParsedURL, err := helperfunctions.GetParsedURL(scope.IdentityProviderUris.TokenURI)
	if err != nil {
		panic(
			"failed to get issuer hostname from issuer URI " + scope.IdentityProviderUris.IssuerURI +
				" due to the following error: " + err.Error(),
		)
	}
	var oAuthClusterConfigPatchValue map[string]interface{}
	if idpAsParsedURL.Port() != "" {
		// Internal IDP
		port, strconvErr := strconv.Atoi(idpAsParsedURL.Port())
		if strconvErr != nil {
			panic(strconvErr)
		}
		oAuthClusterConfigPatchValue = configpatch.GetInternalOAuthClusterConfigPatchValue(
			idpAsParsedURL.Hostname(),
			port,
		)
	} else {
		oAuthClusterConfigPatchValue = configpatch.GetExternalOAuthClusterPatchValue(idpAsParsedURL.Host)
	}

	luaScriptConfigPatchValue, err := structpb.NewStruct(configpatch.GetLuaScriptConfigPatch(*scope))
	if err != nil {
		panic(
			"failed to serialize Lua script config patch value due to the following error: " + err.Error(),
		)
	}

	oAuthClusterConfigPatchValueAsPbStruct, err := structpb.NewStruct(
		oAuthClusterConfigPatchValue,
	)
	if err != nil {
		panic(
			"failed to serialize OAuth Cluster Config Patch to protobuf struct due to the following error: " + err.Error(),
		)
	}

	oAuthSidecarConfigPatchValueAsPbStruct, err := structpb.NewStruct(
		configpatch.GetOAuthSidecarConfigPatchValue(*scope),
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
