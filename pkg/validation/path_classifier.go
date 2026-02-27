package validation

import (
	"fmt"
	"strings"
)

type pathKind int

const (
	pathKindPlain pathKind = iota
	pathKindTemplate
	pathKindLegacyStar
)

func classifyPath(path string) (pathKind, error) {
	hasCurlyBrackets := strings.ContainsAny(path, "{}")
	hasRawAsterisk := hasRawAsteriskOutsideCurlyBrackets(path)

	if hasRawAsterisk {
		// Cannot mix legacy '*' syntax with curly bracket syntax.
		if hasCurlyBrackets {
			return 0, fmt.Errorf(errBracketsBeyondTemplateFmt, path)
		}
		// Exactly one '*' allowed, and it must be the last character.
		if strings.Count(path, "*") > 1 || (path != "*" && !strings.HasSuffix(path, "*")) {
			return 0, fmt.Errorf("invalid path: %s; '*' must appear only once, be at the end, or be '*'", path)
		}
		return pathKindLegacyStar, nil
	}

	if hasCurlyBrackets {
		return pathKindTemplate, nil
	}

	return pathKindPlain, nil
}

func hasRawAsteriskOutsideCurlyBrackets(path string) bool {
	inInsideBracket := false
	for _, r := range path {
		switch r {
		case '{':
			inInsideBracket = true
		case '}':
			inInsideBracket = false
		case '*':
			if !inInsideBracket {
				return true
			}
		}
	}
	return false
}
