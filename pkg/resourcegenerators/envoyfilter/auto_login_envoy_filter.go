package envoyfilter

import (
	"strconv"

	"google.golang.org/protobuf/types/known/structpb"
	"istio.io/api/networking/v1alpha3"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	"github.com/kartverket/ztoperator/pkg/utilities"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v1alpha4.EnvoyFilter {
	if scope.IsMisconfigured() || scope.AuthPolicy.Spec.AutoLogin == nil ||
		!scope.AuthPolicy.Spec.AutoLogin.Enabled {
		return nil
	}

	idpAsParsedURL, err := utilities.GetParsedURL(scope.IdentityProviderUris.TokenURI)
	if err != nil {
		panic(
			"failed to get issuer hostname from token URI " + scope.IdentityProviderUris.TokenURI + " due to the following error: " + err.Error(),
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

	oAuthClusterConfigPatchValueAsPbStruct, err := structpb.NewStruct(
		oAuthClusterConfigPatchValue,
	)
	if err != nil {
		panic(
			"failed to serialize OAuth Cluster Config Patch to protobuf struct due to the following error: " + err.Error(),
		)
	}

	oAuthSidecarConfigPatchValueAsPbStruct, err := structpb.NewStruct(
		configpatch.GetOAuthSidecarConfigPatchValue(
			scope.IdentityProviderUris.TokenURI,
			scope.IdentityProviderUris.AuthorizationURI,
			scope.AutoLoginConfig.RedirectPath,
			scope.AutoLoginConfig.LogoutPath,
			scope.IdentityProviderUris.EndSessionURI,
			*scope.OAuthCredentials.ClientID,
			scope.AutoLoginConfig.Scopes,
			scope.AuthPolicy.Spec.AcceptedResources,
		),
	)
	if err != nil {
		panic(
			"failed to serialize OAuth Sidecar Config Patch to protobuf struct due to the following error: " + err.Error(),
		)
	}

	var configPatches []*v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch

	luaScriptConfigPatch, luaScriptConfigPatchErr := configpatch.GetLuaScriptConfigPatch(scope)
	if luaScriptConfigPatchErr != nil {
		panic(luaScriptConfigPatchErr.Error())
	}
	configPatches = append(configPatches, luaScriptConfigPatch)

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
