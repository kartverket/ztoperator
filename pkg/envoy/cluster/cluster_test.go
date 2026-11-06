package cluster_test

import (
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/kartverket/ztoperator/pkg/envoy/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testClusterName = "test-cluster"

func onlyEndpoint(t *testing.T, c *clusterv3.Cluster) *endpointv3.Endpoint {
	t.Helper()
	require.NotNil(t, c.GetLoadAssignment())
	assert.Equal(t, c.GetName(), c.GetLoadAssignment().GetClusterName())
	require.Len(t, c.GetLoadAssignment().GetEndpoints(), 1)
	lbEndpoints := c.GetLoadAssignment().GetEndpoints()[0].GetLbEndpoints()
	require.Len(t, lbEndpoints, 1)
	endpoint := lbEndpoints[0].GetEndpoint()
	require.NotNil(t, endpoint)
	return endpoint
}

func upstreamTLSContext(t *testing.T, c *clusterv3.Cluster) *tlsv3.UpstreamTlsContext {
	t.Helper()
	transportSocket := c.GetTransportSocket()
	require.NotNil(t, transportSocket, "expected a TLS transport socket")
	assert.Equal(t, "envoy.transport_sockets.tls", transportSocket.GetName())

	tlsContext := &tlsv3.UpstreamTlsContext{}
	require.NoError(t, transportSocket.GetTypedConfig().UnmarshalTo(tlsContext))
	return tlsContext
}

func TestFromURI_HttpHasNoTransportSocket(t *testing.T) {
	c, err := cluster.FromURI(testClusterName, "http://mock-oauth2.auth:8080/entraid/jwks")
	require.NoError(t, err)

	assert.Nil(t, c.GetTransportSocket(), "plain http cluster must not configure TLS")

	socketAddress := onlyEndpoint(t, c).GetAddress().GetSocketAddress()
	assert.Equal(t, "mock-oauth2.auth", socketAddress.GetAddress())
	assert.Equal(t, uint32(8080), socketAddress.GetPortValue())
}

func TestFromURI_HttpsConfiguresUpstreamTLS(t *testing.T) {
	c, err := cluster.FromURI(testClusterName, "https://login.example.com:8443/oauth2/token")
	require.NoError(t, err)

	tlsContext := upstreamTLSContext(t, c)
	assert.Equal(t, "login.example.com", tlsContext.GetSni())

	common := tlsContext.GetCommonTlsContext()
	require.NotNil(t, common)
	assert.Equal(t, tlsv3.TlsParameters_TLSv1_2, common.GetTlsParams().GetTlsMinimumProtocolVersion())
	assert.Equal(t, tlsv3.TlsParameters_TLSv1_3, common.GetTlsParams().GetTlsMaximumProtocolVersion())
	assert.Equal(t, []string{"http/1.1"}, common.GetAlpnProtocols())

	validation := common.GetValidationContext()
	require.NotNil(t, validation, "upstream certificate must be validated")
	assert.Equal(t, "/etc/ssl/certs/ca-certificates.crt", validation.GetTrustedCa().GetFilename())
	require.Len(t, validation.GetMatchTypedSubjectAltNames(), 1)
	san := validation.GetMatchTypedSubjectAltNames()[0]
	assert.Equal(t, tlsv3.SubjectAltNameMatcher_DNS, san.GetSanType())
	assert.Equal(t, "login.example.com", san.GetMatcher().GetExact())

	socketAddress := onlyEndpoint(t, c).GetAddress().GetSocketAddress()
	assert.Equal(t, "login.example.com", socketAddress.GetAddress())
	assert.Equal(t, uint32(8443), socketAddress.GetPortValue())
}

func TestFromURI_UsesDefaultPortForScheme(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		expectedPort uint32
		expectTLS    bool
	}{
		{name: "http defaults to 80", uri: "http://issuer.example.com/jwks", expectedPort: 80, expectTLS: false},
		{name: "https defaults to 443", uri: "https://issuer.example.com/jwks", expectedPort: 443, expectTLS: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := cluster.FromURI(testClusterName, test.uri)
			require.NoError(t, err)

			socketAddress := onlyEndpoint(t, c).GetAddress().GetSocketAddress()
			assert.Equal(t, "issuer.example.com", socketAddress.GetAddress())
			assert.Equal(t, test.expectedPort, socketAddress.GetPortValue())
			assert.Equal(t, test.expectTLS, c.GetTransportSocket() != nil)
		})
	}
}

func TestFromURI_HttpAndHttpsShareCommonClusterSettings(t *testing.T) {
	for _, uri := range []string{"http://issuer.example.com", "https://issuer.example.com"} {
		t.Run(uri, func(t *testing.T) {
			c, err := cluster.FromURI(testClusterName, uri)
			require.NoError(t, err)

			assert.Equal(t, testClusterName, c.GetName())
			assert.Equal(t, "envoy.cluster.dns", c.GetClusterType().GetName())
			assert.NotNil(t, c.GetConnectTimeout())
			assert.Contains(
				t,
				c.GetTypedExtensionProtocolOptions(),
				"envoy.extensions.upstreams.http.v3.HttpProtocolOptions",
			)

			endpoint := onlyEndpoint(t, c)
			assert.Equal(t, "issuer.example.com", endpoint.GetHostname())
			assert.Equal(t, corev3.SocketAddress_TCP, endpoint.GetAddress().GetSocketAddress().GetProtocol())

			assert.NoError(t, c.ValidateAll())
		})
	}
}

func TestFromURI_RejectsInvalidURIs(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{name: "unsupported scheme", uri: "ftp://issuer.example.com"},
		{name: "missing scheme", uri: "issuer.example.com/jwks"},
		{name: "empty", uri: ""},
		{name: "non-numeric port", uri: "https://issuer.example.com:abc"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := cluster.FromURI(testClusterName, test.uri)
			require.Error(t, err)
			assert.Nil(t, c)
		})
	}
}
