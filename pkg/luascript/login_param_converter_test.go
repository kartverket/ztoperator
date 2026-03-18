package luascript_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/stretchr/testify/assert"
)

func TestBuildLuaParams(t *testing.T) {
	t.Run("returns empty table for nil map", func(t *testing.T) {
		result := luascript.ConvertLoginParamsToLuaParams(nil)
		assert.Equal(t, "{}", result)
	})

	t.Run("returns empty table for empty map", func(t *testing.T) {
		result := luascript.ConvertLoginParamsToLuaParams(map[string]string{})
		assert.Equal(t, "{}", result)
	})

	t.Run("single key-value pair", func(t *testing.T) {
		result := luascript.ConvertLoginParamsToLuaParams(map[string]string{
			"foo": "bar",
		})
		assert.Equal(t, `{["foo"]="bar"}`, result)
	})

	t.Run("multiple key-value pairs are sorted by key", func(t *testing.T) {
		result := luascript.ConvertLoginParamsToLuaParams(map[string]string{
			"z_key": "val1",
			"a_key": "val2",
			"m_key": "val3",
		})
		assert.Equal(t, `{["a_key"]="val2",["m_key"]="val3",["z_key"]="val1"}`, result)
	})

	t.Run("spaces in values are encoded as + per application/x-www-form-urlencoded", func(t *testing.T) {
		result := luascript.ConvertLoginParamsToLuaParams(map[string]string{
			"scope": "openid profile email",
		})
		assert.Equal(t, `{["scope"]="openid+profile+email"}`, result)
	})

	t.Run("values with URL-unsafe characters are percent-encoded", func(t *testing.T) {
		result := luascript.ConvertLoginParamsToLuaParams(map[string]string{
			"redirect_uri": "https://example.com/callback?foo=bar",
		})
		assert.Equal(t, `{["redirect_uri"]="https%3A%2F%2Fexample.com%2Fcallback%3Ffoo%3Dbar"}`, result)
	})

	t.Run("key with double quote is escaped to prevent Lua injection", func(t *testing.T) {
		result := luascript.ConvertLoginParamsToLuaParams(map[string]string{
			`bad"]=true} os.execute("x") --`: "val",
		})
		// The double quotes in the key must be escaped so they remain inside the Lua string
		// literal rather than terminating it. The text "os.execute" is still present but is
		// just part of the harmless string key, not executable Lua code.
		assert.Contains(t, result, `\"`)
		assert.Contains(t, result, `["bad\"]=true} os.execute(\"x\") --"]="val"`)
	})

	t.Run("key with backslash is escaped", func(t *testing.T) {
		result := luascript.ConvertLoginParamsToLuaParams(map[string]string{
			`key\with\slashes`: "val",
		})
		assert.Contains(t, result, `key\\with\\slashes`)
	})

	t.Run("value with double quote after URL encoding is still safe", func(t *testing.T) {
		// url.QueryEscape will encode " as %22 which is already safe,
		// but we verify the defense-in-depth escaping works too
		result := luascript.ConvertLoginParamsToLuaParams(map[string]string{
			"key": `value"with"quotes`,
		})
		// url.QueryEscape turns " into %22, so no raw " should appear in the value
		assert.NotContains(t, result, `="value"`)
	})
}
