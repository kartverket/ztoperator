package luascript_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/stretchr/testify/assert"
)

func TestEscapeLuaString(t *testing.T) {
	t.Run("plain string is unchanged", func(t *testing.T) {
		assert.Equal(t, "hello", luascript.EscapeLuaString("hello"))
	})

	t.Run("backslash is escaped", func(t *testing.T) {
		assert.Equal(t, `foo\\bar`, luascript.EscapeLuaString(`foo\bar`))
	})

	t.Run("double quote is escaped", func(t *testing.T) {
		assert.Equal(t, `foo\"bar`, luascript.EscapeLuaString(`foo"bar`))
	})

	t.Run("newline is escaped", func(t *testing.T) {
		assert.Equal(t, `foo\nbar`, luascript.EscapeLuaString("foo\nbar"))
	})

	t.Run("carriage return is escaped", func(t *testing.T) {
		assert.Equal(t, `foo\rbar`, luascript.EscapeLuaString("foo\rbar"))
	})

	t.Run("null byte is removed", func(t *testing.T) {
		assert.Equal(t, "foobar", luascript.EscapeLuaString("foo\x00bar"))
	})

	t.Run("tab is escaped", func(t *testing.T) {
		assert.Equal(t, `foo\tbar`, luascript.EscapeLuaString("foo\tbar"))
	})

	t.Run("vertical tab is escaped", func(t *testing.T) {
		assert.Equal(t, `foo\vbar`, luascript.EscapeLuaString("foo\vbar"))
	})

	t.Run("form feed is escaped", func(t *testing.T) {
		assert.Equal(t, `foo\fbar`, luascript.EscapeLuaString("foo\fbar"))
	})

	t.Run("combined injection payload is neutralized", func(t *testing.T) {
		// Attempt to break out of a Lua string and inject code
		input := `value") os.execute("evil") --`
		expected := `value\") os.execute(\"evil\") --`
		assert.Equal(t, expected, luascript.EscapeLuaString(input))
	})

	t.Run("backslash before double quote is fully escaped", func(t *testing.T) {
		// A raw \" in the input should become \\\", not just \"
		input := `a\"b`
		expected := `a\\\"b`
		assert.Equal(t, expected, luascript.EscapeLuaString(input))
	})

	t.Run("empty string is unchanged", func(t *testing.T) {
		assert.Equal(t, "", luascript.EscapeLuaString(""))
	})

	t.Run("normal URL is unchanged", func(t *testing.T) {
		url := "https://login.microsoftonline.com/tenant/oauth2/v2.0/authorize"
		assert.Equal(t, url, luascript.EscapeLuaString(url))
	})
}
