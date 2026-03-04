package luascript

import "strings"

const (
	singleStarPlaceholder = "<<SINGLESTAR>>"
	doubleStarPlaceholder = "<<DOUBLESTAR>>"
)

// ConvertRequestMatcherPathToLuaPattern converts a raw request matcher path
// into a fully anchored Lua pattern string for use in string.match calls inside
// the generated EnvoyFilter Lua script.
//
// Examples:
//
//	ConvertRequestMatcherPathToLuaPattern("/api/v1.0/items")  → "^/api/v1%.0/items$"
//	ConvertRequestMatcherPathToLuaPattern("/api/{*}/items")   → "^/api/[^/]+/items$"
//	ConvertRequestMatcherPathToLuaPattern("/api/{**}")        → "^/api/.*$"
//	ConvertRequestMatcherPathToLuaPattern("/api/*")           → "^/api/.*$"
func ConvertRequestMatcherPathToLuaPattern(path string) string {
	return "^" + toLuaPattern(path) + "$"
}

func toLuaPattern(path string) string {
	if strings.ContainsAny(path, "{}") {
		// New path wildcard syntax: strip braces, leaving the wildcard content.
		// {*} → *, {**} → **, api{**} → api**
		path = strings.ReplaceAll(path, "{", "")
		path = strings.ReplaceAll(path, "}", "")
	} else {
		// Old wildcard syntax: a single trailing * means match-any (like **).
		path = strings.ReplaceAll(path, "*", "**")
	}

	path = strings.ReplaceAll(path, "**", doubleStarPlaceholder) // replace double star first
	path = strings.ReplaceAll(path, "*", singleStarPlaceholder)  // replace single star second

	path = escapeLuaPatternChars(path)

	path = strings.ReplaceAll(path, singleStarPlaceholder, "[^/]+")
	path = strings.ReplaceAll(path, doubleStarPlaceholder, ".*")

	return path
}

// escapeLuaPatternChars escapes all characters with a special meaning in Lua
// pattern matching. Must be called AFTER wildcard processing.
func escapeLuaPatternChars(s string) string {
	if strings.ContainsAny(s, "*{}") {
		panic("escapeLuaPatternChars should be called after wildcard processing: " + s)
	}
	replacer := strings.NewReplacer(
		"%", "%%", // escape the escape character itself first to avoid double-escaping
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
