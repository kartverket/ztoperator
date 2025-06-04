package utils

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
)

var (
	MatchOneTemplate = "{*}"
	MatchAnyTemplate = "{**}"

	// Valid pchar from https://datatracker.ietf.org/doc/html/rfc3986#appendix-A
	// pchar = unreserved / pct-encoded / sub-delims / ":" / "@"
	validLiteral = regexp.MustCompile("^[a-zA-Z0-9-._~%!$&'()+,;:@=]+$")
)

func LowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.Requeue:
		return i
	case j.Requeue:
		return j
	case i.RequeueAfter < j.RequeueAfter:
		return i
	default:
		return j
	}
}

func BuildObjectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{"type": "ztoperator.kartverket.no"},
	}
}

func Ptr[T any](v T) *T {
	return &v
}

func ValidatePaths(paths []string) error {
	for _, path := range paths {
		if strings.Contains(path, "{") || strings.Contains(path, "}") {
			if err := validateNewPathSyntax(paths); err != nil {
				return err
			}
			continue
		}
		if strings.Count(path, "*") > 1 || (strings.Contains(path, "*") && !(path == "*" || strings.HasPrefix(path, "*") || strings.HasSuffix(path, "*"))) {
			return fmt.Errorf("invalid path: %s; '*' must appear only once, be at the start, end, or be '*'", path)
		}
	}
	return nil
}

func validateNewPathSyntax(paths []string) error {
	for _, path := range paths {
		containsPathTemplate := strings.Contains(path, MatchOneTemplate) || strings.Contains(path, MatchAnyTemplate)
		foundMatchAnyTemplate := false
		// Strip leading and trailing slashes if they exist
		path = strings.Trim(path, "/")
		globs := strings.Split(path, "/")
		for _, glob := range globs {
			// If glob is a supported path template, skip the check
			// If glob is {**}, it must be the last operator in the template
			if glob == MatchOneTemplate && !foundMatchAnyTemplate {
				continue
			} else if glob == MatchAnyTemplate && !foundMatchAnyTemplate {
				foundMatchAnyTemplate = true
				continue
			} else if (glob == MatchAnyTemplate || glob == MatchOneTemplate) && foundMatchAnyTemplate {
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
