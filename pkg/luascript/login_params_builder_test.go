package luascript_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/stretchr/testify/assert"
)

func TestBuildLuaParams(t *testing.T) {
	t.Run("returns empty table for nil map", func(t *testing.T) {
		result := luascript.BuildLuaParams(nil)
		assert.Equal(t, "{}", result)
	})

	t.Run("returns empty table for empty map", func(t *testing.T) {
		result := luascript.BuildLuaParams(map[string]string{})
		assert.Equal(t, "{}", result)
	})

	t.Run("single key-value pair", func(t *testing.T) {
		result := luascript.BuildLuaParams(map[string]string{
			"foo": "bar",
		})
		assert.Equal(t, `{["foo"]="bar"}`, result)
	})

	t.Run("multiple key-value pairs are sorted by key", func(t *testing.T) {
		result := luascript.BuildLuaParams(map[string]string{
			"z_key": "val1",
			"a_key": "val2",
			"m_key": "val3",
		})
		assert.Equal(t, `{["a_key"]="val2",["m_key"]="val3",["z_key"]="val1"}`, result)
	})

	t.Run("spaces in values are encoded as + per application/x-www-form-urlencoded", func(t *testing.T) {
		result := luascript.BuildLuaParams(map[string]string{
			"scope": "openid profile email",
		})
		assert.Equal(t, `{["scope"]="openid+profile+email"}`, result)
	})

	t.Run("values with URL-unsafe characters are percent-encoded", func(t *testing.T) {
		result := luascript.BuildLuaParams(map[string]string{
			"redirect_uri": "https://example.com/callback?foo=bar",
		})
		assert.Equal(t, `{["redirect_uri"]="https%3A%2F%2Fexample.com%2Fcallback%3Ffoo%3Dbar"}`, result)
	})
}
