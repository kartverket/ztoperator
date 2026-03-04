package luascript

import "strings"

// EscapeLuaPatternChars escapes all characters with a special meaning in Lua pattern matching
//
// Must be called AFTER wildcard processing.
func EscapeLuaPatternChars(s string) string {
	if strings.ContainsAny(s, "*{}") {
		panic("escapeLuaPatternChars should be called after wildcard processing: " + s)
	}
	replacer := strings.NewReplacer(
		"%", "%%", // escape the escape character itself first to avoid double-escaping for other characters
		"^", "%^", // pattern anchor
		"$", "%$", // pattern anchor
		".", "%.", // any character
		"+", "%+", // 1 or more repetitions (greedy)
		"-", "%-", // 0 or more repetitions (lazy)
		"?", "%?", // 0 or 1 occurrence
		"[", "%[", // character class start
		"]", "%]", // character class end
		"(", "%(", // capture group start
		")", "%)", // capture group end
	)
	return replacer.Replace(s)
}
