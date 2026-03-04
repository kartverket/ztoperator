package luascript

import (
	_ "embed"
	"fmt"
	"net/url"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
)

const (
	BypassOauthLoginHeaderName = "x-bypass-login"
	DenyRedirectHeaderName     = "x-deny-redirect"
)

//go:embed ztoperator.lua
var luaScript string

func GenerateLuaScript(
	authPolicy *v1alpha1.AuthPolicy,
	autoLoginConfig state.AutoLoginConfig,
	identityProviderUris state.IdentityProviderUris,
) string {
	ignoreAuthRequestMatchers := IgnoreAuthMatchers(authPolicy.Spec.IgnoreAuthRules)
	requireAuthRequestMatchers := RequireAuthMatchers(authPolicy.Spec.AuthRules, autoLoginConfig)
	denyRedirectRequestMatchers := DenyRedirectMatchers(authPolicy.Spec.AuthRules)

	ignoreRulesLua := ConvertRequestMatchersToLuaTableString(ignoreAuthRequestMatchers)
	requireRulesLua := ConvertRequestMatchersToLuaTableString(requireAuthRequestMatchers)
	denyRedirectRulesLua := ConvertRequestMatchersToLuaTableString(denyRedirectRequestMatchers)

	loginParamsAsLua := ConvertLoginParamsToLuaParams(autoLoginConfig.LoginParams)

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

func IgnoreAuthMatchers(ignoreAuthRules *[]v1alpha1.RequestMatcher) []v1alpha1.RequestMatcher {
	if ignoreAuthRules != nil {
		return *ignoreAuthRules
	}
	return []v1alpha1.RequestMatcher{}
}

func DenyRedirectMatchers(authRules *[]v1alpha1.RequestAuthRule) []v1alpha1.RequestMatcher {
	var result []v1alpha1.RequestMatcher
	if authRules != nil {
		for _, authRule := range *authRules {
			if authRule.DenyRedirect != nil && *authRule.DenyRedirect {
				result = append(result, authRule.RequestMatcher)
			}
		}
	}
	return result
}

func RequireAuthMatchers(authRules *[]v1alpha1.RequestAuthRule, autoLoginConfig state.AutoLoginConfig) []v1alpha1.RequestMatcher {
	matchers := v1alpha1.GetRequestMatchers(authRules)

	autoLoginPaths := []string{autoLoginConfig.RedirectPath, autoLoginConfig.LogoutPath}
	if autoLoginConfig.LoginPath != nil {
		autoLoginPaths = append(autoLoginPaths, *autoLoginConfig.LoginPath)
	}
	matchers = append(matchers, v1alpha1.RequestMatcher{
		Paths:   autoLoginPaths,
		Methods: []string{},
	})
	return matchers
}
