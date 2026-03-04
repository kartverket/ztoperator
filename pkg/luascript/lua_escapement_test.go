package luascript_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/stretchr/testify/assert"
)

func TestEscapeLuaPatternChars(t *testing.T) {
	t.Run("all special characters are escaped", func(t *testing.T) {
		assert.Equal(t, "%^%$%%%(%)%.%[%]%+%- %?", luascript.EscapeLuaPatternChars("^$%().[]+- ?"))
	})

	t.Run("percent is escaped before other special characters to avoid double-escaping", func(t *testing.T) {
		assert.Equal(t, "%%%.%%", luascript.EscapeLuaPatternChars("%.%"))
	})
}

func TestEscapeLuaPatternCharsPanicsOnWildcard(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "asterisk wildcard * suffix",
			input: "/api*",
		},
		{
			name:  "asterisk wildcard * standalone",
			input: "/api/*",
		},
		{
			name:  "match single wildcard {*} standalone",
			input: "/api/{*}",
		},
		{
			name:  "match multi wildcard {**} standalone",
			input: "/api/{**}",
		},
		{
			name:  "match multi wildcard {**} suffix",
			input: "/api{**}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() {
				luascript.EscapeLuaPatternChars(tt.input)
			}, "Should panic when input contains wildcard characters that must be processed first")
		})
	}
}
