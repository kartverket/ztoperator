package luascript_test

import (
	"testing"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/stretchr/testify/assert"
)

func TestConvertRequestMatchersToLuaTableString(t *testing.T) {
	t.Run("empty slice produces empty Lua table", func(t *testing.T) {
		paths := []string{}
		methods := []string{}
		expected := "{}"

		result := luascript.ConvertRequestMatchersToLuaTableString([]v1alpha1.RequestMatcher{{Paths: paths, Methods: methods}})
		assert.Equal(t, expected, result)
	})

	t.Run("root path", func(t *testing.T) {
		paths := []string{"/"}
		methods := []string{}
		expected := `{{regex="^/$",methods={}}}`

		result := luascript.ConvertRequestMatchersToLuaTableString([]v1alpha1.RequestMatcher{{Paths: paths, Methods: methods}})
		assert.Equal(t, expected, result)
	})

	t.Run("plain path, no methods matches all methods", func(t *testing.T) {
		paths := []string{"/api"}
		methods := []string{}
		expected := `{{regex="^/api$",methods={}}}`

		result := luascript.ConvertRequestMatchersToLuaTableString([]v1alpha1.RequestMatcher{{Paths: paths, Methods: methods}})
		assert.Equal(t, expected, result)
	})

	t.Run("plain path, single method", func(t *testing.T) {
		paths := []string{"/api"}
		methods := []string{"GET"}
		expected := `{{regex="^/api$",methods={["GET"]=true}}}`

		result := luascript.ConvertRequestMatchersToLuaTableString([]v1alpha1.RequestMatcher{{Paths: paths, Methods: methods}})
		assert.Equal(t, expected, result)
	})

	t.Run("plain path, multiple methods", func(t *testing.T) {
		paths := []string{"/api"}
		methods := []string{"GET", "POST"}
		expected := `{{regex="^/api$",methods={["GET"]=true,["POST"]=true}}}`

		result := luascript.ConvertRequestMatchersToLuaTableString([]v1alpha1.RequestMatcher{{Paths: paths, Methods: methods}})
		assert.Equal(t, expected, result)
	})

	t.Run("special characters in paths are escaped", func(t *testing.T) {
		paths := []string{"/api.v1/some-resource/c++/(v1)/100%25"}
		methods := []string{"GET"}
		expected := `{{regex="^/api%.v1/some%-resource/c%+%+/%(v1%)/100%%25$",methods={["GET"]=true}}}`

		result := luascript.ConvertRequestMatchersToLuaTableString([]v1alpha1.RequestMatcher{{Paths: paths, Methods: methods}})
		assert.Equal(t, expected, result)
	})

	t.Run("single-segment wildcard {*} matches single segment", func(t *testing.T) {
		paths := []string{"/api/{*}"}
		methods := []string{"GET"}
		expected := `{{regex="^/api/[^/]+$",methods={["GET"]=true}}}`

		result := luascript.ConvertRequestMatchersToLuaTableString([]v1alpha1.RequestMatcher{{Paths: paths, Methods: methods}})
		assert.Equal(t, expected, result)
	})

	t.Run("multi-segment wildcard {**} matches multiple segments", func(t *testing.T) {
		paths := []string{"/api/{**}"}
		methods := []string{}
		expected := `{{regex="^/api/.*$",methods={}}}`

		result := luascript.ConvertRequestMatchersToLuaTableString([]v1alpha1.RequestMatcher{{Paths: paths, Methods: methods}})
		assert.Equal(t, expected, result)
	})

	t.Run("legacy star wildcard matches multiple segments", func(t *testing.T) {
		paths := []string{"/api/*"}
		methods := []string{}
		expected := `{{regex="^/api/.*$",methods={}}}`

		result := luascript.ConvertRequestMatchersToLuaTableString([]v1alpha1.RequestMatcher{{Paths: paths, Methods: methods}})
		assert.Equal(t, expected, result)
	})
}
