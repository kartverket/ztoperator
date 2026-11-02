package envoymodel

import (
	"fmt"
	"time"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	overloadv3 "github.com/envoyproxy/go-control-plane/envoy/config/overload/v3"
	caresv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/network/dns_resolver/cares/v3"
	regexenginesv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/regex_engines/v3"
	cgroupmemoryv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/resource_monitors/cgroup_memory/v3"
	downstreamconnectionsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/resource_monitors/downstream_connections/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/constants"
	"github.com/kartverket/ztoperator/pkg/envoy"
	"github.com/kartverket/ztoperator/pkg/model"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	EnvoyExtensionGoogleRE2Name             = "envoy.regex_engines.google_re2"
	EnvoyExtensionCAresDnsResolverName      = "envoy.network.dns_resolver.cares"
	EnvoyExtensionHttpConnectionManagerName = "envoy.filters.network.http_connection_manager"

	AnyAddress = "0.0.0.0"

	AdminPort = 9901

	MaxActiveDownstreamConnections = 10000

	RuntimeLayerStatic = "static_layer"
	Re2MaxProgramSize  = "32768"

	IngressListenerName = "ingress"
)

func ToEnvoyBootstrap(
	authPolicy v1alpha1.AuthPolicy,
	autoLoginConfig model.AutoLoginConfig,
	identityProviderUris model.IdentityProviderUris,
	upstreamCluster *clusterv3.Cluster,
	clientID *string,
) (*bootstrapv3.Bootstrap, error) {
	envoyHttpFilterConfig, err := envoy.GetEnvoyHttpFilterConfig(
		authPolicy,
		autoLoginConfig,
		identityProviderUris,
		clientID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get envoy http filter config: %w", err)
	}

	ingress, err := ingressListener(envoyHttpFilterConfig.GetHttpFilters()...)
	if err != nil {
		return nil, fmt.Errorf("failed to create ingress listener: %w", err)
	}

	probe, err := probeListener()
	if err != nil {
		return nil, fmt.Errorf("failed to create probe listener: %w", err)
	}

	overloadManager, err := getOverloadManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create overload manager: %w", err)
	}

	typedDnsResolverConfig, err := anypb.New(&caresv3.CaresDnsResolverConfig{
		UdpMaxQueries:       wrapperspb.UInt32(100),
		QueryTimeoutSeconds: wrapperspb.UInt64(2),
		QueryTries:          wrapperspb.UInt32(3),
		DnsResolverOptions:  &corev3.DnsResolverOptions{},
	})
	if err != nil {
		return nil, err
	}

	emptyRE2Config, err := anypb.New(&regexenginesv3.GoogleRE2{})
	if err != nil {
		return nil, err
	}

	logFormat, err := structpb.NewStruct(map[string]any{
		"time":    "%Y-%m-%dT%T.%e%z",
		"level":   "%l",
		"logger":  "%n",
		"thread":  "%t",
		"source":  "%g:%#",
		"message": "%j",
	})
	if err != nil {
		return nil, fmt.Errorf("build application log format: %w", err)
	}

	nodeName := fmt.Sprintf("%s.%s", authPolicy.Name, authPolicy.Namespace)

	bootstrap := &bootstrapv3.Bootstrap{
		Node: &corev3.Node{
			Id:      nodeName,
			Cluster: nodeName,
		},
		StaticResources: &bootstrapv3.Bootstrap_StaticResources{
			Listeners: []*listenerv3.Listener{ingress, probe},
			Clusters:  append(envoyHttpFilterConfig.GetClusters(), upstreamCluster),
			Secrets:   nil,
		},
		Admin: &bootstrapv3.Admin{
			Address: &corev3.Address{
				Address: &corev3.Address_SocketAddress{
					SocketAddress: &corev3.SocketAddress{
						Protocol:      corev3.SocketAddress_TCP,
						Address:       constants.LoopbackAddress,
						PortSpecifier: &corev3.SocketAddress_PortValue{PortValue: AdminPort},
					},
				},
			},
			IgnoreGlobalConnLimit: true,
			AllowPaths: []*matcherv3.StringMatcher{
				exactMatcher("/ready"),
				exactMatcher("/server_info"),
			},
		},
		OverloadManager: overloadManager,
		LayeredRuntime: &bootstrapv3.LayeredRuntime{
			Layers: []*bootstrapv3.RuntimeLayer{
				{
					Name: RuntimeLayerStatic,
					LayerSpecifier: &bootstrapv3.RuntimeLayer_StaticLayer{
						StaticLayer: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"re2.max_program_size.error_level": structpb.NewStringValue(Re2MaxProgramSize),
							},
						},
					},
				},
			},
		},
		Watchdogs: &bootstrapv3.Watchdogs{
			MainThreadWatchdog: &bootstrapv3.Watchdog{
				MissTimeout:      durationpb.New(200 * time.Millisecond),
				MegamissTimeout:  durationpb.New(1 * time.Second),
				KillTimeout:      durationpb.New(60 * time.Second),
				MultikillTimeout: durationpb.New(30 * time.Second),
				MultikillThreshold: &typev3.Percent{
					Value: 50,
				},
			},
			WorkerWatchdog: &bootstrapv3.Watchdog{
				MissTimeout:      durationpb.New(200 * time.Millisecond),
				MegamissTimeout:  durationpb.New(1 * time.Second),
				KillTimeout:      durationpb.New(60 * time.Second),
				MultikillTimeout: durationpb.New(30 * time.Second),
				MultikillThreshold: &typev3.Percent{
					Value: 50,
				},
			},
		},
		ApplicationLogConfig: &bootstrapv3.Bootstrap_ApplicationLogConfig{
			LogFormat: &bootstrapv3.Bootstrap_ApplicationLogConfig_LogFormat{
				LogFormat: &bootstrapv3.Bootstrap_ApplicationLogConfig_LogFormat_JsonFormat{
					JsonFormat: logFormat,
				},
			},
		},
		TypedDnsResolverConfig: &corev3.TypedExtensionConfig{
			Name:        EnvoyExtensionCAresDnsResolverName,
			TypedConfig: typedDnsResolverConfig,
		},
		DefaultRegexEngine: &corev3.TypedExtensionConfig{Name: EnvoyExtensionGoogleRE2Name, TypedConfig: emptyRE2Config},
	}

	if validateErr := bootstrap.ValidateAll(); validateErr != nil {
		return nil, fmt.Errorf("validate envoy bootstrap: %w", validateErr)
	}

	return bootstrap, nil
}

