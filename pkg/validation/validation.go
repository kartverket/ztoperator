package validation

import (
	"context"
	"fmt"
	"regexp"

	"github.com/kartverket/ztoperator/internal/state"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	matchOneTemplate = "{*}"
	matchAnyTemplate = "{**}"

	pathsValidation authPolicyValidatorType = iota
	podAnnotationsValidation
)

var (
	// Valid pchar from https://datatracker.ietf.org/doc/html/rfc3986#appendix-A
	// pchar = unreserved / pct-encoded / sub-delims / ":" / "@".
	validLiteral = regexp.MustCompile("^[a-zA-Z0-9-._~%!$&'()+,;:@=]+$")
)

type authPolicyValidatorType int

type AuthPolicyValidator struct {
	Type     authPolicyValidatorType
	Validate func(ctx context.Context, k8sClient client.Client, scope *state.Scope) error
}

func GetValidators() []AuthPolicyValidator {
	return []AuthPolicyValidator{
		{
			Type: pathsValidation,
			Validate: func(_ context.Context, _ client.Client, scope *state.Scope) error {
				return ValidatePaths(scope.AuthPolicy.GetPaths())
			},
		},
		{
			Type:     podAnnotationsValidation,
			Validate: validatePodAnnotations,
		},
	}
}

func (t authPolicyValidatorType) String() string {
	switch t {
	case pathsValidation:
		return "Path validation"
	case podAnnotationsValidation:
		return "Pod annotation"
	default:
		panic(fmt.Sprintf("unknown authPolicyValidatorType %d", t))
	}
}
