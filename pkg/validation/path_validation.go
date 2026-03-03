package validation

import (
	"fmt"
	"strings"
)

const (
	errBracketsBeyondTemplateFmt = "invalid or unsupported path %s. Contains '{' or '}' beyond a supported path template"
	errMatchAnyNotLastFmt        = "invalid or unsupported path %s. {**} is not the last operator"
	errInvalidLiteralFmt         = "invalid or unsupported path %s. Contains segment %s with invalid string literal"
)

func ValidatePaths(paths []string) error {
	for _, path := range paths {
		if err := validatePath(path); err != nil {
			return err
		}
	}
	return nil
}

func validatePath(path string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("invalid path: %s; must start with '/'", path)
	}

	kind, err := classifyPath(path)
	if err != nil {
		return err
	}

	switch kind {
	case pathKindLegacyStar:
		plainPartOfPath := strings.TrimSuffix(path, "*")
		return validatePlainPathLiterals(plainPartOfPath)
	case pathKindTemplate:
		return validateTemplatePath(path)
	case pathKindPlain:
		return validatePlainPathLiterals(path)
	default:
		return fmt.Errorf("unsupported path kind for path %s", path)
	}
}

func validatePlainPathLiterals(path string) error {
	for _, seg := range toPathSegments(path) {
		if seg == "" {
			continue
		}
		if !validLiteral.MatchString(seg) {
			return fmt.Errorf(errInvalidLiteralFmt, path, seg)
		}
	}
	return nil
}

type templateState struct {
	seenMatchOne   bool
	seenMatchMulti bool
}

func validateTemplatePath(path string) error {
	segments := toPathSegments(path)
	state := templateState{}

	for i, segment := range segments {
		isLastSeg := i == len(segments)-1
		if err := validateTemplatePathSegment(path, segment, isLastSeg, &state); err != nil {
			return err
		}
	}

	return nil
}

func validateTemplatePathSegment(path, segment string, isLastSegment bool, state *templateState) error {
	switch {
	case segment == matchOneTemplate:
		// standalone {*} segment - not allowed after {**}
		if state.seenMatchMulti {
			return fmt.Errorf(errMatchAnyNotLastFmt, path)
		}
		state.seenMatchOne = true

	case segment == matchAnyTemplate:
		// standalone {**} segment - not allowed after {**}
		if state.seenMatchMulti {
			return fmt.Errorf(errMatchAnyNotLastFmt, path)
		}
		state.seenMatchMulti = true

	case strings.HasSuffix(segment, matchAnyTemplate):
		// prefix{**} — a literal prefix followed by {**}, e.g. "api{**}".
		// Not allowed after any other template operator, and must be the last segment.
		prefix := strings.TrimSuffix(segment, matchAnyTemplate)
		if strings.ContainsAny(prefix, "{}") {
			return fmt.Errorf(errBracketsBeyondTemplateFmt, path)
		}
		if !validLiteral.MatchString(prefix) {
			return fmt.Errorf(errInvalidLiteralFmt, path, segment)
		}
		if !isLastSegment {
			return fmt.Errorf(errMatchAnyNotLastFmt, path)
		}
		if state.seenMatchMulti || state.seenMatchOne {
			return fmt.Errorf(errMatchAnyNotLastFmt, path)
		}
		state.seenMatchMulti = true

	case strings.ContainsAny(segment, "{}"):
		// Any other use of braces is not supported.
		return fmt.Errorf(errBracketsBeyondTemplateFmt, path)

	default:
		// Plain literal segment — validate that it contains only allowed characters.
		if !validLiteral.MatchString(segment) {
			return fmt.Errorf(errInvalidLiteralFmt, path, segment)
		}
	}
	return nil
}

func toPathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}
