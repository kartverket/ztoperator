package validation_test

import (
	"strings"
	"testing"

	"github.com/kartverket/ztoperator/pkg/validation"
)

func TestValidateWellKnownURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "accepts a well-formed https URL",
			uri:     "https://example.com/.well-known/openid-configuration",
			wantErr: false,
		},
		{
			name:    "accepts a well-formed http URL with port",
			uri:     "http://mock-oauth2.auth:8080/entraid/.well-known/openid-configuration",
			wantErr: false,
		},
		{
			name:    "rejects empty string",
			uri:     "",
			wantErr: true,
			errMsg:  "must not be empty",
		},
		{
			name:    "rejects unsupported scheme",
			uri:     "ftp://example.com/.well-known/openid-configuration",
			wantErr: true,
			errMsg:  "must use http or https scheme",
		},
		{
			name:    "rejects missing host",
			uri:     "https:///path",
			wantErr: true,
			errMsg:  "must include a host",
		},
		{
			name:    "rejects query string",
			uri:     "https://example.com/.well-known/openid-configuration?tenant=x",
			wantErr: true,
			errMsg:  "must not contain a query string",
		},
		{
			name:    "rejects fragment",
			uri:     "https://example.com/.well-known/openid-configuration#section",
			wantErr: true,
			errMsg:  "must not contain a fragment",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidateWellKnownURI(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errMsg != "" && !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error to contain %q, got %q", tc.errMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
