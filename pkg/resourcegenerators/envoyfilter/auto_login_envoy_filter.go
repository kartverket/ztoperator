package envoyfilter

import (
	"github.com/kartverket/ztoperator/pkg/envoy"
	"github.com/kartverket/ztoperator/pkg/model"
	"istio.io/api/networking/v1alpha3"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *model.Scope, objectMeta v1.ObjectMeta) *v1alpha4.EnvoyFilter {
	if !scope.AuthPolicy.Spec.Enabled || scope.InvalidConfig || scope.AuthPolicy.Spec.AutoLogin == nil ||
		!scope.AuthPolicy.Spec.AutoLogin.Enabled {
		return nil
	}

	autoLoginFiltersAndClusters, err := envoy.GetAutoLoginFiltersAndClusters(
		scope.AuthPolicy,
		scope.AutoLoginConfig,
		scope.IdentityProviderUris,
		scope.OAuthCredentials.ClientID,
	)
	if err != nil {
		panic("failed to get envoy http filter config: " + err.Error())
	}

	configPatches, err := autoLoginFiltersAndClusters.ToEnvoyFilterConfigPatches()
	if err != nil {
		panic("failed to convert envoy HTTP filters and clusters to EnvoyFilter configPatches: " + err.Error())
	}

	return &v1alpha4.EnvoyFilter{
		ObjectMeta: objectMeta,
		Spec: v1alpha3.EnvoyFilter{
			ConfigPatches: *configPatches,
			WorkloadSelector: &v1alpha3.WorkloadSelector{
				Labels: scope.AuthPolicy.Spec.Selector.MatchLabels,
			},
		},
	}
}
