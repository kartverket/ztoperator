package luascript_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/stretchr/testify/assert"
)

func TestConvertRequestMatcherPathToRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		reason   string
	}{
		// Simple paths
		{
			name:     "root",
			input:    "/",
			expected: "^/$",
			reason:   "Root path should be anchored",
		},
		{
			name:     "simple path",
			input:    "/api",
			expected: "^/api$",
			reason:   "Simple paths should be anchored",
		},
		{
			name:     "multi-level path",
			input:    "/api/users/list",
			expected: "^/api/users/list$",
			reason:   "Multi-level paths should be anchored",
		},

		// Symbols that require escaping in Lua patterns
		{
			name:     "path with all lua-pattern special characters",
			input:    "/%^$.+-?[]()",
			expected: "^/%%%^%$%.%+%-%?%[%]%(%)$",
			reason:   "All Lua pattern magic characters must be escaped",
		},

		// Paths with legacy wildcards
		{
			name:     "path with single legacy wildcard",
			input:    "/api*",
			expected: "^/api.*$",
			reason:   "Single legacy wildcard matches multiple path segments",
		},
		{
			name:     "path with single legacy wildcard as standalone segment",
			input:    "/api/*",
			expected: "^/api/.*$",
			reason:   "Single legacy wildcard matches multiple path segments",
		},

		// Paths with wildcard operators
		{
			name:     "path with single-segment wildcard as standalone end segment",
			input:    "/api/{*}",
			expected: "^/api/[^/]+$",
			reason:   "Single-segment end wildcard matches a single path segment",
		},
		{
			name:     "path with single-segment wildcard as standalone middle",
			input:    "/api/{*}/moreapi",
			expected: "^/api/[^/]+/moreapi$",
			reason:   "Single-segment middle wildcard matches a single path segment",
		},
		{
			name:     "path with multi-segment wildcard as suffix",
			input:    "/api{**}",
			expected: "^/api.*$",
			reason:   "Single-segment suffix wildcard matches multiple path segments",
		},
		{
			name:     "path with multi-segment wildcard as standalone end segment",
			input:    "/api/{**}",
			expected: "^/api/.*$",
			reason:   "Multi-segment end wildcard matches multiple path segments",
		},
		{
			name:     "path with multi-segment wildcard as standalone middle segment",
			input:    "/api/{**}/moreapi",
			expected: "^/api/.*/moreapi$",
			reason:   "Multi-segment middle wildcard matches multiple path segments",
		},
		{
			name:     "path with single-segment and multi-segment wildcard as standalone segments",
			input:    "/api/{*}/moreapi/{**}",
			expected: "^/api/[^/]+/moreapi/.*$",
			reason:   "Multi-segment middle wildcard matches multiple path segments",
		},

		// Escaping correctness
		{
			name:     "all lua-pattern special characters in a single path are each escaped",
			input:    "/%^$%().[]+-?",
			expected: "^/%%%^%$%%%(%)%.%[%]%+%-%?$",
			reason:   "Every Lua magic character must be individually escaped",
		},
		{
			name:     "percent is escaped before other characters to avoid double-escaping",
			input:    "/%.%",
			expected: "^/%%%.%%$",
			reason:   "If % were not escaped first, a %X sequence would incorrectly become %%X instead of %%%X",
		},
		{
			name:     "path with single-segment wildcard, suffix multi-segment wildcard, and special characters",
			input:    "/api.v1/{*}/items+(v2){**}",
			expected: "^/api%.v1/[^/]+/items%+%(v2%).*$",
			reason:   "Special characters in literal segments must be escaped while wildcards expand correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := luascript.ConvertRequestMatcherPathToLuaPattern(tt.input)
			assert.Equal(t, tt.expected, result, "Reason: %s", tt.reason)
		})
	}
}
