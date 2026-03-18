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

func defaultAuthPolicy() *v1alpha1.AuthPolicy {
	return &v1alpha1.AuthPolicy{
		Spec: v1alpha1.AuthPolicySpec{
			Enabled:      true,
			WellKnownURI: "https://idp.example.com/.well-known/openid-configuration",
			Selector:     v1alpha1.WorkloadSelector{MatchLabels: map[string]string{"app": "test"}},
			IgnoreAuthRules: &[]v1alpha1.RequestMatcher{
				{Paths: []string{"/public"}, Methods: []string{"GET"}},
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

func TestGeneratedLuaScript_OnRequest_IgnoreAuthRules_BypassAsExpected(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	ignoredMethodHandle := runOnRequest(t, script, map[string]string{
		":path":   "/public", // ignored rule in defaultAuthPolicy
		":method": "GET",     // ignored path in defaultAuthPolicy
	})

	assert.Equal(t, "true", ignoredMethodHandle[luascript.BypassOauthLoginHeaderName])
	assert.Equal(t, "false", ignoredMethodHandle[luascript.DenyRedirectHeaderName])

	nonIgnoredMethodHandle := runOnRequest(t, script, map[string]string{
		":path":   "/public", // ignored path in defaultAuthPolicy
		":method": "POST",    // not ignored method in defaultAuthPolicy
	})

	assert.Equal(t, "false", nonIgnoredMethodHandle[luascript.BypassOauthLoginHeaderName])
	assert.Equal(t, "false", nonIgnoredMethodHandle[luascript.DenyRedirectHeaderName])
}

func TestGeneratedLuaScript_OnRequest_AuthRules_DoNotBypass(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	authRuleHandle := runOnRequest(t, script, map[string]string{
		":path":   "/secure",
		":method": "GET",
	})

	assert.Equal(t, "false", authRuleHandle[luascript.BypassOauthLoginHeaderName])
	assert.Equal(t, "false", authRuleHandle[luascript.DenyRedirectHeaderName])

	noRuleHandle := runOnRequest(t, script, map[string]string{
		":path":   "/noRulesHere",
		":method": "GET",
	})

	assert.Equal(t, "false", noRuleHandle[luascript.BypassOauthLoginHeaderName])
	assert.Equal(t, "false", noRuleHandle[luascript.DenyRedirectHeaderName])
}

func TestGeneratedLuaScript_OnRequest_AutoLoginPaths_DoNotBypass(t *testing.T) {
	autoLoginConfig := defaultAutoLoginConfig()
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), autoLoginConfig, defaultIdpUris())

	autoLoginPaths := []string{autoLoginConfig.RedirectPath, autoLoginConfig.LogoutPath, *autoLoginConfig.LoginPath}
	for _, path := range autoLoginPaths {
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

func TestGeneratedLuaScript_OnRequest_QueryStringStrippedBeforeMatching(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	handle := runOnRequest(t, script, map[string]string{
		":path":   "/public?foo=bar", // ignore rule in defaultAuthPolicy for /public
		":method": "GET",             // ignored path in defaultAuthPolicy
	})

	assert.Equal(t, "true", handle[luascript.BypassOauthLoginHeaderName])
}

func TestGeneratedLuaScript_OnRequest_DenyRedirectDoesNotRedirect(t *testing.T) {
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

func TestGeneratedLuaScript_OnResponse_DoesNotRedirectNon302Responses(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	handle := runOnResponse(t, script, map[string]string{
		":status":  "200",
		"location": "https://idp.example.com/authorize?client_id=x",
	})

	assert.Equal(t, "https://idp.example.com/authorize?client_id=x", handle["location"])
}

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

func TestGeneratedLuaScript_OnResponse_PostLogoutRedirectUriNotAppendedWhenEmpty(t *testing.T) {
	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), defaultIdpUris())

	handle := runOnResponse(t, script, map[string]string{
		":status":  "302",
		"location": "https://idp.example.com/endsession?id_token_hint=abc",
	})

	assert.NotContains(t, handle["location"], "post_logout_redirect_uri")
}

func TestGeneratedLuaScript_OnRequest_SingleSegmentWildcard_MatchesOneSegment(t *testing.T) {
	policy := defaultAuthPolicy()
	policy.Spec.IgnoreAuthRules = &[]v1alpha1.RequestMatcher{
		{Paths: []string{"/api/{*}/items"}, Methods: []string{}},
	}
	script := luascript.GenerateLuaScript(policy, defaultAutoLoginConfig(), defaultIdpUris())

	handle := runOnRequest(t, script, map[string]string{":path": "/api/v1/items", ":method": "GET"})
	assert.Equal(t, "true", handle[luascript.BypassOauthLoginHeaderName], "/api/v1/items should be public")

	invalidPaths := []string{
		"/api/v1/extra/items",
		"/api/spe(c)ial-c?haracters/extra/items",
	}
	for _, path := range invalidPaths {
		t.Run(path, func(t *testing.T) {
			handle = runOnRequest(t, script, map[string]string{":path": path, ":method": "GET"})
			assert.Equal(t, "false", handle[luascript.BypassOauthLoginHeaderName], "%s should not match {*}", path)
		})
	}
}

func TestGeneratedLuaScript_OnRequest_DoubleStarWildcard_MatchesMultipleSegments(t *testing.T) {
	policy := defaultAuthPolicy()
	policy.Spec.IgnoreAuthRules = &[]v1alpha1.RequestMatcher{
		{Paths: []string{"/api/{**}"}, Methods: []string{}},
	}
	script := luascript.GenerateLuaScript(policy, defaultAutoLoginConfig(), defaultIdpUris())

	validPaths := []string{
		"/api/",
		"/api/v1",
		"/api/v1/users",
		"/api/v1/users/123",
		"/api/v1/spe(c)ial-c?haracters/123",
	}
	for _, path := range validPaths {
		t.Run(path, func(t *testing.T) {
			handle := runOnRequest(t, script, map[string]string{":path": path, ":method": "GET"})
			assert.Equal(t, "true", handle[luascript.BypassOauthLoginHeaderName], "%s should match /api/{**}", path)
		})
	}
}

func TestGeneratedLuaScript_OnRequest_LegacyStarWildcard_MatchesMultipleSegments(t *testing.T) {
	policy := defaultAuthPolicy()
	policy.Spec.IgnoreAuthRules = &[]v1alpha1.RequestMatcher{
		{Paths: []string{"/api*"}, Methods: []string{}},
	}
	script := luascript.GenerateLuaScript(policy, defaultAutoLoginConfig(), defaultIdpUris())

	validPaths := []string{
		"/api",
		"/api/",
		"/api/v1",
		"/api/v1/users",
		"/api/v1/users/123",
		"/api/v1/spe(c)ial-c?haracters/123",
	}
	for _, path := range validPaths {
		t.Run(path, func(t *testing.T) {
			handle := runOnRequest(t, script, map[string]string{":path": path, ":method": "GET"})
			assert.Equal(t, "true", handle[luascript.BypassOauthLoginHeaderName], "%s should match /api/{**}", path)
		})
	}
}

func TestGeneratedLuaScript_InjectionInLoginParamKey_ProducesValidLua(t *testing.T) {
	cfg := defaultAutoLoginConfig()
	cfg.LoginParams = map[string]string{
		`evil"]=true} os.execute("rm -rf /") --`: "value",
	}

	script := luascript.GenerateLuaScript(defaultAuthPolicy(), cfg, defaultIdpUris())

	// The generated script must be valid Lua that can be loaded without error
	L := lua.NewState()
	defer L.Close()

	require.NoError(t, L.DoString(mockHandleStub))
	require.NoError(t, L.DoString(script), "generated Lua script should be syntactically valid despite injection attempt in key")
}

func TestGeneratedLuaScript_InjectionInLoginParamValue_ProducesValidLua(t *testing.T) {
	cfg := defaultAutoLoginConfig()
	cfg.LoginParams = map[string]string{
		"acr_values": `high" os.execute("evil") --`,
	}

	script := luascript.GenerateLuaScript(defaultAuthPolicy(), cfg, defaultIdpUris())

	L := lua.NewState()
	defer L.Close()

	require.NoError(t, L.DoString(mockHandleStub))
	require.NoError(t, L.DoString(script), "generated Lua script should be syntactically valid despite injection attempt in value")
}

func TestGeneratedLuaScript_InjectionInAuthorizationURI_ProducesValidLua(t *testing.T) {
	idpUris := defaultIdpUris()
	idpUris.AuthorizationURI = `https://evil.com/authorize?x=1" os.execute("evil") --`

	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), idpUris)

	L := lua.NewState()
	defer L.Close()

	require.NoError(t, L.DoString(mockHandleStub))
	require.NoError(t, L.DoString(script), "generated Lua script should be syntactically valid despite injection attempt in authorization URI")
}

func TestGeneratedLuaScript_InjectionInEndSessionURI_ProducesValidLua(t *testing.T) {
	idpUris := defaultIdpUris()
	idpUris.EndSessionURI = helperfunctions.Ptr(`https://evil.com/endsession?x=1" os.execute("evil") --`)

	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), idpUris)

	L := lua.NewState()
	defer L.Close()

	require.NoError(t, L.DoString(mockHandleStub))
	require.NoError(t, L.DoString(script), "generated Lua script should be syntactically valid despite injection attempt in end-session URI")
}

