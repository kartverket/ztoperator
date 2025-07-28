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

func GetExternalOAuthClusterPatchValue(idpHostname string) map[string]interface{} {
	return map[string]interface{}{
		"name":              "oauth",
		"dns_lookup_family": "V4_ONLY",
		"type":              "LOGICAL_DNS",
		"connect_timeout":   "10s",
		"lb_policy":         "ROUND_ROBIN",
		"transport_socket": map[string]interface{}{
			"name": "envoy.transport_sockets.tls",
			"typed_config": map[string]interface{}{
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
	}
}
