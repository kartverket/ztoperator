package envoymodel_test

import (
	"os"
	"testing"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	httpconnectionmanagerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/constants"
	"github.com/kartverket/ztoperator/pkg/envoy"
	"github.com/kartverket/ztoperator/pkg/envoy/cluster"
	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter"
	"github.com/kartverket/ztoperator/pkg/model"
	envoymodel "github.com/kartverket/ztoperator/pkg/model/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testEnvoyPort      = 15080
	testEnvoyProbePort = 15081
	testClientID       = "my-client-id"
)

// TestMain loads the operator config, since the listeners read the Envoy ports from it.
func TestMain(m *testing.M) {
	_ = os.Setenv("ZTOPERATOR_ENVOY_PORT", "15080")
	_ = os.Setenv("ZTOPERATOR_ENVOY_PROBE_PORT", "15081")
	_ = os.Setenv("ZTOPERATOR_ALLOWED_OAUTH_PROVIDERS", `{
		"test": {
			"wellKnownURI": "https://issuer.example.com/.well-known/openid-configuration",
			"issuer": "https://issuer.example.com",
			"tokenEndpoint": "https://token.example.com/token",
			"jwksURI": "https://jwks.example.com/keys"
		}
	}`)
	if err := config.Load(); err != nil {
		panic("failed to load config for tests: " + err.Error())
	}
	os.Exit(m.Run())
}

func testAuthPolicy(enabled bool) v1alpha1.AuthPolicy {
	return v1alpha1.AuthPolicy{
		Name: "my-app", Namespace: "my-namespace",
		Spec: v1alpha1.AuthPolicySpec{Enabled: enabled},
	}
}

