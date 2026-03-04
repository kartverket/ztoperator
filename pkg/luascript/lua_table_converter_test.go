package luascript

import (
	"testing"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestBuildLuaRulesFromMatchers(t *testing.T) {
	t.Run("root path", func(t *testing.T) {
		result := BuildLuaRulesFromMatchers([]v1alpha1.RequestMatcher{
			{Paths: []string{"/"}, Methods: []string{}},
		})
		assert.Equal(t, `{{regex="^/$",methods={}}}`, result)
	})

	t.Run("plain path", func(t *testing.T) {
		result := BuildLuaRulesFromMatchers([]v1alpha1.RequestMatcher{
			{Paths: []string{"/api"}, Methods: []string{"GET"}},
		})
		assert.Equal(t, `{{regex="^/api$",methods={["GET"]=true}}}`, result)
	})

	t.Run("dot in path is escaped to %. so it matches only a literal dot", func(t *testing.T) {
		// Without escaping, Lua's string.match treats '.' as "any character",
		// causing /api/v1X0/items to match as well as /api/v1.0/items.
		result := BuildLuaRulesFromMatchers([]v1alpha1.RequestMatcher{
			{Paths: []string{"/api/v1.0/items"}, Methods: []string{"GET"}},
		})
		assert.Equal(t, `{{regex="^/api/v1%.0/items$",methods={["GET"]=true}}}`, result)
	})

	t.Run("hyphen in path is escaped to %- so it is not treated as lazy repetition", func(t *testing.T) {
		// Without escaping, '-' in a Lua pattern means 0-or-more (lazy), silently
		// matching paths it should not.
		result := BuildLuaRulesFromMatchers([]v1alpha1.RequestMatcher{
			{Paths: []string{"/some-resource"}, Methods: []string{"GET"}},
		})
		assert.Equal(t, `{{regex="^/some%-resource$",methods={["GET"]=true}}}`, result)
	})

	t.Run("plus in path is escaped to %+ so it is not treated as greedy repetition", func(t *testing.T) {
		// '+' is a valid sub-delim (RFC 3986); without escaping it acts as
		// "1-or-more" in a Lua pattern.
		result := BuildLuaRulesFromMatchers([]v1alpha1.RequestMatcher{
			{Paths: []string{"/c++/api"}, Methods: []string{"GET"}},
		})
		assert.Equal(t, `{{regex="^/c%+%+/api$",methods={["GET"]=true}}}`, result)
	})

	t.Run("parentheses in path are escaped to %( %) so they are not capture groups", func(t *testing.T) {
		// '(' and ')' are valid sub-delims (RFC 3986); without escaping they open
		// and close a Lua pattern capture group, corrupting the match.
		result := BuildLuaRulesFromMatchers([]v1alpha1.RequestMatcher{
			{Paths: []string{"/api/(v1)/items"}, Methods: []string{"GET"}},
		})
		assert.Equal(t, `{{regex="^/api/%(v1%)/items$",methods={["GET"]=true}}}`, result)
	})

	t.Run("percent in path is escaped to %% so it is not a pattern escape prefix", func(t *testing.T) {
		// '%' is valid in a path as the start of a pct-encoded triplet (RFC 3986);
		// without escaping it is the Lua pattern escape character, causing the
		// following character to be misread as a pattern class.
		result := BuildLuaRulesFromMatchers([]v1alpha1.RequestMatcher{
			{Paths: []string{"/api/100%25/done"}, Methods: []string{"GET"}},
		})
		assert.Equal(t, `{{regex="^/api/100%%25/done$",methods={["GET"]=true}}}`, result)
	})

	t.Run("single-segment wildcard {*} becomes [^/]+", func(t *testing.T) {
		result := BuildLuaRulesFromMatchers([]v1alpha1.RequestMatcher{
			{Paths: []string{"/api/{*}"}, Methods: []string{"GET"}},
		})
		assert.Equal(t, `{{regex="^/api/[^/]+$",methods={["GET"]=true}}}`, result)
	})

	t.Run("multi-segment wildcard {**} becomes .*", func(t *testing.T) {
		result := BuildLuaRulesFromMatchers([]v1alpha1.RequestMatcher{
			{Paths: []string{"/api/{**}"}, Methods: []string{}},
		})
		assert.Equal(t, `{{regex="^/api/.*$",methods={}}}`, result)
	})

	t.Run("legacy star wildcard becomes .*", func(t *testing.T) {
		result := BuildLuaRulesFromMatchers([]v1alpha1.RequestMatcher{
			{Paths: []string{"/api/*"}, Methods: []string{}},
		})
		assert.Equal(t, `{{regex="^/api/.*$",methods={}}}`, result)
	})
}
