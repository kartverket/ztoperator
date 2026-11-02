package envoy

import (
	"errors"
	"slices"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	httpconnectionmanagerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/envoy/cluster"
	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter"
	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/kartverket/ztoperator/pkg/model"
	istionetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
)

const (
	JwksClusterName          = "jwks"
	TokenEndpointClusterName = "token"
)

type HttpFiltersAndClusters interface {
	GetHttpFilters() []*httpconnectionmanagerv3.HttpFilter
	GetClusters() []*clusterv3.Cluster
}

type AutoLoginFiltersAndClusters struct {
	Lua          *httpconnectionmanagerv3.HttpFilter
	OAuth2       *httpconnectionmanagerv3.HttpFilter
	TokenCluster *clusterv3.Cluster
}

func (a *AutoLoginFiltersAndClusters) GetHttpFilters() []*httpconnectionmanagerv3.HttpFilter {
	if a == nil {
		return []*httpconnectionmanagerv3.HttpFilter{}
	}
	if a.Lua == nil || a.OAuth2 == nil {
		panic("AutoLoginFiltersAndClusters.Lua or AutoLoginFiltersAndClusters.OAuth2 is nil, this should never happen")
	}
	return []*httpconnectionmanagerv3.HttpFilter{a.Lua, a.OAuth2}
}
func (a *AutoLoginFiltersAndClusters) GetClusters() []*clusterv3.Cluster {
	if a == nil {
		return []*clusterv3.Cluster{}
	}
	if a.TokenCluster == nil {
		panic("AutoLoginFiltersAndClusters.TokenCluster is nil, this should never happen")
	}
	return []*clusterv3.Cluster{a.TokenCluster}
}

func (a AutoLoginFiltersAndClusters) ToEnvoyFilterConfigPatches() (*[]*istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch, error) {
	configPatches := make([]*istionetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch, 0, 3)
	for _, httpFilter := range a.GetHttpFilters() {
		if configPatch, err := HttpFilterToEnvoyFilterConfigPatch(httpFilter); err != nil {
			return nil, err
		} else {
			configPatches = append(configPatches, configPatch)
		}
	}
	for _, autoLoginCluster := range a.GetClusters() {
		if configPatch, err := ClusterToEnvoyFilterConfigPatch(autoLoginCluster); err != nil {
			return nil, err
		} else {
			configPatches = append(configPatches, configPatch)
		}
	}

	return &configPatches, nil
}

type JwtAuthenticationFiltersAndClusters struct {
	JwtAuthn    *httpconnectionmanagerv3.HttpFilter
	JwksCluster *clusterv3.Cluster
}

func (j *JwtAuthenticationFiltersAndClusters) GetHttpFilters() []*httpconnectionmanagerv3.HttpFilter {
	if j == nil {
		return []*httpconnectionmanagerv3.HttpFilter{}
	}
	if j.JwtAuthn == nil {
		panic("JwtAuthenticationFiltersAndClusters.JwtAuthn is nil, this should never happen")
	}
	return []*httpconnectionmanagerv3.HttpFilter{j.JwtAuthn}
}
func (j *JwtAuthenticationFiltersAndClusters) GetClusters() []*clusterv3.Cluster {
	if j == nil {
		return []*clusterv3.Cluster{}
	}
	if j.JwksCluster == nil {
		panic("JwtAuthenticationFiltersAndClusters.JwksCluster is nil, this should never happen")
	}
	return []*clusterv3.Cluster{j.JwksCluster}
}

type HttpFilterConfig struct {
	AutoLoginHttpFiltersAndClusters        *AutoLoginFiltersAndClusters
	JwtAuthenticationHttpFilterAndClusters *JwtAuthenticationFiltersAndClusters
}

func (e *HttpFilterConfig) GetHttpFilters() []*httpconnectionmanagerv3.HttpFilter {
	return slices.Concat(
		e.AutoLoginHttpFiltersAndClusters.GetHttpFilters(),
		e.JwtAuthenticationHttpFilterAndClusters.GetHttpFilters(),
	)
}
func (e *HttpFilterConfig) GetClusters() []*clusterv3.Cluster {
	return slices.Concat(
		e.AutoLoginHttpFiltersAndClusters.GetClusters(),
		e.JwtAuthenticationHttpFilterAndClusters.GetClusters(),
	)
}

func GetEnvoyHttpFilterConfig(
	authPolicy v1alpha1.AuthPolicy,
	autoLoginConfig model.AutoLoginConfig,
	identityProviderUris model.IdentityProviderUris,
	clientID *string,
) (*HttpFilterConfig, error) {
	envoyHttpFilterConfig := HttpFilterConfig{}
	if !authPolicy.Spec.Enabled {
		return &envoyHttpFilterConfig, nil
	}

	if autoLoginConfig.Enabled {
		autoLoginFiltersAndClusters, err := GetAutoLoginFiltersAndClusters(
			authPolicy,
			autoLoginConfig,
			identityProviderUris,
			clientID,
		)
		if err != nil {
			return nil, err
		}
		envoyHttpFilterConfig.AutoLoginHttpFiltersAndClusters = autoLoginFiltersAndClusters
	}
	jwtAuthenticationFiltersAndClusters, err := GetJwtAuthenticationFiltersAndClusters(
		identityProviderUris.IssuerURI,
		identityProviderUris.JwksURI,
	)
	if err != nil {
		return nil, err
	}
	envoyHttpFilterConfig.JwtAuthenticationHttpFilterAndClusters = jwtAuthenticationFiltersAndClusters
	return &envoyHttpFilterConfig, nil
}

func GetAutoLoginFiltersAndClusters(
	authPolicy v1alpha1.AuthPolicy,
	autoLoginConfig model.AutoLoginConfig,
	identityProviderUris model.IdentityProviderUris,
	clientID *string,
) (*AutoLoginFiltersAndClusters, error) {
	if clientID == nil {
		return nil, errors.New("encountered client ID as nil when generating auto-login filters and clusters, this should never happen")
	}
	luaFilter, err := httpfilter.LuaHttpFilter(
		luascript.GenerateLuaScript(
			authPolicy,
			autoLoginConfig,
			identityProviderUris,
		),
	)
	if err != nil {
		return nil, err
	}
	oauth2Filter, err := httpfilter.OAuth2HttpFilter(
		autoLoginConfig,
		identityProviderUris,
		*clientID,
		TokenEndpointClusterName,
	)
	if err != nil {
		return nil, err
	}

	tokenEndpointCluster, err := cluster.FromURI(TokenEndpointClusterName, identityProviderUris.TokenURI)
	if err != nil {
		return nil, err
	}

	return &AutoLoginFiltersAndClusters{
		Lua:          luaFilter,
		OAuth2:       oauth2Filter,
		TokenCluster: tokenEndpointCluster,
	}, nil
}

func GetJwtAuthenticationFiltersAndClusters(
	issuerURI, jwksURI string,
) (*JwtAuthenticationFiltersAndClusters, error) {
	jwtAuthenticationFilter, err := httpfilter.JwtAuthenticationHttpFilter(
		issuerURI,
		jwksURI,
		JwksClusterName,
	)
	if err != nil {
		return nil, err
	}

	jwksCluster, err := cluster.FromURI(JwksClusterName, jwksURI)
	if err != nil {
		return nil, err
	}
	return &JwtAuthenticationFiltersAndClusters{
		JwtAuthn:    jwtAuthenticationFilter,
		JwksCluster: jwksCluster,
	}, nil
}