func TestGeneratedLuaScript_InjectionInMethod_ProducesValidLua(t *testing.T) {
	policy := defaultAuthPolicy()
	policy.Spec.IgnoreAuthRules = &[]v1alpha1.RequestMatcher{
		{
			Paths:   []string{"/public"},
			Methods: []string{`GET"]=true} os.execute("rm -rf /") --`},
		},
	}

	script := luascript.GenerateLuaScript(policy, defaultAutoLoginConfig(), defaultIdpUris())

	L := lua.NewState()
	defer L.Close()

	require.NoError(t, L.DoString(mockHandleStub))
	require.NoError(t, L.DoString(script), "generated Lua script should be syntactically valid despite injection attempt in HTTP method")
}

func TestGeneratedLuaScript_NilEndSessionURI_ProducesValidLua(t *testing.T) {
	idpUris := defaultIdpUris()
	idpUris.EndSessionURI = nil

	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), idpUris)

	L := lua.NewState()
	defer L.Close()

	require.NoError(t, L.DoString(mockHandleStub))
	require.NoError(t, L.DoString(script), "generated Lua script should be valid when EndSessionURI is nil")
}

func TestGeneratedLuaScript_NilEndSessionURI_LogoutRedirectSkipped(t *testing.T) {
	idpUris := defaultIdpUris()
	idpUris.EndSessionURI = nil

	script := luascript.GenerateLuaScript(defaultAuthPolicy(), defaultAutoLoginConfig(), idpUris)

	// A 302 to some arbitrary location must pass through unmodified when
	// end_session_endpoint is "" (the Lua string.sub guard never matches).
	headers := runOnResponse(t, script, map[string]string{
		":status":  "302",
		"location": "https://other.example.com/somewhere",
	})

	assert.Equal(t, "https://other.example.com/somewhere", headers["location"])
}
