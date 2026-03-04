package luascript

import (
	_ "embed"
	"fmt"
	"net/url"
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

	ignoreAuthAsLuaPatterns := convertToLuaPatterns(ignoreAuthRequestMatchers)
	requireAuthAsLuaPatterns := convertToLuaPatterns(requireAuthRequestMatchers)
	denyRedirectAsLuaPatterns := convertToLuaPatterns(denyRedirectRequestMatchers)

	// Produce equivalent Lua tables that the generated script can iterate over.
	ignoreRulesLua := BuildLuaRules(ignoreAuthAsLuaPatterns)
	requireRulesLua := BuildLuaRules(requireAuthAsLuaPatterns)
	denyRedirectRulesLua := BuildLuaRules(denyRedirectAsLuaPatterns)

	loginParamsAsLua := BuildLuaParams(autoLoginConfig.LoginParams)

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

func convertToLuaPatterns(requestMatchers []v1alpha1.RequestMatcher) []v1alpha1.RequestMatcher {
	result := make([]v1alpha1.RequestMatcher, 0, len(requestMatchers))
	for _, matcher := range requestMatchers {
		pathAsLuaPattern := make([]string, 0, len(matcher.Paths))
		for _, path := range matcher.Paths {
			pathAsLuaPattern = append(pathAsLuaPattern, convertRequestMatcherPathToRegex(path))
		}
		result = append(result, v1alpha1.RequestMatcher{
			Paths:   pathAsLuaPattern,
			Methods: matcher.Methods,
		})
	}
	return result
}

func convertRequestMatcherPathToRegex(path string) string {
	path = strings.ReplaceAll(path, "-", "%-")
	if strings.Contains(path, "*") || strings.Contains(path, "{") {
		path = convertToEnvoyWildcards(path)
		return "^" + envoyWildcardsToLuaPattern(path) + "$"
	}
	return "^" + EscapeLuaPatternChars(path) + "$"
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

	const doubleStarPlaceholder = "<<DOUBLE_STAR>>"
	path = strings.ReplaceAll(path, "**", doubleStarPlaceholder)
	path = strings.ReplaceAll(path, "*", "[^/]+")
	path = strings.ReplaceAll(path, ".", "%.")
	return strings.ReplaceAll(path, doubleStarPlaceholder, ".*")
}
func envoyWildcardsToLuaPattern(path string) string {


}
