package luascript

import "strings"

const (
	singleStarPlaceholder = "<<SINGLESTAR>>"
	doubleStarPlaceholder = "<<DOUBLESTAR>>"
)

func ConvertRequestMatcherPathToRegex(path string) string {
	return "^" + wildcardsToLuaPattern(path) + "$"
}

func wildcardsToLuaPattern(path string) string {
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

	path = EscapeLuaPatternChars(path)

	path = strings.ReplaceAll(path, singleStarPlaceholder, "[^/]+")
	path = strings.ReplaceAll(path, doubleStarPlaceholder, ".*")

	return path
}
