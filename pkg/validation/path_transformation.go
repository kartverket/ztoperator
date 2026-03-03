package validation

import "strings"

// TransformPathsForIstio converts paths with non-standalone {**} suffix to Istio-compatible * suffix.
// Assumes the input paths have already been validated with validation.ValidatePaths.
// Specifically, transforms paths with {**} suffix to * suffix, if and only if:
// - the {**} is not a standalone segment
// - there are no other template operators, e.g. {*} or {**}, in the path
//
// E.g. "/api{**}" -> "/api*", but "/api/{**}" and "/api/{*}/test{**}" remain unchanged.
func TransformPathsForIstio(validatedPaths []string) []string {
	transformed := make([]string, len(validatedPaths))
	for i, path := range validatedPaths {
		transformed[i] = transformPathForIstio(path)
	}
	return transformed
}

func transformPathForIstio(validatedPath string) string {
	// Return paths without {**} suffix unchanged
	if !strings.HasSuffix(validatedPath, matchAnyTemplate) {
		return validatedPath
	}

	trimmedPath := strings.TrimSuffix(validatedPath, matchAnyTemplate)

	// Return paths with {**} as final standalone segment unchanged
	if strings.HasSuffix(trimmedPath, "/") {
		// It's a standalone segment like "/api/{**}", don't transform
		return validatedPath
	}

	// Return paths with non-standalone {**} suffix unchanged if there are other template operators in the path.
	// Either case (changed or unchanged) will be invalid Istio syntax.
	if strings.Contains(trimmedPath, matchOneTemplate) || strings.Contains(trimmedPath, matchAnyTemplate) {
		return validatedPath
	}

	// Transform: replace {**} suffix with *
	return trimmedPath + "*"
}
