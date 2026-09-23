package authpolicy_test

import (
	"context"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/webhook/authpolicy"
	"github.com/kartverket/ztoperator/pkg/rest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const webhookAllowedURI = "https://idp.example.com/.well-known/openid-configuration"

var _ = Describe("AuthPolicy validator", func() {
	var validator *authpolicy.AuthPolicyCustomValidator

	BeforeEach(func() {
		validator = newAuthPolicyValidator()
	})

	It("accepts a valid AuthPolicy", func() {
		warnings, err := validator.ValidateCreate(context.Background(), validWebhookAuthPolicy())

		Expect(warnings).To(BeEmpty())
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects a URI outside the configured allowlist", func() {
		authPolicy := validWebhookAuthPolicy()
		authPolicy.Spec.WellKnownURI = "https://other.example.com/.well-known/openid-configuration"

		_, err := validator.ValidateCreate(context.Background(), authPolicy)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("is not in the configured allowlist"))
	})

	It("rejects invalid paths", func() {
		authPolicy := validWebhookAuthPolicy()
		authPolicy.Spec.AuthRules = &[]ztoperatorv1alpha1.RequestAuthRule{{
			RequestMatcher: ztoperatorv1alpha1.RequestMatcher{Paths: []string{"not-a-path"}},
		}}

		_, err := validator.ValidateCreate(context.Background(), authPolicy)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("must start with '/'"))
	})

	It("rejects invalid AuthPolicy creates and updates", func() {
		invalid := validWebhookAuthPolicy()
		invalid.Spec.WellKnownURI = "https://not-allowed.example.com/.well-known/openid-configuration"

		_, createErr := validator.ValidateCreate(context.Background(), invalid)
		_, updateErr := validator.ValidateUpdate(context.Background(), validWebhookAuthPolicy(), invalid)

		Expect(createErr).To(HaveOccurred())
		Expect(updateErr).To(HaveOccurred())
	})

	It("accepts a valid AuthPolicy update", func() {
		oldAuthPolicy := validWebhookAuthPolicy()
		newAuthPolicy := oldAuthPolicy.DeepCopy()
		newAuthPolicy.Spec.AuthRules = &[]ztoperatorv1alpha1.RequestAuthRule{{
			RequestMatcher: ztoperatorv1alpha1.RequestMatcher{Paths: []string{"/admin"}},
		}}

		warnings, err := validator.ValidateUpdate(context.Background(), oldAuthPolicy, newAuthPolicy)

		Expect(warnings).To(BeEmpty())
		Expect(err).NotTo(HaveOccurred())
	})
})

func newAuthPolicyValidator() *authpolicy.AuthPolicyCustomValidator {
	return &authpolicy.AuthPolicyCustomValidator{
		DiscoveryDocumentCache: map[string]rest.DiscoveryDocument{
			webhookAllowedURI: {},
		},
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
