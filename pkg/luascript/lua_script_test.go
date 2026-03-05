package luascript_test

import (
	"testing"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
)

// mockHandleStub is a self-contained Lua snippet that defines a make_handle()
// factory returning a plain Lua table that satisfies the Envoy handle API used
// by ztoperator.lua:
//
//	handle:headers():get(key)
//	handle:headers():add(key, val)
//	handle:headers():replace(key, val)
//	handle:logCritical(msg)
//
// After calling envoy_on_request / envoy_on_response the test reads results
// directly from the handle.hdrs table.
const mockHandleStub = `
function make_handle(initial_headers)
    local hdrs = {}
    for k, v in pairs(initial_headers or {}) do hdrs[k] = v end

    local headers_obj = {
        get     = function(_, k) return hdrs[k] end,
        add     = function(_, k, v) hdrs[k] = v end,
        replace = function(_, k, v) hdrs[k] = v end,
    }
    return {
        hdrs        = hdrs,
        headers     = function(_) return headers_obj end,
        logCritical = function(_, msg) end,
    }
end
`

func runOnRequest(t *testing.T, script string, requestHeaders map[string]string) map[string]string {
	t.Helper()
	L := lua.NewState()
	defer L.Close()

	require.NoError(t, L.DoString(mockHandleStub))
	require.NoError(t, L.DoString(script))

	handle := buildLuaHandle(t, L, requestHeaders)
	require.NoError(t, L.CallByParam(lua.P{Fn: L.GetGlobal("envoy_on_request"), NRet: 0, Protect: true}, handle))

	return readHeaders(t, L, handle)
}

func runOnResponse(t *testing.T, script string, responseHeaders map[string]string) map[string]string {
	t.Helper()
	L := lua.NewState()
	defer L.Close()

	require.NoError(t, L.DoString(mockHandleStub))
	require.NoError(t, L.DoString(script))

	handle := buildLuaHandle(t, L, responseHeaders)
	require.NoError(t, L.CallByParam(lua.P{Fn: L.GetGlobal("envoy_on_response"), NRet: 0, Protect: true}, handle))

	return readHeaders(t, L, handle)
}

// buildLuaHandle calls make_handle(initial_headers) inside the VM and returns
// the resulting Lua table as an LValue ready to pass to envoy_on_*.
func buildLuaHandle(t *testing.T, L *lua.LState, headers map[string]string) lua.LValue {
	t.Helper()
	initial := L.NewTable()
	for k, v := range headers {
		L.SetField(initial, k, lua.LString(v))
	}
	require.NoError(t, L.CallByParam(lua.P{Fn: L.GetGlobal("make_handle"), NRet: 1, Protect: true}, initial))
	handle := L.Get(-1)
	L.Pop(1)
	return handle
}

// readHeaders extracts the hdrs table from a handle returned by make_handle.
func readHeaders(t *testing.T, L *lua.LState, handle lua.LValue) map[string]string {
	t.Helper()
	tbl, ok := handle.(*lua.LTable)
	require.True(t, ok, "handle is not a Lua table")
	hdrs, ok := L.GetField(tbl, "hdrs").(*lua.LTable)
	require.True(t, ok, "handle.hdrs is not a Lua table")
	result := make(map[string]string)
	hdrs.ForEach(func(k, v lua.LValue) {
		result[k.String()] = v.String()
	})
	return result
}

// --- fixtures ---

func defaultAuthPolicy() *v1alpha1.AuthPolicy {
	return &v1alpha1.AuthPolicy{
		Spec: v1alpha1.AuthPolicySpec{
			Enabled:      true,
			WellKnownURI: "https://idp.example.com/.well-known/openid-configuration",
			Selector:     v1alpha1.WorkloadSelector{MatchLabels: map[string]string{"app": "test"}},
			IgnoreAuthRules: &[]v1alpha1.RequestMatcher{
				{Paths: []string{"/public"}, Methods: []string{}},
			},
			AuthRules: &[]v1alpha1.RequestAuthRule{
				{RequestMatcher: v1alpha1.RequestMatcher{Paths: []string{"/secure"}, Methods: []string{}}},
			},
			AutoLogin: &v1alpha1.AutoLogin{
				Enabled:      true,
				LoginPath:    helperfunctions.Ptr("/login"),
				RedirectPath: helperfunctions.Ptr("/oauth2/callback"),
				LogoutPath:   helperfunctions.Ptr("/logout"),
				Scopes:       []string{"openid"},
			},
		},
	}
}

