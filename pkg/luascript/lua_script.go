package luascript

import (
	_ "embed"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
)

const (
	BypassOauthLoginHeaderName = "x-bypass-login"
	DenyRedirectHeaderName     = "x-deny-redirect"
)

//go:embed ztoperator.lua
var luaScript string

func GetLuaScript(
	authPolicy *v1alpha1.AuthPolicy,
	autoLoginConfig state.AutoLoginConfig,
	identityProviderUris state.IdentityProviderUris,
) string {
	ignoreAuthRequestMatchers := func(ignoreAuthRules *[]v1alpha1.RequestMatcher) []v1alpha1.RequestMatcher {
		if ignoreAuthRules != nil {
			return *ignoreAuthRules
		}
		return []v1alpha1.RequestMatcher{}
	}(authPolicy.Spec.IgnoreAuthRules)

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
	}(authPolicy.Spec.AuthRules)

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
			authPolicy.Spec.AuthRules,
		),
		autoLoginConfig,
	)

	ignoreAuthAsRE2Regex := convertToRE2Regex(ignoreAuthRequestMatchers)
	requireAuthAsRE2Regex := convertToRE2Regex(requireAuthRequestMatchers)
	denyRedirectAsRE2Regex := convertToRE2Regex(denyRedirectRequestMatchers)

	// Produce equivalent Lua tables that the generated script can iterate over.
	ignoreRulesLua := buildLuaRules(ignoreAuthAsRE2Regex)
	requireRulesLua := buildLuaRules(requireAuthAsRE2Regex)
	denyRedirectRulesLua := buildLuaRules(denyRedirectAsRE2Regex)

	loginParamsAsLua := buildLuaParams(encodeLoginParams(autoLoginConfig.LoginParams))

	var queryEscapedPostLogoutRedirectURI string
	if autoLoginConfig.PostLogoutRedirectURI != nil {
		queryEscapedPostLogoutRedirectURI = url.QueryEscape(*autoLoginConfig.PostLogoutRedirectURI)
	} else {
		// We handle postLogoutRedirectURI == nil as "" to make it easier when building the Lua script
		queryEscapedPostLogoutRedirectURI = ""
	}

	return fmt.Sprintf(
		luaScript,
		ignoreRulesLua,
		requireRulesLua,
		denyRedirectRulesLua,
		identityProviderUris.AuthorizationURI,
		loginParamsAsLua,
		*identityProviderUris.EndSessionURI,
		queryEscapedPostLogoutRedirectURI,
		BypassOauthLoginHeaderName,
		DenyRedirectHeaderName,
	)
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
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for _, k := range keys {
		v := params[k]
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
	result := make([]v1alpha1.RequestMatcher, 0, len(requestMatchers))
	for _, matcher := range requestMatchers {
		pathAsRE2Regex := make([]string, 0, len(matcher.Paths))
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
	path = strings.ReplaceAll(path, "-", "%-")
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
