package validation_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/validation"
)

type pathValidationTestCase struct {
	name    string
	paths   []string
	wantErr bool
	errMsg  string
}

func TestValidatePaths_MustStartWithSlash(t *testing.T) {
	errMsg := "must start with '/'"

	invalidPaths := []string{
		"api/v1",
		"secure",
		"public",
		"*",
		"{*}",
		"{**}",
	}

	tests := make([]pathValidationTestCase, 0, len(invalidPaths))
	for _, path := range invalidPaths {
		tests = append(tests, pathValidationTestCase{
			name:    "invalid path without leading slash: " + path,
			paths:   []string{path},
			wantErr: true,
			errMsg:  errMsg,
		})
	}

	performTests(t, tests)
}

func TestValidatePaths_SingleAsteriskWildcardOnlyValidAtEndOfPath(t *testing.T) {
	errMsg := "'*' must appear"

	invalidPaths := []string{
		"/*/api",
		"/*api",
		"/api/*/test",
		"/api/*test",
		"/api/te*st",
		"/api/*/users/*",
		"/**",
		"/api/**",
		"/api/**/users",
	}

	tests := make([]pathValidationTestCase, 0, len(invalidPaths))
	for _, path := range invalidPaths {
		tests = append(tests, pathValidationTestCase{
			name:    "invalid asterisk wildcard: " + path,
			paths:   []string{path},
			wantErr: true,
			errMsg:  errMsg,
		})
	}

	performTests(t, tests)
}

func TestValidatePaths_CurlyBracketsNotPartOfWildcardNotAllowed(t *testing.T) {
	errMsg := "Contains '{' or '}' beyond a supported path template"

	invalidPaths := []string{
		"/{}",
		"/api/{v1}/users",
		"/api/v{1}/users",
		"/api/v1}/users",
		"/api/{v1/users",
		"/{*",
		"/*}",
		"/{*api}",
		"/{*/api}",
		"/{***}",
		"/{****}",
	}

	tests := make([]pathValidationTestCase, 0, len(invalidPaths))
	for _, path := range invalidPaths {
		tests = append(tests, pathValidationTestCase{
			name:    "invalid curly brackets: " + path,
			paths:   []string{path},
			wantErr: true,
			errMsg:  errMsg,
		})
	}

	performTests(t, tests)
}

func TestValidatePaths_MultiPathSegmentNotLastOperatorNotAllowed(t *testing.T) {
	invalidPathsNotLast := []string{
		"/api/{**}/{*}",
		"/{**}/test/{*}",
		"/{**}/{**}",
	}

	invalidPathsBrackets := []string{
		"/{**}/{**}test",
		"/{**}{*}",
		"/api/{**}{*}",
		"/{**}test",
		"/api/{**}test",
	}

	tests := make([]pathValidationTestCase, 0, len(invalidPathsNotLast)+len(invalidPathsBrackets))

	for _, path := range invalidPathsNotLast {
		tests = append(tests, pathValidationTestCase{
			name:    "invalid multi-path segment not last: " + path,
			paths:   []string{path},
			wantErr: true,
			errMsg:  "{**} is not the last operator",
		})
	}

	for _, path := range invalidPathsBrackets {
		tests = append(tests, pathValidationTestCase{
			name:    "invalid multi-path segment with suffix: " + path,
			paths:   []string{path},
			wantErr: true,
			errMsg:  "Contains '{' or '}' beyond a supported path template",
		})
	}

	performTests(t, tests)
}

func TestValidatePaths_IllegalStringLiteralsNotAllowed(t *testing.T) {
	errMsg := "invalid string literal"

	invalidPaths := []string{
		// Legacy star syntax
		"/api space/*",
		"/api#fragment/*",
		"/api?query/*",
		"/api/<script>/*",
		"/api/array[0]/*",
		"/api/path\\segment/*",

		"/api space{**}",
		"/api#fragment{**}",
		"/api?query{**}",
		"/api<script{**}",
		"/api[0]{**}",
		"/api\\segment{**}",
		"/api|pipe{**}",
		"/api^symbol{**}",
		"/api`quote{**}",
		"/api\"quote{**}",

		"/api/{*}/user profile",
		"/api/{*}/test#fragment",
		"/api/{*}/query?param",
		"/api/{*}/<script>",
		"/api/{*}/array[0]",
		"/api/{*}/path\\segment",
		"/api/{*}/test|pipe",
		"/api/{*}/test^symbol",
		"/api/{*}/test`quote",
		"/api/{*}/test\"quote",
		"/api/{*}/test\nline",
		"/api/{*}/test\ttab",

		// Plain paths should also be validated
		"/api/user profile",
		"/api/test#fragment",
		"/api/query?param",
		"/api/<script>",
		"/api/array[0]",
		"/api/path\\segment",
		"/api/test|pipe",
		"/api/test^symbol",
		"/api/test`quote",
		"/api/test\"quote",
		"/api/test\nline",
		"/api/test\ttab",
	}

	tests := make([]pathValidationTestCase, 0, len(invalidPaths))
	for _, path := range invalidPaths {
		tests = append(tests, pathValidationTestCase{
			name:    "invalid string literal: " + path,
			paths:   []string{path},
			wantErr: true,
			errMsg:  errMsg,
		})
	}

	performTests(t, tests)
}