func testIdentityProviderUris() model.IdentityProviderUris {
	return model.IdentityProviderUris{
		IssuerURI:        "https://issuer.example.com",
		JwksURI:          "https://jwks.example.com/keys",
		TokenURI:         "https://token.example.com/token",
		AuthorizationURI: "https://issuer.example.com/authorize",
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

func upstreamCluster(t *testing.T) *clusterv3.Cluster {
	t.Helper()
	c, err := cluster.FromURI(constants.EnvoyBootstrapUpstreamClusterName, "http://127.0.0.1:8080")
	require.NoError(t, err)
	return c
}

func buildBootstrap(t *testing.T, autoLoginConfig model.AutoLoginConfig) *bootstrapv3.Bootstrap {
	t.Helper()
	bootstrap, err := envoymodel.ToEnvoyBootstrap(
		testAuthPolicy(true),
		autoLoginConfig,
		testIdentityProviderUris(),
		upstreamCluster(t),
		new(testClientID),
	)
	require.NoError(t, err)
	require.NotNil(t, bootstrap)
	return bootstrap
}

func findListener(t *testing.T, bootstrap *bootstrapv3.Bootstrap, name string) *listenerv3.Listener {
	t.Helper()
	for _, l := range bootstrap.GetStaticResources().GetListeners() {
		if l.GetName() == name {
			return l
		}
	}
	require.Failf(t, "listener not found", "expected listener %q", name)
	return nil
}

func httpConnectionManager(t *testing.T, listener *listenerv3.Listener) *httpconnectionmanagerv3.HttpConnectionManager {
	t.Helper()
	require.Len(t, listener.GetFilterChains(), 1)
	filters := listener.GetFilterChains()[0].GetFilters()
	require.Len(t, filters, 1)
	assert.Equal(t, envoymodel.EnvoyExtensionHttpConnectionManagerName, filters[0].GetName())

	hcm := &httpconnectionmanagerv3.HttpConnectionManager{}
	require.NoError(t, filters[0].GetTypedConfig().UnmarshalTo(hcm))
	return hcm
}

func httpFilterNames(hcm *httpconnectionmanagerv3.HttpConnectionManager) []string {
	names := make([]string, 0, len(hcm.GetHttpFilters()))
	for _, f := range hcm.GetHttpFilters() {
		names = append(names, f.GetName())
	}
	return names
}

func findHttpFilter(
	t *testing.T,
	hcm *httpconnectionmanagerv3.HttpConnectionManager,
	name string,
) *httpconnectionmanagerv3.HttpFilter {
	t.Helper()
	for _, f := range hcm.GetHttpFilters() {
		if f.GetName() == name {
			return f
		}
	}
	require.Failf(t, "http filter not found", "expected filter %q in %v", name, httpFilterNames(hcm))
	return nil
}

func clusterNames(bootstrap *bootstrapv3.Bootstrap) []string {
	clusters := bootstrap.GetStaticResources().GetClusters()
	names := make([]string, 0, len(clusters))
	for _, c := range clusters {
		names = append(names, c.GetName())
	}
	return names
}

func ingressRouteCluster(t *testing.T, hcm *httpconnectionmanagerv3.HttpConnectionManager) string {
	t.Helper()
	virtualHosts := hcm.GetRouteConfig().GetVirtualHosts()
	require.Len(t, virtualHosts, 1)
	require.Len(t, virtualHosts[0].GetRoutes(), 1)
	route := virtualHosts[0].GetRoutes()[0]
	assert.Equal(t, "/", route.GetMatch().GetPrefix())
	return route.GetRoute().GetCluster()
}

func assertListenerAddress(t *testing.T, listener *listenerv3.Listener, port uint32) {
	t.Helper()
	socketAddress := listener.GetAddress().GetSocketAddress()
	assert.Equal(t, envoymodel.AnyAddress, socketAddress.GetAddress())
	assert.Equal(t, port, socketAddress.GetPortValue())
	assert.Equal(t, corev3.SocketAddress_TCP, socketAddress.GetProtocol())
}

func TestToEnvoyBootstrap_SetsNodeIdAndCluster(t *testing.T) {
	bootstrap := buildBootstrap(t, model.AutoLoginConfig{})

	// Envoy refuses to start SDS (used for the OAuth2 secrets) without both node id and cluster.
	assert.Equal(t, "my-app.my-namespace", bootstrap.GetNode().GetId())
	assert.Equal(t, "my-app.my-namespace", bootstrap.GetNode().GetCluster())
}

func TestToEnvoyBootstrap_HasIngressAndProbeListeners(t *testing.T) {
	bootstrap := buildBootstrap(t, model.AutoLoginConfig{})

	listeners := bootstrap.GetStaticResources().GetListeners()
	require.Len(t, listeners, 2)

	assertListenerAddress(t, findListener(t, bootstrap, envoymodel.IngressListenerName), testEnvoyPort)
	assertListenerAddress(t, findListener(t, bootstrap, envoymodel.ProbeListenerName), testEnvoyProbePort)
}

func TestToEnvoyBootstrap_IngressFiltersWithAutoLoginDisabledHasExpectedHttpFilters(t *testing.T) {
	bootstrap := buildBootstrap(t, model.AutoLoginConfig{Enabled: false})
	hcm := httpConnectionManager(t, findListener(t, bootstrap, envoymodel.IngressListenerName))

	assert.Equal(
		t,
		[]string{
			httpfilter.EnvoyExtensionHttpFilterJwtAuthentication, // TODO: Verify order
			httpfilter.EnvoyExtensionHttpFilterRouter,
		},
		httpFilterNames(hcm),
	)
}

func TestToEnvoyBootstrap_IngressFiltersWithAutoLoginEnabledHasExpectedHttpFilters(t *testing.T) {
	bootstrap := buildBootstrap(t, autoLoginEnabledConfig())
	hcm := httpConnectionManager(t, findListener(t, bootstrap, envoymodel.IngressListenerName))

	assert.Equal(t,
		[]string{
			httpfilter.EnvoyExtensionHttpFilterLua,
			httpfilter.EnvoyExtensionHttpFilterOAuth2,
			httpfilter.EnvoyExtensionHttpFilterJwtAuthentication,
			httpfilter.EnvoyExtensionHttpFilterRouter,
		},
		httpFilterNames(hcm),
	)
}

func TestToEnvoyBootstrap_IngressRoutesToUpstreamCluster(t *testing.T) {
	bootstrap := buildBootstrap(t, autoLoginEnabledConfig())
	hcm := httpConnectionManager(t, findListener(t, bootstrap, envoymodel.IngressListenerName))

	assert.Equal(t, constants.EnvoyBootstrapUpstreamClusterName, ingressRouteCluster(t, hcm))
}

func TestToEnvoyBootstrap_ClustersWithAutoLoginDisabled(t *testing.T) {
	bootstrap := buildBootstrap(t, model.AutoLoginConfig{Enabled: false})

	assert.ElementsMatch(t,
		[]string{envoy.JwksClusterName, constants.EnvoyBootstrapUpstreamClusterName},
		clusterNames(bootstrap),
	)
	assert.NotContains(t, clusterNames(bootstrap), envoy.TokenEndpointClusterName)
}

func TestToEnvoyBootstrap_ClustersWithAutoLoginEnabled(t *testing.T) {
	bootstrap := buildBootstrap(t, autoLoginEnabledConfig())

	assert.ElementsMatch(t,
		[]string{envoy.TokenEndpointClusterName, envoy.JwksClusterName, constants.EnvoyBootstrapUpstreamClusterName},
		clusterNames(bootstrap),
	)
}

func TestToEnvoyBootstrap_DisabledAuthPolicyOnlyProxiesToUpstream(t *testing.T) {
	bootstrap, err := envoymodel.ToEnvoyBootstrap(
		testAuthPolicy(false),
		autoLoginEnabledConfig(),
		testIdentityProviderUris(),
		upstreamCluster(t),
		new(testClientID),
	)
	require.NoError(t, err)

	hcm := httpConnectionManager(t, findListener(t, bootstrap, envoymodel.IngressListenerName))
	assert.Equal(t, []string{httpfilter.EnvoyExtensionHttpFilterRouter}, httpFilterNames(hcm))
	assert.Equal(t, []string{constants.EnvoyBootstrapUpstreamClusterName}, clusterNames(bootstrap))
}
