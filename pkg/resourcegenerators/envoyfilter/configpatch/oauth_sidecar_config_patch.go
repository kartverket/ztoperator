package configpatch

const (
	TokenSecretFileName       = "token-secret.yaml"
	HmacSecretFileName        = "hmac-secret.yaml"
	IstioTokenSecretSource    = "/etc/istio/config/" + TokenSecretFileName
	IstioHmacSecretSource     = "/etc/istio/config/" + HmacSecretFileName
	IstioCredentialsDirectory = "/etc/istio/config"
)

func GetOAuthSidecarConfigPatchValue(
	tokenEndpoint string,
	authorizationEndpoint string,
	redirectPath string,
	signoutPath string,
	clientID string,
	authScopes []string,
	resources *[]string,
) map[string]interface{} {
	var resourcesInterface []interface{}
	if resources != nil {
		for _, resource := range *resources {
			resourcesInterface = append(resourcesInterface, resource)
		}
	}

	authScopesInterface := make([]interface{}, len(authScopes))
	for i, v := range authScopes {
		authScopesInterface[i] = v
	}

	oauthSidecarConfigPatchValue := map[string]interface{}{
		"token_endpoint": map[string]interface{}{
			"cluster": "oauth",
			"uri":     tokenEndpoint,
			"timeout": "5s",
		},
		"retry_policy":           map[string]interface{}{},
		"authorization_endpoint": authorizationEndpoint,
		"redirect_uri":           "https://%REQ(:authority)%" + redirectPath,
		"redirect_path_matcher": map[string]interface{}{
			"path": map[string]interface{}{
				"exact": redirectPath,
			},
		},
		"signout_path": map[string]interface{}{
			"path": map[string]interface{}{
				"exact": signoutPath,
			},
		},
		"forward_bearer_token": true,
		"use_refresh_token":    true,
		"pass_through_matcher": []interface{}{
			map[string]interface{}{
				"name": "authorization",
				"string_match": map[string]interface{}{
					"prefix": "Bearer ",
				},
			},
			map[string]interface{}{
				"name": BypassOauthLoginHeaderName,
				"string_match": map[string]interface{}{
					"exact": "true",
				},
			},
		},
		"deny_redirect_matcher": []interface{}{
			map[string]interface{}{
				"name": DenyRedirectHeaderName,
				"string_match": map[string]interface{}{
					"exact": "true",
				},
			},
		},
		"credentials": map[string]interface{}{
			"client_id": clientID,
			"token_secret": map[string]interface{}{
				"name": "token",
				"sds_config": map[string]interface{}{
					"path_config_source": map[string]interface{}{
						"path": IstioTokenSecretSource,
						"watched_directory": map[string]interface{}{
							"path": IstioCredentialsDirectory,
						},
					},
				},
			},
			"hmac_secret": map[string]interface{}{
				"name": "hmac",
				"sds_config": map[string]interface{}{
					"path_config_source": map[string]interface{}{
						"path": IstioHmacSecretSource,
						"watched_directory": map[string]interface{}{
							"path": IstioCredentialsDirectory,
						},
					},
				},
			},
		},
		"auth_scopes": authScopesInterface,
	}

	if resources != nil && len(*resources) > 0 {
		oauthSidecarConfigPatchValue["resources"] = resourcesInterface
	}

	return map[string]interface{}{
		"name": "envoy.filters.http.oauth2",
		"typed_config": map[string]interface{}{
			"@type":  "type.googleapis.com/envoy.extensions.filters.http.oauth2.v3.OAuth2",
			"config": oauthSidecarConfigPatchValue,
		},
	}
}
