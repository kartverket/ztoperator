package configmap

import (
	_ "embed"
	"fmt"
	"net/url"
	"strings"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	v2 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	BypassOauthLoginHeaderName = "x-bypass-login"
	DenyRedirectHeaderName     = "x-deny-redirect"
	LuaScriptFileName          = "ztoperator.lua"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v2.ConfigMap {
	if scope.IsMisconfigured() || scope.AuthPolicy.Spec.AutoLogin == nil ||
		!scope.AuthPolicy.Spec.AutoLogin.Enabled {
		return nil
	}

	ignoreAuthRequestMatchers := func(ignoreAuthRules *[]v1alpha1.RequestMatcher) []v1alpha1.RequestMatcher {
		if ignoreAuthRules != nil {
			return *ignoreAuthRules
		}
		return []v1alpha1.RequestMatcher{}
	}(scope.AuthPolicy.Spec.IgnoreAuthRules)

	denyRedirectRequestMatchers := func(authRules *[]v1alpha1.RequestAuthRule) []v1alpha1.RequestMatcher {
		var result []v1alpha1.RequestMatcher
		if authRules != nil {
			for _, authRule := range *authRules {
				if authRule.DenyRedirect != nil && *authRule.DenyRedirect {
					result = append(result, authRule.RequestMatcher)
				}
			}
		}
		return result
	}(scope.AuthPolicy.Spec.AuthRules)

	requireAuthRequestMatchers := func(
		requireAuth []v1alpha1.RequestMatcher,
		autoLoginConfig state.AutoLoginConfig,
	) []v1alpha1.RequestMatcher {
		var autoLoginPaths []string
		autoLoginPaths = append(autoLoginPaths, autoLoginConfig.RedirectPath)
		autoLoginPaths = append(autoLoginPaths, autoLoginConfig.LogoutPath)
		if autoLoginConfig.LoginPath != nil {
			autoLoginPaths = append(autoLoginPaths, *autoLoginConfig.LoginPath)
		}
		requireAuth = append(requireAuth, v1alpha1.RequestMatcher{
			Paths:   autoLoginPaths,
			Methods: []string{},
		})
		return requireAuth
	}(
		v1alpha1.GetRequestMatchers(
			scope.AuthPolicy.Spec.AuthRules,
		),
		scope.AutoLoginConfig,
	)

	ignoreAuthAsRE2Regex := convertToRE2Regex(ignoreAuthRequestMatchers)
	requireAuthAsRE2Regex := convertToRE2Regex(requireAuthRequestMatchers)
	denyRedirectAsRE2Regex := convertToRE2Regex(denyRedirectRequestMatchers)

	// Produce equivalent Lua tables that the generated script can iterate over.
	ignoreRulesLua := buildLuaRules(ignoreAuthAsRE2Regex)
	requireRulesLua := buildLuaRules(requireAuthAsRE2Regex)
	denyRedirectRulesLua := buildLuaRules(denyRedirectAsRE2Regex)

	loginParamsAsLua := buildLuaParams(encodeLoginParams(scope.AutoLoginConfig.LoginParams))

	var queryEscapedPostLogoutRedirectURI string
	if scope.AutoLoginConfig.PostLogoutRedirectURI != nil {
		queryEscapedPostLogoutRedirectURI = url.QueryEscape(*scope.AutoLoginConfig.PostLogoutRedirectURI)
	} else {
		// We handle postLogoutRedirectURI == nil as "" to make it easier when building the Lua script
		queryEscapedPostLogoutRedirectURI = ""
	}

	luaScript := fmt.Sprintf(
		scope.AutoLoginConfig.LuaScriptConfig.LuaScript,
		ignoreRulesLua,
		requireRulesLua,
		denyRedirectRulesLua,
		scope.IdentityProviderUris.AuthorizationURI,
		loginParamsAsLua,
		*scope.IdentityProviderUris.EndSessionURI,
		queryEscapedPostLogoutRedirectURI,
		BypassOauthLoginHeaderName,
		DenyRedirectHeaderName,
	)

	return &v2.ConfigMap{
		ObjectMeta: objectMeta,
		Data: map[string]string{
			LuaScriptFileName: luaScript,
		},
	}
}

func encodeLoginParams(raw map[string]string) map[string]string {
	encoded := make(map[string]string, len(raw))
	for k, v := range raw {
		encoded[k] = url.QueryEscape(v)
	}
	return encoded
}

func buildLuaParams(params map[string]string) string {
	if len(params) == 0 {
		return "{}"
	}
	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for k, v := range params {
		if !first {
			sb.WriteString(",")
		}
		first = false
		sb.WriteString(fmt.Sprintf(`["%s"]="%s"`, k, v))
	}
	sb.WriteString("}")
	return sb.String()
}

func convertToRE2Regex(requestMatchers []v1alpha1.RequestMatcher) []v1alpha1.RequestMatcher {
	var result []v1alpha1.RequestMatcher
	for _, matcher := range requestMatchers {
		var pathAsRE2Regex []string
		for _, path := range matcher.Paths {
			pathAsRE2Regex = append(pathAsRE2Regex, "^"+convertRequestMatcherPathToRegex(path))
		}
		result = append(result, v1alpha1.RequestMatcher{
			Paths:   pathAsRE2Regex,
			Methods: matcher.Methods,
		})
	}
	return result
}

func convertRequestMatcherPathToRegex(path string) string {
	if strings.Contains(path, "*") || strings.Contains(path, "{") {
		path = convertToEnvoyWildcards(path)
		return envoyWildcardsToRE2Regex(path)
	}
	return path + "$"
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
	path = strings.ReplaceAll(path, ".", "%.")
	return strings.ReplaceAll(path, doubleStarPlaceholder, ".*")
}

// escapeLuaString ensures any back‑slashes or quotes in the regex are safe for Lua source.
func escapeLuaString(s string) string {
	if s == "/" {
		return "^/$"
	}
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// buildLuaRules converts a slice of RequestMatcher into a Lua literal table.
// Each entry becomes {regex="<path‑regex>", methods={["GET"]=true, ...}}.
func buildLuaRules(requestMatchers []v1alpha1.RequestMatcher) string {
	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for _, matcher := range requestMatchers {
		for _, path := range matcher.Paths {
			if !first {
				sb.WriteString(",")
			}
			first = false

			sb.WriteString(`{regex="`)
			sb.WriteString(escapeLuaString(path))
			sb.WriteString(`",methods={`)

			if len(matcher.Methods) > 0 {
				for idx, method := range matcher.Methods {
					if idx > 0 {
						sb.WriteString(",")
					}
					sb.WriteString(`["`)
					sb.WriteString(method)
					sb.WriteString(`"]=true`)
				}
			}
			sb.WriteString("}}")
		}
	}
	sb.WriteString("}")
	return sb.String()
}
