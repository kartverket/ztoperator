package envoyfilter_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"istio.io/api/networking/v1alpha3"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetDesired_ReturnsNil_WhenDisabled(t *testing.T) {
	scope := defaultScope()
	scope.AuthPolicy.Spec.Enabled = false
	assert.Nil(t, envoyfilter.GetDesired(&scope, defaultObjectMeta()))
}

func TestGetDesired_ReturnsNil_WhenAutoLoginNil(t *testing.T) {
	scope := defaultScope()
	scope.AuthPolicy.Spec.AutoLogin = nil
	assert.Nil(t, envoyfilter.GetDesired(&scope, defaultObjectMeta()))
}

func TestGetDesired_ReturnsNil_WhenAutoLoginDisabled(t *testing.T) {
	scope := defaultScope()
	scope.AuthPolicy.Spec.AutoLogin = &ztoperatorv1alpha1.AutoLogin{Enabled: false}
	assert.Nil(t, envoyfilter.GetDesired(&scope, defaultObjectMeta()))
}

func TestGetDesired_ReturnsNil_WhenInvalidConfig(t *testing.T) {
	scope := defaultScope()
	scope.InvalidConfig = true
	assert.Nil(t, envoyfilter.GetDesired(&scope, defaultObjectMeta()))
}

func TestGetDesired_ObjectMetaIsPreserved(t *testing.T) {
	scope := defaultScope()
	name := "auth-policy-login"
	namespace := "mynamespace"
	labels := map[string]string{"team": "x"}
	om := metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}

	ef := envoyfilter.GetDesired(&scope, om)

	require.NotNil(t, ef)
	assert.Equal(t, name, ef.Name)
	assert.Equal(t, namespace, ef.Namespace)
	assert.Equal(t, labels, ef.Labels)
}

func TestGetDesired_WorkloadSelectorMatchesAuthPolicySelector(t *testing.T) {
	scope := defaultScope()

	ef := envoyfilter.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ef)
	assert.Equal(t, scope.AuthPolicy.Spec.Selector.MatchLabels, ef.Spec.WorkloadSelector.Labels)
}

func TestGetDesired_ProducesExactlyThreeConfigPatches(t *testing.T) {
	scope := defaultScope()

	ef := envoyfilter.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ef)
	assert.Len(t, ef.Spec.ConfigPatches, 3)
}

// patch[0]: Lua filter — inserted before jwt_authn in the inbound sidecar HTTP chain.
func TestGetDesired_LuaPatch_ApplyToAndOperation(t *testing.T) {
	ef := envoyfilter.GetDesired(helperfunctions.Ptr(defaultScope()), defaultObjectMeta())

	require.NotNil(t, ef)
	p := ef.Spec.ConfigPatches[0]
	assert.Equal(t, v1alpha3.EnvoyFilter_HTTP_FILTER, p.ApplyTo)
	assert.Equal(t, v1alpha3.EnvoyFilter_Patch_INSERT_BEFORE, p.Patch.Operation)
	assert.Equal(t, v1alpha3.EnvoyFilter_SIDECAR_INBOUND, p.Match.Context)

	filterName := p.Match.GetListener().GetFilterChain().GetFilter().GetName()
	assert.Equal(t, "envoy.filters.network.http_connection_manager", filterName)
}

// patch[1]: OAuth cluster — added to the cluster list.
func TestGetDesired_ClusterPatch_ApplyToAndOperation(t *testing.T) {
	ef := envoyfilter.GetDesired(helperfunctions.Ptr(defaultScope()), defaultObjectMeta())

	require.NotNil(t, ef)
	p := ef.Spec.ConfigPatches[1]
	assert.Equal(t, v1alpha3.EnvoyFilter_CLUSTER, p.ApplyTo)
	assert.Equal(t, v1alpha3.EnvoyFilter_Patch_ADD, p.Patch.Operation)
}

// patch[2]: OAuth2 sidecar filter — inserted before jwt_authn in the inbound sidecar HTTP chain.
func TestGetDesired_OAuthSidecarPatch_ApplyToAndOperation(t *testing.T) {
	ef := envoyfilter.GetDesired(helperfunctions.Ptr(defaultScope()), defaultObjectMeta())

	require.NotNil(t, ef)
	p := ef.Spec.ConfigPatches[2]
	assert.Equal(t, v1alpha3.EnvoyFilter_HTTP_FILTER, p.ApplyTo)
	assert.Equal(t, v1alpha3.EnvoyFilter_Patch_INSERT_BEFORE, p.Patch.Operation)
	assert.Equal(t, v1alpha3.EnvoyFilter_SIDECAR_INBOUND, p.Match.Context)

	filterName := p.Match.GetListener().GetFilterChain().GetFilter().GetName()
	assert.Equal(t, "envoy.filters.network.http_connection_manager", filterName)

	subFilterName := p.Match.GetListener().GetFilterChain().GetFilter().GetSubFilter().GetName()
	assert.Equal(t, "envoy.filters.http.jwt_authn", subFilterName)
}

func TestGetDesired_InternalIdP_ClusterPatchHasNoTLS(t *testing.T) {
	scope := defaultScope() // token URI has port 8080

	ef := envoyfilter.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ef)
	clusterValue := ef.Spec.ConfigPatches[1].Patch.Value.AsMap()
	_, hasTransportSocket := clusterValue["transport_socket"]
	assert.False(t, hasTransportSocket, "internal IdP cluster should not have TLS transport_socket")
}

func TestGetDesired_ExternalIdP_ClusterPatchHasTLS(t *testing.T) {
	scope := defaultScope()
	scope.IdentityProviderUris.TokenURI = "https://login.microsoftonline.com/tenant/oauth2/v2.0/token"

	ef := envoyfilter.GetDesired(&scope, defaultObjectMeta())

	require.NotNil(t, ef)
	clusterValue := ef.Spec.ConfigPatches[1].Patch.Value.AsMap()
	ts, ok := clusterValue["transport_socket"].(map[string]interface{})
	require.True(t, ok, "external IdP cluster must have transport_socket")
	assert.Equal(t, "envoy.transport_sockets.tls", ts["name"])
}

func defaultScope() state.Scope {
	clientID := "entraid_server"
	endSession := "http://mock-oauth2.auth:8080/entraid/endsession"
	return state.Scope{
		AuthPolicy: ztoperatorv1alpha1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "auth-policy", Namespace: "default"},
			Spec: ztoperatorv1alpha1.AuthPolicySpec{
				Enabled: true,
				Selector: ztoperatorv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{"app": "application"},
				},
				AutoLogin: &ztoperatorv1alpha1.AutoLogin{Enabled: true},
			},
		},
		OAuthCredentials: state.OAuthCredentials{
			ClientID: &clientID,
		},
		IdentityProviderUris: state.IdentityProviderUris{
			IssuerURI:        "http://mock-oauth2.auth:8080/entraid",
			TokenURI:         "http://mock-oauth2.auth:8080/entraid/token",
			AuthorizationURI: "http://mock-oauth2.auth:8080/entraid/authorize",
			EndSessionURI:    &endSession,
		},
		AutoLoginConfig: state.AutoLoginConfig{
			Enabled:      true,
			RedirectPath: "/oauth2/callback",
			LogoutPath:   "/logout",
			Scopes:       []string{"openid"},
			LuaScriptConfig: state.LuaScriptConfig{
				LuaScript: "-- generated lua",
			},
			EnvoySecretName: "auth-policy-envoy-secret",
		},
	}
}

func defaultObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: "auth-policy-login", Namespace: "default"}
}