func getOverloadManager() (*overloadv3.OverloadManager, error) {
	cgroupMemoryMonitor, err := anypb.New(&cgroupmemoryv3.CgroupMemoryConfig{})
	if err != nil {
		return nil, fmt.Errorf("marshal cgroup memory monitor config: %w", err)
	}

	downstreamConnectionsMonitor, err := anypb.New(&downstreamconnectionsv3.DownstreamConnectionsConfig{
		MaxActiveDownstreamConnections: MaxActiveDownstreamConnections,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal downstream connections monitor config: %w", err)
	}

	const (
		memoryMonitor      = "envoy.resource_monitors.cgroup_memory"
		connectionsMonitor = "envoy.resource_monitors.global_downstream_max_connections"
	)

	return &overloadv3.OverloadManager{
		RefreshInterval: durationpb.New(250 * time.Millisecond),
		ResourceMonitors: []*overloadv3.ResourceMonitor{
			{
				Name:       memoryMonitor,
				ConfigType: &overloadv3.ResourceMonitor_TypedConfig{TypedConfig: cgroupMemoryMonitor},
			},
			{
				Name:       connectionsMonitor,
				ConfigType: &overloadv3.ResourceMonitor_TypedConfig{TypedConfig: downstreamConnectionsMonitor},
			},
		},
		Actions: []*overloadv3.OverloadAction{
			{
				Name: "envoy.overload_actions.shrink_heap",
				Triggers: []*overloadv3.Trigger{
					{
						Name: memoryMonitor,
						TriggerOneof: &overloadv3.Trigger_Threshold{
							Threshold: &overloadv3.ThresholdTrigger{Value: 0.95},
						},
					},
				},
			},
			{
				Name: "envoy.overload_actions.disable_http_keepalive",
				Triggers: []*overloadv3.Trigger{
					{
						Name: memoryMonitor,
						TriggerOneof: &overloadv3.Trigger_Threshold{
							Threshold: &overloadv3.ThresholdTrigger{Value: 0.95},
						},
					},
				},
			},
			{
				Name: "envoy.overload_actions.stop_accepting_requests",
				Triggers: []*overloadv3.Trigger{
					{
						Name: memoryMonitor,
						TriggerOneof: &overloadv3.Trigger_Threshold{
							Threshold: &overloadv3.ThresholdTrigger{Value: 0.98},
						},
					},
				},
			},
		},
	}, nil
}

func exactMatcher(value string) *matcherv3.StringMatcher {
	return &matcherv3.StringMatcher{MatchPattern: &matcherv3.StringMatcher_Exact{Exact: value}}
}
