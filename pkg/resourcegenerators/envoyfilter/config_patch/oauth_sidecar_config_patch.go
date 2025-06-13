package config_patch

import (
	"fmt"
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"strings"
)

const (
	IstioTokenSecretSource    = "/etc/istio/config/token-secret.yaml"
	IstioHmacSecretSource     = "/etc/istio/config/hmac-secret.yaml"
	IstioCredentialsDirectory = "/etc/istio/config"
)

type normalizedIgnoreAuthRule struct {
	Path   string
	Method string
}

func GetOAuthSidecarConfigPatchValue(
	tokenEndpoint string,
	authorizationEndpoint string,
	redirectPath string,
	signoutPath string,
	clientId string,
	authScopes []string,
	ignoreAuthRules *[]v1alpha1.RequestMatcher,
) map[string]interface{} {
	passThroughMatchers := []interface{}{
		map[string]interface{}{
			"name": "authorization",
			"string_match": map[string]interface{}{
				"prefix": "Bearer ",
			},
		},
	}
	if ignoreAuthRules != nil {
		passThroughMatchers = append(passThroughMatchers, getPassThroughMatcher(*ignoreAuthRules))
	}

	//TODO: Add resource indicator as resources config field
	//TODO: Must update podSettings, is this wanted??
	//TODO: RetryPolicy??
	//TODO: end_session_endpoint
	//TODO: CookieConfig??

	// Convert authScopes []string into []interface{}
	authScopesInterface := make([]interface{}, len(authScopes))
	for i, v := range authScopes {
		authScopesInterface[i] = v
	}

	return map[string]interface{}{
		"name": "envoy.filters.http.oauth2",
		"typed_config": map[string]interface{}{
			"@type": "type.googleapis.com/envoy.extensions.filters.http.oauth2.v3.OAuth2",
			"config": map[string]interface{}{
				"token_endpoint": map[string]interface{}{
					"cluster": "oauth",
					"uri":     tokenEndpoint,
					"timeout": "5s",
				},
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
					"client_id": clientId,
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
			},
		},
	}
}

func normalizeIgnoreAuthRules(ignoreAuthRules []v1alpha1.RequestMatcher) []normalizedIgnoreAuthRule {
	var normalizedIgnoreAuthRules []normalizedIgnoreAuthRule
	for _, ignoreAuthRule := range ignoreAuthRules {
		for _, path := range ignoreAuthRule.Paths {
			if len(ignoreAuthRule.Methods) == 0 {
				ignoreAuthRule.Methods = v1alpha1.AcceptedHttpMethods
			}
			for _, method := range ignoreAuthRule.Methods {
				normalizedIgnoreAuthRules = append(normalizedIgnoreAuthRules, normalizedIgnoreAuthRule{
					Path:   path,
					Method: method,
				})
			}
		}
	}
	return normalizedIgnoreAuthRules
}

func getPassThroughMatcher(rules []v1alpha1.RequestMatcher) map[string]interface{} {
	var regexString []string
	for _, rule := range rules {
		for _, path := range rule.Paths {
			var methodsPattern []string
			if len(rule.Methods) == 0 {
				rule.Methods = v1alpha1.AcceptedHttpMethods
			}
			for _, method := range rule.Methods {
				methodsPattern = append(methodsPattern, method)
			}
			var methodsPatternString string
			if len(methodsPattern) > 1 {
				concatenated := strings.Join(methodsPattern, "|")
				methodsPatternString = fmt.Sprintf("(%s)", concatenated)
			} else {
				methodsPatternString = methodsPattern[0]
			}
			regexString = append(regexString, fmt.Sprintf(`%s:%s`, methodsPatternString, convertPathToRegex(path)))
		}
	}
	var result string
	if len(regexString) > 1 {
		concatenated := strings.Join(regexString, "|")
		result = fmt.Sprintf(`^(%s)$`, concatenated)
	} else {
		result = fmt.Sprintf(`^%s$`, regexString[0])
	}

	return map[string]interface{}{
		"name": BypassOauthLoginHeaderName,
		"string_match": map[string]interface{}{
			"safe_regex": map[string]interface{}{
				"google_re2": map[string]interface{}{},
				"regex":      fmt.Sprintf("%s", result),
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
	return strings.ReplaceAll(pathWithIstioWildcards, "*", "**")
}

func envoyWildcardsToRE2Regex(path string) string {
	const doubleStarPlaceholder = "<<DOUBLE_STAR>>"
	path = strings.ReplaceAll(path, "**", doubleStarPlaceholder)
	path = strings.ReplaceAll(path, "*", "[^/]+")
	return strings.ReplaceAll(path, doubleStarPlaceholder, ".*")
}
