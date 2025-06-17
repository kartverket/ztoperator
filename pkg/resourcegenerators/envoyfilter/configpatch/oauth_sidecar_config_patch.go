package configpatch

import (
	"fmt"
	"strings"

	"github.com/kartverket/ztoperator/api/v1alpha1"
)

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
	ignoreAuthRules *[]v1alpha1.RequestMatcher,
	loginPath *string,
) map[string]interface{} {
	passThroughMatchers := []interface{}{
		map[string]interface{}{
			"name": "authorization",
			"string_match": map[string]interface{}{
				"prefix": "Bearer ",
			},
		},
	}
	if loginPath != nil {
		passThroughMatchers = append(
			passThroughMatchers,
			getPassThroughMatcherFromLoginPath(*loginPath, redirectPath, signoutPath),
		)
	} else if ignoreAuthRules != nil {
		passThroughMatchers = append(passThroughMatchers, getPassThroughMatcherFromIgnoreAuthRules(*ignoreAuthRules))
	}

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
		"pass_through_matcher": passThroughMatchers,
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

func getPassThroughMatcherFromLoginPath(loginPath, redirectPath, logoutPath string) map[string]interface{} {
	return map[string]interface{}{
		"name": ":path",
		"string_match": map[string]interface{}{
			"safe_regex": map[string]interface{}{
				"google_re2": map[string]interface{}{},
				"regex": fmt.Sprintf(
					"^(%s|%s.*|%s)$",
					convertPathToRegex(loginPath),
					convertPathToRegex(redirectPath),
					convertPathToRegex(logoutPath),
				),
			},
		},
		"invert_match": true,
	}
}

func getPassThroughMatcherFromIgnoreAuthRules(rules []v1alpha1.RequestMatcher) map[string]interface{} {
	var regexPattern []string
	for _, rule := range rules {
		for _, path := range rule.Paths {
			var methodsPattern []string
			if len(rule.Methods) == 0 {
				rule.Methods = v1alpha1.GetAcceptedHTTPMethods()
			}
			methodsPattern = append(methodsPattern, rule.Methods...)
			var methodsPatternString string
			if len(methodsPattern) > 1 {
				concatenated := strings.Join(methodsPattern, "|")
				methodsPatternString = fmt.Sprintf("(%s)", concatenated)
			} else {
				methodsPatternString = methodsPattern[0]
			}
			regexPattern = append(regexPattern, fmt.Sprintf(`%s:%s`, methodsPatternString, convertPathToRegex(path)))
		}
	}
	var result string
	if len(regexPattern) > 1 {
		concatenated := strings.Join(regexPattern, "|")
		result = fmt.Sprintf(`^(%s)$`, concatenated)
	} else {
		result = fmt.Sprintf(`^%s$`, regexPattern[0])
	}

	return map[string]interface{}{
		"name": BypassOauthLoginHeaderName,
		"string_match": map[string]interface{}{
			"safe_regex": map[string]interface{}{
				"google_re2": map[string]interface{}{},
				"regex":      result,
			},
		},
	}
}

func convertPathToRegex(path string) string {
	if strings.Contains(path, "*") || strings.Contains(path, "{") {
		path = convertToEnvoyWildcards(path)
		return envoyWildcardsToRE2Regex(path)
	}
	return path
}

func convertToEnvoyWildcards(pathWithIstioWildcards string) string {
	if strings.Contains(pathWithIstioWildcards, "{") {
		// New path wildcard syntax
		removedStartBracket := strings.ReplaceAll(pathWithIstioWildcards, "{", "")
		return strings.ReplaceAll(removedStartBracket, "}", "")
	}
	// Old wildcard syntax
	return strings.ReplaceAll(pathWithIstioWildcards, "*", "**")
}

func envoyWildcardsToRE2Regex(path string) string {
	const doubleStarPlaceholder = "<<DOUBLE_STAR>>"
	path = strings.ReplaceAll(path, "**", doubleStarPlaceholder)
	path = strings.ReplaceAll(path, "*", "[^/]+")
	return strings.ReplaceAll(path, doubleStarPlaceholder, ".*")
}