func TestValidatePaths_MultiPathSegmentWithPrefixAsLastOperatorWithOtherOperatorsNotAllowed(t *testing.T) {
	invalidPaths := []string{
		"/api/{*}/test{**}",
		"/api/{**}/test{**}",
	}

	tests := make([]pathValidationTestCase, 0, len(invalidPaths))
	for _, path := range invalidPaths {
		tests = append(tests, pathValidationTestCase{
			name:    "invalid multi-path segment with prefix and other operators: " + path,
			paths:   []string{path},
			wantErr: true,
			errMsg:  "{**} is not the last operator",
		})
	}

	performTests(t, tests)
}

func TestValidatePaths_MultiPathSegmentWithPrefixAsNonLastOperatorNotAllowed(t *testing.T) {
	invalidPaths := []string{
		"/api{**}/test",
		"/api/v{**}/test",
	}

	tests := make([]pathValidationTestCase, 0, len(invalidPaths))
	for _, path := range invalidPaths {
		tests = append(tests, pathValidationTestCase{
			name:    "invalid multi-path segment with prefix and other operators: " + path,
			paths:   []string{path},
			wantErr: true,
			errMsg:  "{**} is not the last operator",
		})
	}

	performTests(t, tests)
}

func TestValidatePaths_MultiPathSegmentWithPrefixAllowedAsLastOperatorWithoutOtherOperators(t *testing.T) {
	validPaths := []string{
		"/api{**}",
		"/api/test{**}",
	}

	tests := make([]pathValidationTestCase, 0, len(validPaths))
	for _, path := range validPaths {
		tests = append(tests, pathValidationTestCase{
			name:    "valid path: " + path,
			paths:   []string{path},
			wantErr: false,
		})
	}

	performTests(t, tests)
}

func TestValidatePaths_ValidPaths(t *testing.T) {
	validPaths := []string{
		// Simple paths
		"/",
		"/api",
		"/api/v1",
		"/users/profile",

		// Standalone wildcard at end of path
		"/*",
		"/api/*",
		"/api/v1/users/*",

		// Single-segment path template
		"/api/{*}",

		// Multiple single-segment path templates
		"/api/{*}/users/{*}",

		// Single-segment path template with prefix and suffix
		"/api/{*}/users",
		"/api/{*}/users/{*}/profile",

		// Root multi-segment wildcard
		"/{**}",

		// Multi-segment wildcard at end
		"/api/{**}",
		"/api/v1/{**}",

		// Multi-segment wildcard with suffix
		"/api/{**}/users",

		// Single and multi-segment templates
		"/api/{*}/data/{**}",

		// Multiple single templates before multi-segment
		"/api/{*}/users/{*}/data/{**}",

		// Valid special characters (RFC 3986 pchar)
		"/api/{*}/user-profile", // -
		"/api/{*}/user_profile", // _
		"/api/{*}/file.json",    // .
		"/api/{*}/path~test",    // ~
		"/api/{*}/test%20",      // %
		"/api/{*}/test!value",   // !
		"/api/{*}/test$value",   // $
		"/api/{*}/test&value",   // &
		"/api/{*}/user's-data",  // '
		"/api/{*}/test(1)",      // ( )
		"/api/{*}/test+value",   // +
		"/api/{*}/test,value",   // ,
		"/api/{*}/test;value",   // ;
		"/api/{*}/test:value",   // :
		"/api/{*}/user@domain",  // @
		"/api/{*}/key=value",    // =

		"/api/user-profile", // -
		"/api/user_profile", // _
		"/api/file.json",    // .
		"/api/path~test",    // ~
		"/api/test%20",      // %
		"/api/test!value",   // !
		"/api/test$value",   // $
		"/api/test&value",   // &
		"/api/user's-data",  // '
		"/api/test(1)",      // ( )
		"/api/test+value",   // +
		"/api/test,value",   // ,
		"/api/test;value",   // ;
		"/api/test:value",   // :
		"/api/user@domain",  // @
		"/api/key=value",    // =
	}

	tests := make([]pathValidationTestCase, 0, len(validPaths))
	for _, path := range validPaths {
		tests = append(tests, pathValidationTestCase{
			name:    "valid path: " + path,
			paths:   []string{path},
			wantErr: false,
		})
	}

	performTests(t, tests)
}

func TestValidatePaths_MultipleValidPathsWithOneInvalidIsInvalid(t *testing.T) {
	tests := []pathValidationTestCase{
		{
			name:    "multiple valid paths with one missing leading slash",
			paths:   []string{"invalid", "/valid", "/valid/{*}", "/valid/{**}"},
			wantErr: true,
			errMsg:  "must start with '/'",
		},
	}

	performTests(t, tests)
}

func performTests(t *testing.T, tests []pathValidationTestCase) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidatePaths(tt.paths)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePaths() expected error but got none")
					return
				}
				if tt.errMsg != "" && !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("ValidatePaths() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("ValidatePaths() unexpected error = %v", err)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
