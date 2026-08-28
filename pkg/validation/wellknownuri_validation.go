package validation

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateWellKnownURI verifies that uri is a well-formed http or https URL
// suitable for use as an OpenID Connect / OAuth discovery endpoint.
//
// The check is intentionally stricter than the CRD Pattern: it uses the
// standard library's net/url parser to reject values that trivially match
// the surface-level pattern but are not usable URLs (empty host, query or
// fragment components, illegal characters, etc.).
func ValidateWellKnownURI(uri string) error {
	if uri == "" {
		return fmt.Errorf("wellKnownURI must not be empty")
	}

	parsed, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("wellKnownURI %q is not a valid URL: %w", uri, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("wellKnownURI %q must use http or https scheme", uri)
	}

	if parsed.Host == "" {
		return fmt.Errorf("wellKnownURI %q must include a host", uri)
	}

	if parsed.RawQuery != "" {
		return fmt.Errorf("wellKnownURI %q must not contain a query string", uri)
	}

	if parsed.Fragment != "" {
		return fmt.Errorf("wellKnownURI %q must not contain a fragment", uri)
	}

	return nil
}