func defaultAutoLoginConfig() state.AutoLoginConfig {
	return state.AutoLoginConfig{
		Enabled:      true,
		LoginPath:    helperfunctions.Ptr("/login"),
		RedirectPath: "/oauth2/callback",
		LogoutPath:   "/logout",
	}
}

func defaultIdpUris() state.IdentityProviderUris {
	return state.IdentityProviderUris{
		AuthorizationURI: "https://idp.example.com/authorize",
		EndSessionURI:    helperfunctions.Ptr("https://idp.example.com/endsession"),
	}
}

// --- tests ---

// TestGeneratedLuaScript_OnRequest_PublicPath verifies the happy case:
// a request to a public (ignored) path that is not in the require rules
// results in bypass=true being set on the request header, allowing the
// Envoy OAuth2 filter to pass the request through without redirecting.
func TestGeneratedLuaScript_OnRequest_PublicPath(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	handle := runOnRequest(t, script, map[string]string{
		":path":   "/public",
		":method": "GET",
	})

	assert.Equal(t, "true", handle[luascript.BypassOauthLoginHeaderName])
	assert.Equal(t, "false", handle[luascript.DenyRedirectHeaderName])
}

// TestGeneratedLuaScript_OnRequest_SecurePath verifies that a request to a
// path that is in the require rules but not in ignore rules is not bypassed.
func TestGeneratedLuaScript_OnRequest_SecurePath(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	handle := runOnRequest(t, script, map[string]string{
		":path":   "/secure",
		":method": "GET",
	})

	assert.Equal(t, "false", handle[luascript.BypassOauthLoginHeaderName])
	assert.Equal(t, "false", handle[luascript.DenyRedirectHeaderName])
}

// TestGeneratedLuaScript_OnRequest_AutoLoginInfraPaths verifies that the
// auto-login infrastructure paths (redirect, logout, login) are never bypassed,
// even if they would otherwise match an ignore rule.
func TestGeneratedLuaScript_OnRequest_AutoLoginInfraPaths(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	for _, path := range []string{"/oauth2/callback", "/logout", "/login"} {
		t.Run(path, func(t *testing.T) {
			handle := runOnRequest(t, script, map[string]string{
				":path":   path,
				":method": "GET",
			})

			assert.Equal(t, "false", handle[luascript.BypassOauthLoginHeaderName])
			assert.Equal(t, "false", handle[luascript.DenyRedirectHeaderName])
		})
	}
}

// TestGeneratedLuaScript_OnRequest_QueryStringStrippedBeforeMatching verifies
// that query string parameters are stripped from the path before rule matching,
// so /public?foo=bar still matches the /public ignore rule.
func TestGeneratedLuaScript_OnRequest_QueryStringStrippedBeforeMatching(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	handle := runOnRequest(t, script, map[string]string{
		":path":   "/public?foo=bar",
		":method": "GET",
	})

	assert.Equal(t, "true", handle[luascript.BypassOauthLoginHeaderName])
}

// TestGeneratedLuaScript_OnRequest_DenyRedirect verifies that a path configured
// with denyRedirect=true results in x-deny-redirect: true, which instructs the
// Envoy OAuth2 filter to return a 401 instead of redirecting to the IdP.
func TestGeneratedLuaScript_OnRequest_DenyRedirect(t *testing.T) {
	policy := defaultAuthPolicy()
	policy.Spec.AuthRules = &[]v1alpha1.RequestAuthRule{
		{
			RequestMatcher: v1alpha1.RequestMatcher{Paths: []string{"/api"}, Methods: []string{}},
			DenyRedirect:   helperfunctions.Ptr(true),
		},
	}
	script := luascript.GenerateLuaScript(policy, defaultAutoLoginConfig(), defaultIdpUris())

	handle := runOnRequest(t, script, map[string]string{
		":path":   "/api",
		":method": "GET",
	})

	assert.Equal(t, "false", handle[luascript.BypassOauthLoginHeaderName])
	assert.Equal(t, "true", handle[luascript.DenyRedirectHeaderName])
}

