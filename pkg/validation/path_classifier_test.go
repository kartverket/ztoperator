package validation

import (
	"strings"
	"testing"
)

func TestClassifyPath_Valid(t *testing.T) {
	assertClassifyKind(t, "/", pathKindPlain)
	assertClassifyKind(t, "/api", pathKindPlain)
	assertClassifyKind(t, "/api/v1", pathKindPlain)

	assertClassifyKind(t, "/api/{*}", pathKindTemplate)
	assertClassifyKind(t, "/api/{**}", pathKindTemplate)
	assertClassifyKind(t, "/api/{*}/{**}", pathKindTemplate)
	assertClassifyKind(t, "/api/{**}/{*}", pathKindTemplate)
	assertClassifyKind(t, "/{*}", pathKindTemplate)
	assertClassifyKind(t, "/{*}/{**}", pathKindTemplate)

	assertClassifyKind(t, "/*", pathKindLegacyStar)
	assertClassifyKind(t, "/api/*", pathKindLegacyStar)
}

func TestClassifyPath_Errors(t *testing.T) {
	assertClassifyError(t, "/api/{*}/*", "Contains '{' or '}' beyond a supported path template")
	assertClassifyError(t, "/api/{*}/test*", "Contains '{' or '}' beyond a supported path template")
	assertClassifyError(t, "/api/{**}/*", "Contains '{' or '}' beyond a supported path template")
	assertClassifyError(t, "/api/{**}/test*", "Contains '{' or '}' beyond a supported path template")

	assertClassifyError(t, "/api/*/test", "be at the end")
	assertClassifyError(t, "/api/v1*/test", "be at the end")

	assertClassifyError(t, "/api/*/test*", "appear only once")
	assertClassifyError(t, "/api/*/test/*", "appear only once")
	assertClassifyError(t, "/api**", "appear only once")
}

func assertClassifyKind(t *testing.T, path string, want pathKind) {
	t.Helper()
	got, err := classifyPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got kind %v, want %v", got, want)
	}
}

func assertClassifyError(t *testing.T, path string, wantContains string) {
	t.Helper()
	_, err := classifyPath(path)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if wantContains != "" && !strings.Contains(err.Error(), wantContains) {
		t.Fatalf("error %q does not contain %q", err.Error(), wantContains)
	}
}
