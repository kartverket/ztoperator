package envoymodel

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	healthcheckv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/health_check/v3"
	httpconnectionmanagerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/constants"
	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	EnvoyExtensionHttpFilterHealthCheck = "envoy.filters.http.health_check"

	ProbeListenerName = "probe"
	ProbeReadyPath    = "/ready"
)

func listenerFromHttpConnectionManager(
	name string,
	port int32,
	httpConnectionManager *httpconnectionmanagerv3.HttpConnectionManager,
) (*listenerv3.Listener, error) {
	typedConfig, err := anypb.New(httpConnectionManager)
	if err != nil {
		return nil, fmt.Errorf("marshal HTTP connection manager for listener %s: %w", name, err)
	}

	return &listenerv3.Listener{
		Name: name,
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Protocol:      corev3.SocketAddress_TCP,
					Address:       AnyAddress,
					PortSpecifier: &corev3.SocketAddress_PortValue{PortValue: uint32(port)},
				},
			},
		},
		FilterChains: []*listenerv3.FilterChain{
			{
				Filters: []*listenerv3.Filter{
					{
						Name:       EnvoyExtensionHttpConnectionManagerName,
						ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: typedConfig},
					},
				},
			},
		},
	}, nil
}

// ingressListener routes all traffic on the Envoy port, after the given filters, to the application cluster.
func ingressListener(filters ...*httpconnectionmanagerv3.HttpFilter) (*listenerv3.Listener, error) {
	router, err := httpfilter.RouterFilter()
	if err != nil {
		return nil, err
	}

	return listenerFromHttpConnectionManager(
		IngressListenerName,
		config.Get().EnvoyPort,
		&httpconnectionmanagerv3.HttpConnectionManager{
			StatPrefix: IngressListenerName,
			HttpFilters: append(
				append([]*httpconnectionmanagerv3.HttpFilter{}, filters...),
				router,
			),
			RouteSpecifier: &httpconnectionmanagerv3.HttpConnectionManager_RouteConfig{
				RouteConfig: &routev3.RouteConfiguration{
					Name: IngressListenerName,
					VirtualHosts: []*routev3.VirtualHost{
						{
							Name:    IngressListenerName,
							Domains: []string{"*"},
							Routes: []*routev3.Route{
								{
									Match: &routev3.RouteMatch{
										PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: "/"},
									},
									Action: &routev3.Route_Route{
										Route: &routev3.RouteAction{
											ClusterSpecifier: &routev3.RouteAction_Cluster{Cluster: constants.EnvoyBootstrapUpstreamClusterName},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	)
}

// probeListener serves the readiness endpoint on the probe port, since the admin interface only listens on loopback.
// The health check filter answers 200 until Envoy is draining or failing health checks, then 503.
func probeListener() (*listenerv3.Listener, error) {
	healthCheck, err := anypb.New(&healthcheckv3.HealthCheck{
		PassThroughMode: wrapperspb.Bool(false),
		Headers: []*routev3.HeaderMatcher{
			{
				Name: ":path",
				HeaderMatchSpecifier: &routev3.HeaderMatcher_StringMatch{
					StringMatch: exactMatcher(ProbeReadyPath),
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal health check filter: %w", err)
	}

	router, err := httpfilter.RouterFilter()
	if err != nil {
		return nil, err
	}

	return listenerFromHttpConnectionManager(
		ProbeListenerName,
		config.Get().EnvoyProbePort,
		&httpconnectionmanagerv3.HttpConnectionManager{
			StatPrefix: ProbeListenerName,
			HttpFilters: []*httpconnectionmanagerv3.HttpFilter{
				{
					Name:       EnvoyExtensionHttpFilterHealthCheck,
					ConfigType: &httpconnectionmanagerv3.HttpFilter_TypedConfig{TypedConfig: healthCheck},
				},
				router,
			},
			RouteSpecifier: &httpconnectionmanagerv3.HttpConnectionManager_RouteConfig{
				RouteConfig: &routev3.RouteConfiguration{
					Name: ProbeListenerName,
					VirtualHosts: []*routev3.VirtualHost{
						{
							Name:    ProbeListenerName,
							Domains: []string{"*"},
							Routes: []*routev3.Route{
								{
									Match: &routev3.RouteMatch{
										PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: "/"},
									},
									Action: &routev3.Route_DirectResponse{
										DirectResponse: &routev3.DirectResponseAction{Status: 404},
									},
								},
							},
						},
					},
				},
			},
		},
	)
}