// TestGeneratedLuaScript_OnResponse_NonRedirectUnchanged verifies that
// envoy_on_response leaves the location header untouched on non-302 responses.
func TestGeneratedLuaScript_OnResponse_NonRedirectUnchanged(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	handle := runOnResponse(t, script, map[string]string{
		":status":  "200",
		"location": "https://idp.example.com/authorize?client_id=x",
	})

	assert.Equal(t, "https://idp.example.com/authorize?client_id=x", handle["location"])
}

// TestGeneratedLuaScript_OnResponse_LoginParamsAppendedToAuthorizeRedirect
// verifies that login_params are appended to the location header when a 302
// redirects to the authorize endpoint.
func TestGeneratedLuaScript_OnResponse_LoginParamsAppendedToAuthorizeRedirect(t *testing.T) {
	cfg := defaultAutoLoginConfig()
	cfg.LoginParams = map[string]string{"acr_values": "idporten-loa-high"}

	script := luascript.GenerateLuaScript(defaultAuthPolicy(), cfg, defaultIdpUris())

	handle := runOnResponse(t, script, map[string]string{
		":status":  "302",
		"location": "https://idp.example.com/authorize?client_id=myclient",
	})

	assert.Contains(t, handle["location"], "acr_values=idporten-loa-high")
}

// TestGeneratedLuaScript_OnResponse_LoginParamsNotAppendedToUnrelatedRedirect
// verifies that login_params are not injected when the 302 location does not
// point to the authorize endpoint.
func TestGeneratedLuaScript_OnResponse_LoginParamsNotAppendedToUnrelatedRedirect(t *testing.T) {
	cfg := defaultAutoLoginConfig()
	cfg.LoginParams = map[string]string{"acr_values": "idporten-loa-high"}

	script := luascript.GenerateLuaScript(defaultAuthPolicy(), cfg, defaultIdpUris())

	handle := runOnResponse(t, script, map[string]string{
		":status":  "302",
		"location": "https://other.example.com/somewhere",
	})

	assert.NotContains(t, handle["location"], "acr_values")
}

// TestGeneratedLuaScript_OnResponse_PostLogoutRedirectUriAppended verifies
// that the post_logout_redirect_uri is appended to the location header when a
// 302 redirects to the end_session endpoint and a URI is configured.
func TestGeneratedLuaScript_OnResponse_PostLogoutRedirectUriAppended(t *testing.T) {
	cfg := defaultAutoLoginConfig()
	cfg.PostLogoutRedirectURI = helperfunctions.Ptr("https://example.com/logged-out")

	script := luascript.GenerateLuaScript(defaultAuthPolicy(), cfg, defaultIdpUris())

	handle := runOnResponse(t, script, map[string]string{
		":status":  "302",
		"location": "https://idp.example.com/endsession?id_token_hint=abc",
	})

	assert.Contains(t, handle["location"], "post_logout_redirect_uri=https%3A%2F%2Fexample.com%2Flogged-out")
}

// TestGeneratedLuaScript_OnResponse_PostLogoutRedirectUriNotAppendedWhenEmpty
// verifies that no post_logout_redirect_uri parameter is injected when none is
// configured.
func TestGeneratedLuaScript_OnResponse_PostLogoutRedirectUriNotAppendedWhenEmpty(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	handle := runOnResponse(t, script, map[string]string{
		":status":  "302",
		"location": "https://idp.example.com/endsession?id_token_hint=abc",
	})

	assert.NotContains(t, handle["location"], "post_logout_redirect_uri")
}
