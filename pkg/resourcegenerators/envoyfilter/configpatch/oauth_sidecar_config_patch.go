package configpatch

import (
	"slices"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/configmap"
)

const (
	TokenSecretFileName       = "token-secret.yaml"
	HmacSecretFileName        = "hmac-secret.yaml"
	IstioTokenSecretSource    = "/etc/istio/config/" + TokenSecretFileName
	IstioHmacSecretSource     = "/etc/istio/config/" + HmacSecretFileName
	IstioCredentialsDirectory = "/etc/istio/config"
)

func GetOAuthSidecarConfigPatchValue(
	scope state.Scope,
) map[string]interface{} {
	var resourcesInterface []interface{}
	if scope.AuthPolicy.Spec.AcceptedResources != nil {
		for _, resource := range *scope.AuthPolicy.Spec.AcceptedResources {
			resourcesInterface = append(resourcesInterface, resource)
		}
	}

	if !slices.Contains(scope.AutoLoginConfig.Scopes, "openid") {
		scope.AutoLoginConfig.Scopes = append(scope.AutoLoginConfig.Scopes, "openid")
	}

	authScopesInterface := make([]interface{}, len(scope.AutoLoginConfig.Scopes))
	for i, authScope := range scope.AutoLoginConfig.Scopes {
		authScopesInterface[i] = authScope
	}

	oauthSidecarConfigPatchValue := map[string]interface{}{
		"token_endpoint": map[string]interface{}{
			"cluster": "oauth",
			"uri":     scope.IdentityProviderUris.TokenURI,
			"timeout": "5s",
		},
		"retry_policy":           map[string]interface{}{},
		"authorization_endpoint": scope.IdentityProviderUris.AuthorizationURI,
		"redirect_uri":           "https://%REQ(:authority)%" + scope.AutoLoginConfig.RedirectPath,
		"redirect_path_matcher": map[string]interface{}{
			"path": map[string]interface{}{
				"exact": scope.AutoLoginConfig.RedirectPath,
			},
		},
		"signout_path": map[string]interface{}{
			"path": map[string]interface{}{
				"exact": scope.AutoLoginConfig.LogoutPath,
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
				"name": configmap.BypassOauthLoginHeaderName,
				"string_match": map[string]interface{}{
					"exact": "true",
				},
			},
		},
		"deny_redirect_matcher": []interface{}{
			map[string]interface{}{
				"name": configmap.BypassOauthLoginHeaderName,
				"string_match": map[string]interface{}{
					"exact": "true",
				},
			},
		},
		"credentials": map[string]interface{}{
			"client_id": *scope.OAuthCredentials.ClientID,
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

	if scope.AuthPolicy.Spec.AcceptedResources != nil && len(*scope.AuthPolicy.Spec.AcceptedResources) > 0 {
		oauthSidecarConfigPatchValue["resources"] = resourcesInterface
	}

	if scope.IdentityProviderUris.EndSessionURI != nil {
		oauthSidecarConfigPatchValue["end_session_endpoint"] = *scope.IdentityProviderUris.EndSessionURI
	}

	return map[string]interface{}{
		"name": "envoy.filters.http.oauth2",
		"typed_config": map[string]interface{}{
			"@type":  "type.googleapis.com/envoy.extensions.filters.http.oauth2.v3.OAuth2",
			"config": oauthSidecarConfigPatchValue,
		},
	}
}
