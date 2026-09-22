package configpatch

func GetInternalOAuthClusterConfigPatchValue(idpHostname string, port int) map[string]interface{} {
	return map[string]interface{}{
		"name":              "oauth",
		"dns_lookup_family": "V4_ONLY",
		"type":              "LOGICAL_DNS",
		"connect_timeout":   "10s",
		"lb_policy":         "ROUND_ROBIN",
		"load_assignment": map[string]interface{}{
			"cluster_name": "oauth",
			"endpoints": []interface{}{
				map[string]interface{}{
					"lb_endpoints": []interface{}{
						map[string]interface{}{
							"endpoint": map[string]interface{}{
								"address": map[string]interface{}{
									"socket_address": map[string]interface{}{
										"address":    idpHostname,
										"port_value": port,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	parsedUrl, err := helperfunctions.GetParsedHttpURL(tokenUrl)
	if err != nil {
		return nil, fmt.Errorf("parse token URL: %w", err)
	}

func GetExternalOAuthClusterPatchValue(idpHostname string) map[string]interface{} {
	return map[string]interface{}{
		"name":              "oauth",
		"dns_lookup_family": "V4_ONLY",
		"type":              "LOGICAL_DNS",
		"connect_timeout":   "10s",
		"lb_policy":         "ROUND_ROBIN",
		"transport_socket": map[string]interface{}{
	if parsedUrl.Tls {
		clusterConfigPatch["transport_socket"] = map[string]any{
			"name": "envoy.transport_sockets.tls",
			"typed_config": map[string]any{
				"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
				"sni":   idpHostname,
			},
		},
		"load_assignment": map[string]interface{}{
			"cluster_name": "oauth",
			"endpoints": []interface{}{
				map[string]interface{}{
					"lb_endpoints": []interface{}{
						map[string]interface{}{
							"endpoint": map[string]interface{}{
								"address": map[string]interface{}{
									"socket_address": map[string]interface{}{
										"address":    idpHostname,
										"port_value": 443,
									},
								},
							},
						},
					},
				},
			},
		},
				"sni":   parsedUrl.Host,
			},
		}
	}
}
