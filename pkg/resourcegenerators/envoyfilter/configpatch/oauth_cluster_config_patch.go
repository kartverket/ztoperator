package configpatch

import (
	"fmt"

	"github.com/kartverket/ztoperator/pkg/helperfunctions"
)

func GetOAuth2ClusterConfig(
	clusterName string,
	tokenUrl string,
) (*map[string]any, error) {
	parsedUrl, err := helperfunctions.GetParsedHttpURL(tokenUrl)
	if err != nil {
		return nil, fmt.Errorf("parse token URL: %w", err)
	}

	clusterConfigPatch := map[string]any{
		"name":              clusterName,
		"dns_lookup_family": "V4_ONLY",
		"type":              "LOGICAL_DNS",
		"connect_timeout":   "10s",
		"lb_policy":         "ROUND_ROBIN",
		"load_assignment": map[string]any{
			"cluster_name": clusterName,
			"endpoints": []any{
				map[string]any{
					"lb_endpoints": []any{
						map[string]any{
							"endpoint": map[string]any{
								"address": map[string]any{
									"socket_address": map[string]any{
										"address":    parsedUrl.Host,
										"port_value": parsedUrl.Port,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if parsedUrl.Tls {
		clusterConfigPatch["transport_socket"] = map[string]any{
			"name": "envoy.transport_sockets.tls",
			"typed_config": map[string]any{
				"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
				"sni":   parsedUrl.Host,
			},
		}
	}
	return &clusterConfigPatch, nil
}
