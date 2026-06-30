package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ztlog "github.com/kartverket/ztoperator/pkg/log"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestGetOAuthDiscoveryDocument_ReturnsCachedDocumentForKnownURI(t *testing.T) {
	t.Parallel()

	resolver := NewDefaultDiscoveryDocumentResolver()
	uri := "https://idporten.no/.well-known/openid-configuration"

	doc, err := resolver.GetOAuthDiscoveryDocument(uri, testLogger())
	if err != nil {
		t.Fatalf("expected no error for cached URI, got: %v", err)
	}
	if doc == nil {
		t.Fatal("expected discovery document, got nil")
	}

	want := GetWellknownURIToDiscoveryDocument()[uri]
	assertStringPtrEqual(t, "issuer", doc.Issuer, want.Issuer)
	assertStringPtrEqual(t, "authorization_endpoint", doc.AuthorizationEndpoint, want.AuthorizationEndpoint)
	assertStringPtrEqual(t, "token_endpoint", doc.TokenEndpoint, want.TokenEndpoint)
	assertStringPtrEqual(t, "jwks_uri", doc.JwksURI, want.JwksURI)
	assertStringPtrEqual(t, "end_session_endpoint", doc.EndSessionEndpoint, want.EndSessionEndpoint)
}

func TestGetOAuthDiscoveryDocument_FetchesUnknownURIOverHTTP(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issuer":"https://issuer.example.com",
			"authorization_endpoint":"https://issuer.example.com/authorize",
			"token_endpoint":"https://issuer.example.com/token",
			"jwks_uri":"https://issuer.example.com/jwks",
			"end_session_endpoint":"https://issuer.example.com/logout"
		}`))
	}))
	defer server.Close()

	resolver := NewDefaultDiscoveryDocumentResolver()

	doc, err := resolver.GetOAuthDiscoveryDocument(server.URL+"/.well-known/openid-configuration", testLogger())
	if err != nil {
		t.Fatalf("expected no error when fetching discovery document, got: %v", err)
	}
	if doc == nil {
		t.Fatal("expected discovery document, got nil")
	}

	assertStringPtrValue(t, "issuer", doc.Issuer, "https://issuer.example.com")
	assertStringPtrValue(t, "authorization_endpoint", doc.AuthorizationEndpoint, "https://issuer.example.com/authorize")
	assertStringPtrValue(t, "token_endpoint", doc.TokenEndpoint, "https://issuer.example.com/token")
	assertStringPtrValue(t, "jwks_uri", doc.JwksURI, "https://issuer.example.com/jwks")
	assertStringPtrValue(t, "end_session_endpoint", doc.EndSessionEndpoint, "https://issuer.example.com/logout")
}

func TestGetOAuthDiscoveryDocument_ReturnsErrorForNon200Response(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	resolver := NewDefaultDiscoveryDocumentResolver()

	doc, err := resolver.GetOAuthDiscoveryDocument(server.URL+"/.well-known/openid-configuration", testLogger())
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
	if doc != nil {
		t.Fatalf("expected nil document on non-200 response, got: %#v", doc)
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected error to contain HTTP status code, got: %v", err)
	}
}

func testLogger() ztlog.Logger {
	return ztlog.Logger{Logger: ctrl.Log.WithName("rest-client-test")}
}

func assertStringPtrEqual(t *testing.T, field string, got, want *string) {
	t.Helper()

	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("field %q mismatch: got=%v want=%v", field, got, want)
	}
	if *got != *want {
		t.Fatalf("field %q mismatch: got=%q want=%q", field, *got, *want)
	}
}

func assertStringPtrValue(t *testing.T, field string, got *string, want string) {
	t.Helper()

	if got == nil {
		t.Fatalf("field %q was nil, wanted %q", field, want)
	}
	if *got != want {
		t.Fatalf("field %q mismatch: got=%q want=%q", field, *got, want)
	}
}

