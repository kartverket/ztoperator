package configpatch

import (
	"fmt"
	"net/url"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
	"istio.io/api/networking/v1alpha3"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
)

const (
	BypassOauthLoginHeaderName = "x-bypass-login"
	DenyRedirectHeaderName     = "x-deny-redirect"
)

func GetLuaScriptConfigPatch(scope *state.Scope) (*v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch, error) {
	var ignoreAuthRequestMatchers []v1alpha1.RequestMatcher
	if scope.AuthPolicy.Spec.IgnoreAuthRules != nil {
		ignoreAuthRequestMatchers = append(ignoreAuthRequestMatchers, *scope.AuthPolicy.Spec.IgnoreAuthRules...)
	}

	var denyRedirectMatchers []v1alpha1.RequestMatcher
	if scope.AuthPolicy.Spec.AuthRules != nil {
		for _, authRule := range *scope.AuthPolicy.Spec.AuthRules {
			if authRule.DenyRedirect != nil && *authRule.DenyRedirect {
				denyRedirectMatchers = append(denyRedirectMatchers, authRule.RequestMatcher)
			}
		}
	}

	luaScript, structPbErr := structpb.NewStruct(getLuaScript(
		ignoreAuthRequestMatchers,
		v1alpha1.GetRequestMatchers(
			scope.AuthPolicy.Spec.AuthRules,
		),
		denyRedirectMatchers,
		scope.AutoLoginConfig.LoginPath,
		scope.AutoLoginConfig.RedirectPath,
		scope.AutoLoginConfig.LogoutPath,
		scope.IdentityProviderUris.AuthorizationURI,
		scope.AutoLoginConfig.LoginParams,
	))
	if structPbErr != nil {
		return nil, fmt.Errorf(
			"failed to serialize Custom Lua Script to protobuf struct for AuthPolicy %s/%s due to the following error: %s",
			scope.AuthPolicy.Namespace,
			scope.AuthPolicy.Name,
			structPbErr.Error(),
		)
	}
	return &v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: v1alpha3.EnvoyFilter_HTTP_FILTER,
		Match: &v1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: v1alpha3.EnvoyFilter_SIDECAR_INBOUND,
			ObjectTypes: &v1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
				Listener: &v1alpha3.EnvoyFilter_ListenerMatch{
					FilterChain: &v1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
						Filter: &v1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
							Name: "envoy.filters.network.http_connection_manager",
						},
					},
				},
			},
		},
		Patch: &v1alpha3.EnvoyFilter_Patch{
			Operation: v1alpha3.EnvoyFilter_Patch_INSERT_BEFORE,
			Value:     luaScript,
		},
	}, nil
}

func getLuaScript(
	ignoreAuth,
	requireAuth,
	denyRedirect []v1alpha1.RequestMatcher,
	loginPath *string,
	redirectPath,
	logoutPath string,
	authorizeEndpoint string,
	loginParams map[string]string,
) map[string]interface{} {
	requireAuthOAuthPaths := []string{
		redirectPath, logoutPath,
	}
	if loginPath != nil {
		requireAuthOAuthPaths = append(requireAuthOAuthPaths, *loginPath)
	}
	requireAuth = append(requireAuth, v1alpha1.RequestMatcher{
		Paths:   requireAuthOAuthPaths,
		Methods: []string{},
	})

	ignoreAuth = convertToRE2Regex(ignoreAuth)
	requireAuth = convertToRE2Regex(requireAuth)
	denyRedirect = convertToRE2Regex(denyRedirect)

	// Produce equivalent Lua tables that the generated script can iterate over.
	ignoreRulesLua := buildLuaRules(ignoreAuth)
	requireRulesLua := buildLuaRules(requireAuth)
	denyRedirectRulesLua := buildLuaRules(denyRedirect)

	loginParamsAsLua := buildLuaParams(encodeLoginParams(loginParams))

	// Build the Lua script. We embed the two rule‑tables and a helper matcher.
	luaScript := fmt.Sprintf(`
local ignore_rules = %s
local require_rules = %s
local deny_redirect_rules = %s
local authorize_endpoint = "%s"
local login_params = %s

-- returns true when {p,m} matches any rule in the supplied table
local function match(rules, p, m)
  for _, rule in ipairs(rules) do
    if string.match(p, rule.regex) then
      -- empty "methods" table == all methods
      if next(rule.methods) == nil or rule.methods[m] then
        return true
      end
    end
  end
  return false
end

-- returns true if {p,m} is in ignore_rules *and* NOT in require_rules
local function should_bypass(p, m)
  local bypass = false
  if p ~= "" and m ~= "" then
    -- bypass only when it is in ignore_rules *and* NOT in require_rules
    if match(ignore_rules, p, m) and not match(require_rules, p, m) then
      bypass = true
    end
  end
  return bypass
end

-- returns true if {p,m} is in deny_redirect_rules
local function should_deny_redirect(p, m)
  local deny_redirect = false
  if p ~= "" and m ~= "" then
    -- deny redirect only when it is in deny_redirect_rules
    if match(deny_redirect_rules, p, m) then
      deny_redirect = true
    end
  end
  return deny_redirect
end

local function is_empty_table(t)
  return type(t) ~= "table" or next(t) == nil
end

function envoy_on_request(request_handle)
  local p = request_handle:headers():get(":path")   or ""
  local m = request_handle:headers():get(":method") or ""
  
  local bypass = should_bypass(p, m)
  request_handle:logCritical("Login bypassed?: " .. tostring(bypass))
  request_handle:headers():add("%s", tostring(bypass))
  
  local deny_redirect = should_deny_redirect(p, m)
  request_handle:logCritical("Deny redirect?: " .. tostring(deny_redirect))	
  request_handle:headers():add("%s", tostring(deny_redirect))
end

function envoy_on_response(response_handle)
  local status = response_handle:headers():get(":status") or ""
  if status == "302" and not is_empty_table(login_params) then
    local loc = response_handle:headers():get("location") or ""
    if loc ~= "" then
      if string.sub(loc, 1, #authorize_endpoint) == authorize_endpoint then
		local base, qs = loc:match("^([^?]+)%%??(.*)$")
		local filtered = {}
		if qs ~= "" then
		    local params = {}
			for key, val in string.gmatch(qs, "([^&=?]+)=([^&=?]+)") do
			  params[key] = val
			end
			for k, v in pairs(login_params) do
			  params[k] = v
		  	end
			for k, v in pairs(params) do
			  table.insert(filtered, k .. "=" .. v)
			end
		end
		
		local new_qs = table.concat(filtered, "&")
		local new_url = base .. (new_qs ~= "" and ("?" .. new_qs) or "")
		response_handle:headers():replace("location", new_url)
	  end	
    end
  end
end
`, ignoreRulesLua, requireRulesLua, denyRedirectRulesLua, authorizeEndpoint, loginParamsAsLua, BypassOauthLoginHeaderName, DenyRedirectHeaderName)

	return map[string]interface{}{
		"name": "envoy.filters.http.lua",
		"typed_config": map[string]interface{}{
			"@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua",
			"default_source_code": map[string]interface{}{
				"inline_string": luaScript,
			},
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
	if params == nil || len(params) == 0 {
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
			pathAsRE2Regex = append(pathAsRE2Regex, "^"+ConvertRequestMatcherPathToRegex(path))
		}
		result = append(result, v1alpha1.RequestMatcher{
			Paths:   pathAsRE2Regex,
			Methods: matcher.Methods,
		})
	}
	return result
}

func ConvertRequestMatcherPathToRegex(path string) string {
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
