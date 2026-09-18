package v1_test

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	v1 "github.com/kartverket/ztoperator/internal/webhook/v1"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const webhookAllowedURI = "https://idp.example.com/.well-known/openid-configuration"

func TestAuthPolicyValidatorAcceptsValidAuthPolicy(t *testing.T) {
	validator := newAuthPolicyValidator()

	warnings, err := validator.ValidateCreate(context.Background(), validWebhookAuthPolicy())

	require.Empty(t, warnings)
	require.NoError(t, err)
}

func TestAuthPolicyValidatorRejectsInvalidWellKnownURI(t *testing.T) {
	validator := newAuthPolicyValidator()
	authPolicy := validWebhookAuthPolicy()
	authPolicy.Spec.WellKnownURI = "ftp://idp.example.com/.well-known/openid-configuration"

	_, err := validator.ValidateCreate(context.Background(), authPolicy)

	require.ErrorContains(t, err, "must use http or https scheme")
}

func TestAuthPolicyValidatorRejectsURIOutsideAllowlist(t *testing.T) {
	validator := newAuthPolicyValidator()
	authPolicy := validWebhookAuthPolicy()
	authPolicy.Spec.WellKnownURI = "https://other.example.com/.well-known/openid-configuration"

	_, err := validator.ValidateCreate(context.Background(), authPolicy)

	require.ErrorContains(t, err, "is not in the configured allowlist")
}

func TestAuthPolicyValidatorRejectsInvalidPaths(t *testing.T) {
	validator := newAuthPolicyValidator()
	authPolicy := validWebhookAuthPolicy()
	authPolicy.Spec.AuthRules = &[]ztoperatorv1alpha1.RequestAuthRule{{
		RequestMatcher: ztoperatorv1alpha1.RequestMatcher{Paths: []string{"not-a-path"}},
	}}

	_, err := validator.ValidateCreate(context.Background(), authPolicy)

	require.ErrorContains(t, err, "must start with '/'")
}

func TestAuthPolicyValidatorRejectsInvalidCreateAndUpdate(t *testing.T) {
	validator := newAuthPolicyValidator()
	invalid := validWebhookAuthPolicy()
	invalid.Spec.WellKnownURI = "https://not-allowed.example.com/.well-known/openid-configuration"

	_, createErr := validator.ValidateCreate(context.Background(), invalid)
	_, updateErr := validator.ValidateUpdate(context.Background(), validWebhookAuthPolicy(), invalid)

	require.Error(t, createErr)
	require.Error(t, updateErr)
}

func TestAuthPolicyValidatorAcceptsValidUpdate(t *testing.T) {
	validator := newAuthPolicyValidator()
	oldAuthPolicy := validWebhookAuthPolicy()
	newAuthPolicy := oldAuthPolicy.DeepCopy()
	newAuthPolicy.Spec.AuthRules = &[]ztoperatorv1alpha1.RequestAuthRule{{
		RequestMatcher: ztoperatorv1alpha1.RequestMatcher{Paths: []string{"/admin"}},
	}}

	warnings, err := validator.ValidateUpdate(context.Background(), oldAuthPolicy, newAuthPolicy)

	require.Empty(t, warnings)
	require.NoError(t, err)
}

func newAuthPolicyValidator() *v1.AuthPolicyCustomValidator {
	return &v1.AuthPolicyCustomValidator{
		DiscoveryDocumentCache: config.NewDiscoveryDocumentCache(
			[]string{webhookAllowedURI},
			nil,
		),
	}
}

func validWebhookAuthPolicy() *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "auth-policy", Namespace: "default"},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			Enabled:      true,
			WellKnownURI: webhookAllowedURI,
			Selector: ztoperatorv1alpha1.WorkloadSelector{
				MatchLabels: map[string]string{"app": "application"},
			},
		},
	}
}
