package validation

import (
	"fmt"
	"strings"
)

func ValidatePaths(paths []string) error {
	for _, path := range paths {
		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("invalid path: %s; must start with '/'", path)
		}
		if strings.Contains(path, "{") {
			if err := validateNewPathSyntax(paths); err != nil {
				return err
			}
			continue
		}
		if strings.Count(path, "*") > 1 ||
			(strings.Contains(path, "*") && (path != "*" && !strings.HasPrefix(path, "*") && !strings.HasSuffix(path, "*"))) {
			return fmt.Errorf("invalid path: %s; '*' must appear only once, be at the start, end, or be '*'", path)
		}
	}
	return nil
}

func validateNewPathSyntax(paths []string) error {
	for _, path := range paths {
		containsPathTemplate := strings.Contains(path, matchOneTemplate) || strings.Contains(path, matchAnyTemplate)
		foundMatchAnyTemplate := false
		// Strip leading and trailing slashes if they exist
		path = strings.Trim(path, "/")
		globs := strings.Split(path, "/")
		for _, glob := range globs {
			// If glob is a supported path template, skip the check
			// If glob is {**}, it must be the last operator in the template
			switch {
			case glob == matchOneTemplate && !foundMatchAnyTemplate:
				continue
			case glob == matchAnyTemplate && !foundMatchAnyTemplate:
				foundMatchAnyTemplate = true
				continue
			case (glob == matchAnyTemplate || glob == matchOneTemplate) && foundMatchAnyTemplate:
				return fmt.Errorf("invalid or unsupported path %s. "+
					"{**} is not the last operator", path)
			}

			// If glob is not a supported path template and contains `{`, or `}` it is invalid.
			// Path is invalid if it contains `{` or `}` beyond a supported path template.
			if strings.ContainsAny(glob, "{}") {
				return fmt.Errorf("invalid or unsupported path %s. "+
					"Contains '{' or '}' beyond a supported path template", path)
			}

			// Validate glob is valid string literal
			// Meets Envoy's valid pchar requirements from https://datatracker.ietf.org/doc/html/rfc3986#appendix-A
			if containsPathTemplate && !validLiteral.MatchString(glob) {
				return fmt.Errorf("invalid or unsupported path %s. "+
					"Contains segment %s with invalid string literal", path, glob)
			}
		}
	}
	return nil
}
