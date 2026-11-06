package envoy_test

import (
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	jwtauthnv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	oauth2v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/oauth2/v3"
	httpconnectionmanagerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/envoy"
	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter"
	"github.com/kartverket/ztoperator/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIssuerURI        = "https://issuer.example.com"
	testJwksURI          = "https://jwks.example.com/keys"
	testTokenURI         = "https://token.example.com/oauth2/token"
	testAuthorizationURI = "https://issuer.example.com/authorize"
	testClientID         = "my-client-id"
)

var testAudiences = []string{"my-audience"}

func testAuthPolicy(enabled bool) v1alpha1.AuthPolicy {
	return v1alpha1.AuthPolicy{
		Name: "test-app", Namespace: "test-namespace",
		Spec: v1alpha1.AuthPolicySpec{Enabled: enabled},
	}
}

func testIdentityProviderUris() model.IdentityProviderUris {
	return model.IdentityProviderUris{
		IssuerURI:        testIssuerURI,
		JwksURI:          testJwksURI,
		TokenURI:         testTokenURI,
		AuthorizationURI: testAuthorizationURI,
		EndSessionURI:    new("https://issuer.example.com/logout"),
	}
}

func autoLoginEnabledConfig() model.AutoLoginConfig {
	return model.AutoLoginConfig{
		Enabled:      true,
		RedirectPath: "/oauth2/callback",
		LogoutPath:   "/logout",
		Scopes:       []string{"openid"},
	}
}

func filterNames(filters []*httpconnectionmanagerv3.HttpFilter) []string {
	result := make([]string, len(filters))
	for i, f := range filters {
		result[i] = f.GetName()
	}
	return result
}

func clusterNames(clusters []*clusterv3.Cluster) []string {
	result := make([]string, len(clusters))
	for i, c := range clusters {
		result[i] = c.GetName()
	}
	return result
}

func findFilter(
	t *testing.T,
	filters []*httpconnectionmanagerv3.HttpFilter,
	name string,
) *httpconnectionmanagerv3.HttpFilter {
	t.Helper()
	for _, f := range filters {
		if f.GetName() == name {
			return f
		}
	}
	require.Failf(t, "filter not found", "expected filter %q in %v", name, filterNames(filters))
	return nil
}

func findCluster(t *testing.T, clusters []*clusterv3.Cluster, name string) *clusterv3.Cluster {
	t.Helper()
	for _, c := range clusters {
		if c.GetName() == name {
			return c
		}
	}
	require.Failf(t, "cluster not found", "expected cluster %q in %v", name, clusterNames(clusters))
	return nil
}

func clusterEndpointHost(t *testing.T, c *clusterv3.Cluster) string {
	t.Helper()
	endpoints := c.GetLoadAssignment().GetEndpoints()
	require.Len(t, endpoints, 1)
	require.Len(t, endpoints[0].GetLbEndpoints(), 1)
	return endpoints[0].GetLbEndpoints()[0].GetEndpoint().GetAddress().GetSocketAddress().GetAddress()
}

func TestGetEnvoyHttpFilterConfig_ReturnsNothingWhenAuthPolicyDisabled(t *testing.T) {
	for _, autoLogin := range []bool{false, true} {
		t.Run(map[bool]string{false: "auto login disabled", true: "auto login enabled"}[autoLogin], func(t *testing.T) {
			autoLoginConfig := model.AutoLoginConfig{}
			if autoLogin {
				autoLoginConfig = autoLoginEnabledConfig()
			}

			config, err := envoy.GetEnvoyHttpFilterConfig(
				testAuthPolicy(false),
				autoLoginConfig,
				testIdentityProviderUris(),
				testAudiences,
				new(testClientID),
			)
			require.NoError(t, err)

			assert.Nil(t, config.AutoLoginHttpFiltersAndClusters)
			assert.Empty(t, config.GetHttpFilters())
			assert.Empty(t, config.GetClusters())
		})
	}
}

