package envoyfilter

import (
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/config_patch"
	"github.com/kartverket/ztoperator/pkg/utils"
	"google.golang.org/protobuf/types/known/structpb"
	"istio.io/api/networking/v1alpha3"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v1alpha4.EnvoyFilter {
	if !scope.AuthPolicy.Spec.Enabled || scope.AuthPolicy.Spec.AutoLogin == nil ||
		!scope.AuthPolicy.Spec.AutoLogin.Enabled ||
		scope.InvalidConfig {
		return nil
	}

	issuerHostName, err := utils.GetHostname(scope.IdentityProviderUris.IssuerUri)
	if err != nil {
		panic(
			"failed to get issuer hostname from issuer URI " + scope.IdentityProviderUris.IssuerUri + " due to the following error: " + err.Error(),
		)
	}
	oAuthClusterConfigPatchValueAsPbStruct, err := structpb.NewStruct(
		config_patch.GetOAuthClusterConfigPatchValue(*issuerHostName),
	)
	if err != nil {
		panic(
			"failed to serialize OAuth Cluster Config Patch to protobuf struct due to the following error: " + err.Error(),
		)
	}

	oAuthSidecarConfigPatchValueAsPbStruct, err := structpb.NewStruct(
		config_patch.GetOAuthSidecarConfigPatchValue(
			scope.IdentityProviderUris.TokenUri,
			scope.IdentityProviderUris.AuthorizationUri,
			scope.AutoLoginConfig.RedirectPath,
			scope.AutoLoginConfig.LogoutPath,
			*scope.OAuthCredentials.ClientId,
			scope.AutoLoginConfig.Scopes,
			scope.AuthPolicy.Spec.AcceptedResources,
			scope.AuthPolicy.Spec.IgnoreAuthRules,
			scope.AutoLoginConfig.LoginPath,
		),
	)
	if err != nil {
		panic(
			"failed to serialize OAuth Sidecar Config Patch to protobuf struct due to the following error: " + err.Error(),
		)
	}

	var configPatches []*v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch

	if scope.AutoLoginConfig.LoginPath == nil && scope.AuthPolicy.Spec.IgnoreAuthRules != nil {
		luaScript, structPbErr := structpb.NewStruct(config_patch.GetLuaScript())
		if structPbErr != nil {
			panic(
				"failed to serialize Custom Lua Script to protobuf struct due to the following error: " + structPbErr.Error(),
			)
		}
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
				Value:     luaScript,
			},
		})
	}

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
