package cluster

import (
	"fmt"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	commondnsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/common/dns/v3"
	dnsclusterv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/dns/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	upstreamhttpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const caBundlePath = "/etc/ssl/certs/ca-certificates.crt"

func FromURI(clusterName, uri string) (*clusterv3.Cluster, error) {
	parsedURL, err := helperfunctions.GetParsedHttpURL(uri)
	if err != nil {
		return nil, err
	}

	dnsClusterConfig, err := anypb.New(&dnsclusterv3.DnsCluster{
		DnsLookupFamily:              commondnsv3.DnsLookupFamily_V4_PREFERRED,
		RespectDnsTtl:                true,
		DnsRefreshRate:               durationpb.New(30 * time.Second),
		DnsMinRefreshRate:            durationpb.New(5 * time.Second),
		DnsJitter:                    durationpb.New(1 * time.Second),
		AllAddressesInSingleEndpoint: true,
		DnsFailureRefreshRate: &dnsclusterv3.DnsCluster_RefreshRate{
			BaseInterval: durationpb.New(1 * time.Second),
			MaxInterval:  durationpb.New(10 * time.Second),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DNS cluster config for %s: %w", clusterName, err)
	}

	httpProtocolOptions, err := anypb.New(&upstreamhttpv3.HttpProtocolOptions{
		CommonHttpProtocolOptions: &corev3.HttpProtocolOptions{
			IdleTimeout:     durationpb.New(55 * time.Second),
			MaxHeadersCount: wrapperspb.UInt32(100),
		},
		UpstreamProtocolOptions: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{
					HttpProtocolOptions: &corev3.Http1ProtocolOptions{},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal HTTP protocol options for %s: %w", clusterName, err)
	}

	cluster := &clusterv3.Cluster{
		Name: clusterName,
		ClusterDiscoveryType: &clusterv3.Cluster_ClusterType{
			ClusterType: &clusterv3.Cluster_CustomClusterType{
				Name:        "envoy.cluster.dns",
				TypedConfig: dnsClusterConfig,
			},
		},
		ConnectTimeout:                durationpb.New(5 * time.Second),
		PerConnectionBufferLimitBytes: wrapperspb.UInt32(32 * 1024),
		CircuitBreakers: &clusterv3.CircuitBreakers{
			Thresholds: []*clusterv3.CircuitBreakers_Thresholds{
				{
					Priority:           corev3.RoutingPriority_DEFAULT,
					MaxConnections:     wrapperspb.UInt32(64),
					MaxPendingRequests: wrapperspb.UInt32(64),
					MaxRequests:        wrapperspb.UInt32(256),
					MaxRetries:         wrapperspb.UInt32(3),
					TrackRemaining:     true,
				},
			},
		},
		UpstreamConnectionOptions: &clusterv3.UpstreamConnectionOptions{
			TcpKeepalive: &corev3.TcpKeepalive{
				KeepaliveTime:     wrapperspb.UInt32(300),
				KeepaliveInterval: wrapperspb.UInt32(30),
				KeepaliveProbes:   wrapperspb.UInt32(3),
			},
		},
		TypedExtensionProtocolOptions: map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": httpProtocolOptions,
		},
		LoadAssignment: &endpointv3.ClusterLoadAssignment{
			ClusterName: clusterName,
			Endpoints: []*endpointv3.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpointv3.LbEndpoint{
						{
							HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
								Endpoint: &endpointv3.Endpoint{
									Hostname: parsedURL.Host,
									Address: &corev3.Address{
										Address: &corev3.Address_SocketAddress{
											SocketAddress: &corev3.SocketAddress{
												Protocol: corev3.SocketAddress_TCP,
												Address:  parsedURL.Host,
												PortSpecifier: &corev3.SocketAddress_PortValue{
													PortValue: uint32(parsedURL.Port),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if parsedURL.Tls {
		upstreamTLSContext, tlsErr := anypb.New(&tlsv3.UpstreamTlsContext{
			Sni: parsedURL.Host,
			CommonTlsContext: &tlsv3.CommonTlsContext{
				TlsParams: &tlsv3.TlsParameters{
					TlsMinimumProtocolVersion: tlsv3.TlsParameters_TLSv1_2,
					TlsMaximumProtocolVersion: tlsv3.TlsParameters_TLSv1_3,
				},
				AlpnProtocols: []string{"http/1.1"},
				ValidationContextType: &tlsv3.CommonTlsContext_ValidationContext{
					ValidationContext: &tlsv3.CertificateValidationContext{
						TrustedCa: &corev3.DataSource{
							Specifier: &corev3.DataSource_Filename{Filename: caBundlePath},
						},
						MatchTypedSubjectAltNames: []*tlsv3.SubjectAltNameMatcher{
							{
								SanType: tlsv3.SubjectAltNameMatcher_DNS,
								Matcher: &matcherv3.StringMatcher{
									MatchPattern: &matcherv3.StringMatcher_Exact{Exact: parsedURL.Host},
								},
							},
						},
					},
				},
			},
		})
		if tlsErr != nil {
			return nil, fmt.Errorf("marshal upstream TLS context for %s: %w", clusterName, tlsErr)
		}
		cluster.TransportSocket = &corev3.TransportSocket{
			Name:       "envoy.transport_sockets.tls",
			ConfigType: &corev3.TransportSocket_TypedConfig{TypedConfig: upstreamTLSContext},
		}
	}

	if validateErr := cluster.ValidateAll(); validateErr != nil {
		return nil, fmt.Errorf("validate cluster %s: %w", clusterName, validateErr)
	}

	return cluster, nil
}
