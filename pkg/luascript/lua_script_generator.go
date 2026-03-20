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
var luaScriptTemplate string

// GenerateLuaScript produces the Lua source code that is embedded as an inline
// Envoy Lua filter inside the generated EnvoyFilter resource.
//
// The Lua filter runs inside the Envoy sidecar on every inbound HTTP request
// and response, and acts as a pre-processing layer for the Envoy OAuth2 filter
// that sits immediately after it in the filter chain:
//
//   - On request: the script evaluates the request path and method against the
//     configured ignore, require, and deny-redirect rules, then sets two
//     synthetic headers that the OAuth2 filter reads to decide how to handle
//     the request:
//
//   - x-bypass-login: "true"  — the OAuth2 filter lets the request through
//     without requiring authentication (used for public paths).
//
//   - x-deny-redirect: "true" — the OAuth2 filter returns a 401 instead of
//     redirecting to the IdP (used for API paths where a browser redirect
//     would be inappropriate).
//
//   - On response: the script intercepts 302 redirects produced by the OAuth2
//     filter and rewrites the Location header:
//
//   - Redirects to the authorize endpoint have any configured loginParams
//     (e.g. acr_values, ui_locales) merged into the query string.
//
//   - Redirects to the end-session endpoint have the postLogoutRedirectUri
//     appended as a query parameter when one is configured.
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

	var endSessionURI string
	if identityProviderUris.EndSessionURI != nil {
		endSessionURI = *identityProviderUris.EndSessionURI
	} else {
		// We handle endSessionURI == nil as "" to make it easier when building the Lua script
		endSessionURI = ""
	}

	return fmt.Sprintf(
		luaScriptTemplate,
		ignoreRulesLua,
		requireRulesLua,
		denyRedirectRulesLua,
		EscapeLuaString(identityProviderUris.AuthorizationURI),
		loginParamsAsLua,
		EscapeLuaString(endSessionURI),
		EscapeLuaString(queryEscapedPostLogoutRedirectURI),
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

func RequireAuthMatchers(
	authRules *[]v1alpha1.RequestAuthRule,
	autoLoginConfig state.AutoLoginConfig,
) []v1alpha1.RequestMatcher {
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