func TestGetEnvoyHttpFilterConfig_AutoLoginDisabled(t *testing.T) {
	config, err := envoy.GetEnvoyHttpFilterConfig(
		testAuthPolicy(true),
		model.AutoLoginConfig{Enabled: false},
		testIdentityProviderUris(),
		testAudiences,
		nil, // client ID is not resolved when auto login is disabled
	)
	require.NoError(t, err)

	assert.Nil(t, config.AutoLoginHttpFiltersAndClusters, "auto login group must be nil when auto login is disabled")
	assert.Empty(t, config.AutoLoginHttpFiltersAndClusters.GetHttpFilters())
	assert.Empty(t, config.AutoLoginHttpFiltersAndClusters.GetClusters())

	filters := filterNames(config.GetHttpFilters())
	assert.Equal(t, []string{httpfilter.EnvoyExtensionHttpFilterJwtAuthentication}, filters)
	assert.NotContains(t, filters, httpfilter.EnvoyExtensionHttpFilterLua)
	assert.NotContains(t, filters, httpfilter.EnvoyExtensionHttpFilterOAuth2)

	clusters := clusterNames(config.GetClusters())
	assert.Equal(t, []string{envoy.JwksClusterName}, clusters)
	assert.NotContains(t, clusters, envoy.TokenEndpointClusterName)
}

func TestGetEnvoyHttpFilterConfig_AutoLoginEnabled(t *testing.T) {
	config, err := envoy.GetEnvoyHttpFilterConfig(
		testAuthPolicy(true),
		autoLoginEnabledConfig(),
		testIdentityProviderUris(),
		testAudiences,
		new(testClientID),
	)
	require.NoError(t, err)

	// Order matters: Lua must run before OAuth2 (it sets pass-through/deny-redirect headers that the
	// OAuth2 filter matches on), and OAuth2 must run before jwt_authn (it injects the bearer token
	// that jwt_authn validates).
	assert.Equal(t,
		[]string{
			httpfilter.EnvoyExtensionHttpFilterLua,
			httpfilter.EnvoyExtensionHttpFilterOAuth2,
			httpfilter.EnvoyExtensionHttpFilterJwtAuthentication,
		},
		filterNames(config.GetHttpFilters()),
	)

	assert.ElementsMatch(t,
		[]string{envoy.TokenEndpointClusterName, envoy.JwksClusterName},
		clusterNames(config.GetClusters()),
	)
}

func TestGetEnvoyHttpFilterConfig_GroupsFiltersAndClusters(t *testing.T) {
	config, err := envoy.GetEnvoyHttpFilterConfig(
		testAuthPolicy(true),
		autoLoginEnabledConfig(),
		testIdentityProviderUris(),
		testAudiences,
		new(testClientID),
	)
	require.NoError(t, err)

	autoLogin := config.AutoLoginHttpFiltersAndClusters
	require.NotNil(t, autoLogin)
	assert.Equal(t, httpfilter.EnvoyExtensionHttpFilterLua, autoLogin.Lua.GetName())
	assert.Equal(t, httpfilter.EnvoyExtensionHttpFilterOAuth2, autoLogin.OAuth2.GetName())
	assert.Equal(t, envoy.TokenEndpointClusterName, autoLogin.TokenCluster.GetName())
	assert.Equal(t,
		[]string{httpfilter.EnvoyExtensionHttpFilterLua, httpfilter.EnvoyExtensionHttpFilterOAuth2},
		filterNames(autoLogin.GetHttpFilters()),
	)
	assert.Equal(t, []string{envoy.TokenEndpointClusterName}, clusterNames(autoLogin.GetClusters()))

	jwtAuthn := config.JwtAuthenticationHttpFilterAndClusters
	assert.Equal(t, httpfilter.EnvoyExtensionHttpFilterJwtAuthentication, jwtAuthn.JwtAuthn.GetName())
	assert.Equal(t, envoy.JwksClusterName, jwtAuthn.JwksCluster.GetName())
	assert.Equal(t,
		[]string{httpfilter.EnvoyExtensionHttpFilterJwtAuthentication},
		filterNames(jwtAuthn.GetHttpFilters()),
	)
	assert.Equal(t, []string{envoy.JwksClusterName}, clusterNames(jwtAuthn.GetClusters()))
}

