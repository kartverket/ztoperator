package luascript_test

import (
	"testing"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/stretchr/testify/assert"
)

func TestBuildLuaRules(t *testing.T) {
	t.Run("empty slice produces empty Lua table", func(t *testing.T) {
		result := luascript.BuildLuaRules([]v1alpha1.RequestMatcher{})
		assert.Equal(t, "{}", result)
	})

	t.Run("single path, no methods matches all methods", func(t *testing.T) {
		result := luascript.BuildLuaRules([]v1alpha1.RequestMatcher{
			{Paths: []string{"^/api$"}, Methods: []string{}},
		})
		assert.Equal(t, `{{regex="^/api$",methods={}}}`, result)
	})

	t.Run("single path, single method", func(t *testing.T) {
		result := luascript.BuildLuaRules([]v1alpha1.RequestMatcher{
			{Paths: []string{"^/api$"}, Methods: []string{"GET"}},
		})
		assert.Equal(t, `{{regex="^/api$",methods={["GET"]=true}}}`, result)
	})

	t.Run("single path, multiple methods", func(t *testing.T) {
		result := luascript.BuildLuaRules([]v1alpha1.RequestMatcher{
			{Paths: []string{"^/api$"}, Methods: []string{"GET", "POST"}},
		})
		assert.Equal(t, `{{regex="^/api$",methods={["GET"]=true,["POST"]=true}}}`, result)
	})

	t.Run("single matcher with multiple paths produces one entry per path", func(t *testing.T) {
		result := luascript.BuildLuaRules([]v1alpha1.RequestMatcher{
			{Paths: []string{"^/a$", "^/b$"}, Methods: []string{"GET"}},
		})
		// Each path gets its own table entry; both share the same methods.
		assert.Equal(t, `{{regex="^/a$",methods={["GET"]=true}},{regex="^/b$",methods={["GET"]=true}}}`, result)
	})

	t.Run("multiple matchers with different methods", func(t *testing.T) {
		result := luascript.BuildLuaRules([]v1alpha1.RequestMatcher{
			{Paths: []string{"^/read$"}, Methods: []string{"GET"}},
			{Paths: []string{"^/write$"}, Methods: []string{"POST", "PUT"}},
		})
		assert.Equal(
			t,
			`{{regex="^/read$",methods={["GET"]=true}},{regex="^/write$",methods={["POST"]=true,["PUT"]=true}}}`,
			result,
		)
	})

	t.Run("nil methods slice treated the same as empty (all methods)", func(t *testing.T) {
		result := luascript.BuildLuaRules([]v1alpha1.RequestMatcher{
			{Paths: []string{"^/api$"}, Methods: nil},
		})
		assert.Equal(t, `{{regex="^/api$",methods={}}}`, result)
	})

	t.Run("matcher with no paths contributes no entries", func(t *testing.T) {
		result := luascript.BuildLuaRules([]v1alpha1.RequestMatcher{
			{Paths: []string{}, Methods: []string{"GET"}},
		})
		assert.Equal(t, "{}", result)
	})

	t.Run("mix of empty-path and non-empty-path matchers", func(t *testing.T) {
		result := luascript.BuildLuaRules([]v1alpha1.RequestMatcher{
			{Paths: []string{}, Methods: []string{"GET"}},
			{Paths: []string{"^/api$"}, Methods: []string{"POST"}},
		})
		assert.Equal(t, `{{regex="^/api$",methods={["POST"]=true}}}`, result)
	})
}
