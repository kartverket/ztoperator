package validation_test

import (
	"testing"

	"github.com/kartverket/ztoperator/pkg/validation"
)

func TestTransformPathsForIstio(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "transform single path with {**} suffix",
			input:    []string{"/api{**}"},
			expected: []string{"/api*"},
		},
		{
			name:     "transform multiple segment path with {**} suffix",
			input:    []string{"/api/test{**}"},
			expected: []string{"/api/test*"},
		},
		{
			name:     "do not transform standalone {**} segment",
			input:    []string{"/api/{**}"},
			expected: []string{"/api/{**}"},
		},
		{
			name:     "do not transform root {**} segment",
			input:    []string{"/{**}"},
			expected: []string{"/{**}"},
		},
		{
			name:     "do not transform path with {*} before {**} suffix",
			input:    []string{"/api/{*}/test{**}"},
			expected: []string{"/api/{*}/test{**}"},
		},
		{
			name:     "do not transform path with standalone {**} before suffix",
			input:    []string{"/api/{**}/test{**}"},
			expected: []string{"/api/{**}/test{**}"},
		},
		{
			name:     "do not transform paths without {**}",
			input:    []string{"/api", "/test/*", "/data/{*}"},
			expected: []string{"/api", "/test/*", "/data/{*}"},
		},
		{
			name:     "transform multiple paths with mixed patterns",
			input:    []string{"/api{**}", "/test/{**}", "/data/*", "/items{**}"},
			expected: []string{"/api*", "/test/{**}", "/data/*", "/items*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validation.TransformPathsForIstio(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("TransformPathsForIstio() returned %d paths, expected %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("TransformPathsForIstio()[%d] = %q, expected %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}