func TestGetEnvoyHttpFilterConfig_ContainsNoNilFiltersOrClusters(t *testing.T) {
	for _, authPolicyEnabled := range []bool{false, true} {
		for _, autoLoginConfig := range []model.AutoLoginConfig{{Enabled: false}, autoLoginEnabledConfig()} {
			config, err := envoy.GetEnvoyHttpFilterConfig(
				testAuthPolicy(authPolicyEnabled),
				autoLoginConfig,
				testIdentityProviderUris(),
				testAudiences,
				new(testClientID),
			)
			require.NoError(t, err)
			assert.NotContains(t, config.GetHttpFilters(), (*httpconnectionmanagerv3.HttpFilter)(nil))
			assert.NotContains(t, config.GetClusters(), (*clusterv3.Cluster)(nil))
		}
	}
}

func TestGetEnvoyHttpFilterConfig_NeverContainsRouterFilter(t *testing.T) {
	// The router filter is appended by the listener and must always be last, so it must not be part of this list.
	for _, autoLoginConfig := range []model.AutoLoginConfig{{Enabled: false}, autoLoginEnabledConfig()} {
		config, err := envoy.GetEnvoyHttpFilterConfig(
			testAuthPolicy(true),
			autoLoginConfig,
			testIdentityProviderUris(),
			testAudiences,
			new(testClientID),
		)
		require.NoError(t, err)
		assert.NotContains(t, filterNames(config.GetHttpFilters()), httpfilter.EnvoyExtensionHttpFilterRouter)
	}
}

func TestGetEnvoyHttpFilterConfigFilters_ReferenceExistingClusters(t *testing.T) {
	config, err := envoy.GetEnvoyHttpFilterConfig(
		testAuthPolicy(true),
		autoLoginEnabledConfig(),
		testIdentityProviderUris(),
		testAudiences,
		new(testClientID),
	)
	require.NoError(t, err)
	httpFilters := config.GetHttpFilters()
	clusters := config.GetClusters()

	// jwt_authn fetches JWKS through the jwks cluster, which must point at the JWKS host.
	jwtAuthn := &jwtauthnv3.JwtAuthentication{}
	require.NoError(t, findFilter(t, httpFilters, httpfilter.EnvoyExtensionHttpFilterJwtAuthentication).
		GetTypedConfig().UnmarshalTo(jwtAuthn))
	require.NotEmpty(t, jwtAuthn.GetProviders())
	for _, provider := range jwtAuthn.GetProviders() {
		assert.Equal(t, testIssuerURI, provider.GetIssuer())
		assert.Equal(t, testAudiences, provider.GetAudiences())
		assert.Equal(t, testJwksURI, provider.GetRemoteJwks().GetHttpUri().GetUri())
		assert.Equal(t, envoy.JwksClusterName, provider.GetRemoteJwks().GetHttpUri().GetCluster())
	}
	assert.Equal(t, "jwks.example.com", clusterEndpointHost(t, findCluster(t, clusters, envoy.JwksClusterName)))

	// OAuth2 exchanges the code through the token cluster, which must point at the token endpoint host.
	oauth2 := &oauth2v3.OAuth2{}
	require.NoError(t, findFilter(t, httpFilters, httpfilter.EnvoyExtensionHttpFilterOAuth2).
		GetTypedConfig().UnmarshalTo(oauth2))
	assert.Equal(t, testTokenURI, oauth2.GetConfig().GetTokenEndpoint().GetUri())
	assert.Equal(t, envoy.TokenEndpointClusterName, oauth2.GetConfig().GetTokenEndpoint().GetCluster())
	assert.Equal(t, testAuthorizationURI, oauth2.GetConfig().GetAuthorizationEndpoint())
	assert.Equal(t, testClientID, oauth2.GetConfig().GetCredentials().GetClientId())
	assert.Equal(t,
		"token.example.com",
		clusterEndpointHost(t, findCluster(t, clusters, envoy.TokenEndpointClusterName)),
	)
}

func TestGetEnvoyHttpFilterConfig_FailsWhenAutoLoginEnabledWithoutClientID(t *testing.T) {
	config, err := envoy.GetEnvoyHttpFilterConfig(
		testAuthPolicy(true),
		autoLoginEnabledConfig(),
		testIdentityProviderUris(),
		testAudiences,
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client ID")
	assert.Nil(t, config)
}
