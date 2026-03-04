package luascript_test

import (
	"testing"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/stretchr/testify/assert"
)

func TestIgnoreAuthMatchers(t *testing.T) {
	t.Run("nil returns empty slice", func(t *testing.T) {
		assert.Equal(t, []v1alpha1.RequestMatcher{}, luascript.IgnoreAuthMatchers(nil))
	})

	t.Run("non-nil returns the dereferenced slice", func(t *testing.T) {
		matchers := []v1alpha1.RequestMatcher{
			{Paths: []string{"/public"}, Methods: []string{"GET"}},
		}
		assert.Equal(t, matchers, luascript.IgnoreAuthMatchers(&matchers))
	})
}

func TestDenyRedirectMatchers(t *testing.T) {
	t.Run("nil auth rules returns nil slice", func(t *testing.T) {
		assert.Nil(t, luascript.DenyRedirectMatchers(nil))
	})

	t.Run("no rules with DenyRedirect set returns nil slice", func(t *testing.T) {
		rules := []v1alpha1.RequestAuthRule{
			{RequestMatcher: v1alpha1.RequestMatcher{Paths: []string{"/api"}}},
		}
		assert.Nil(t, luascript.DenyRedirectMatchers(&rules))
	})

	t.Run("only rules with DenyRedirect=true are included", func(t *testing.T) {
		rules := []v1alpha1.RequestAuthRule{
			{RequestMatcher: v1alpha1.RequestMatcher{Paths: []string{"/included"}}, DenyRedirect: helperfunctions.Ptr(true)},
			{RequestMatcher: v1alpha1.RequestMatcher{Paths: []string{"/excluded"}}, DenyRedirect: helperfunctions.Ptr(false)},
			{RequestMatcher: v1alpha1.RequestMatcher{Paths: []string{"/also-excluded"}}},
		}
		result := luascript.DenyRedirectMatchers(&rules)
		assert.Equal(t, []v1alpha1.RequestMatcher{{Paths: []string{"/included"}}}, result)
	})

	t.Run("multiple rules with DenyRedirect=true are all included", func(t *testing.T) {
		rules := []v1alpha1.RequestAuthRule{
			{RequestMatcher: v1alpha1.RequestMatcher{Paths: []string{"/a"}}, DenyRedirect: helperfunctions.Ptr(true)},
			{RequestMatcher: v1alpha1.RequestMatcher{Paths: []string{"/b"}}, DenyRedirect: helperfunctions.Ptr(true)},
		}
		result := luascript.DenyRedirectMatchers(&rules)
		assert.Equal(t, []v1alpha1.RequestMatcher{
			{Paths: []string{"/a"}},
			{Paths: []string{"/b"}},
		}, result)
	})
}

func TestRequireAuthMatchers(t *testing.T) {
	baseConfig := state.AutoLoginConfig{
		RedirectPath: "/oauth2/callback",
		LogoutPath:   "/logout",
	}

	t.Run("auto-login infrastructure paths are always appended", func(t *testing.T) {
		result := luascript.RequireAuthMatchers(nil, baseConfig)
		assert.Equal(t, []v1alpha1.RequestMatcher{
			{Paths: []string{"/oauth2/callback", "/logout"}, Methods: []string{}},
		}, result)
	})

	t.Run("optional LoginPath is appended when set", func(t *testing.T) {
		cfg := baseConfig
		cfg.LoginPath = helperfunctions.Ptr("/login")
		result := luascript.RequireAuthMatchers(nil, cfg)
		assert.Equal(t, []v1alpha1.RequestMatcher{
			{Paths: []string{"/oauth2/callback", "/logout", "/login"}, Methods: []string{}},
		}, result)
	})

	t.Run("explicit auth rules are prepended before auto-login paths", func(t *testing.T) {
		rules := []v1alpha1.RequestAuthRule{
			{RequestMatcher: v1alpha1.RequestMatcher{Paths: []string{"/secure"}, Methods: []string{"GET"}}},
		}
		result := luascript.RequireAuthMatchers(&rules, baseConfig)
		assert.Equal(t, []v1alpha1.RequestMatcher{
			{Paths: []string{"/secure"}, Methods: []string{"GET"}},
			{Paths: []string{"/oauth2/callback", "/logout"}, Methods: []string{}},
		}, result)
	})
}
