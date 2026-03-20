package luascript

import "strings"

// EscapeLuaString escapes characters that have special meaning inside a
// double-quoted Lua string literal. This prevents user-controlled values from
// breaking out of a "..." string when interpolated into the generated Lua
// script.
//
// The following transformations are applied:
//   - \    → \\  (escape the escape character itself first)
//   - "    → \"  (close the surrounding double-quote)
//   - \n   → \\n (newline — could split the string across lines)
//   - \r   → \\r (carriage return)
//   - \t   → \\t (tab)
//   - \v   → \\v (vertical tab — acts as a line terminator in some Lua builds)
//   - \f   → \\f (form feed — acts as a line terminator in some Lua builds)
//   - \x00 →     (removed, null bytes are never valid in a Lua string literal)
func EscapeLuaString(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
		"\v", `\v`,
		"\f", `\f`,
		"\x00", "",
	)
	return r.Replace(s)
}
