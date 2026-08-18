package v1alpha1_test

import (
	"context"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getValidAuthPolicy() *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-policy",
			Namespace: "default",
		},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			Enabled:      true,
			WellKnownURI: "http://mock-oauth2.auth:8080/entraid/.well-known/openid-configuration",
			Selector: ztoperatorv1alpha1.WorkloadSelector{
				MatchLabels: map[string]string{"app": "application"},
			},
		},
	}
}

var _ = Describe("AuthPolicy CRD", func() {
	Context("When applying an AuthPolicy resource", func() {
		const namespaceName = "default"

		testCtx := context.Background()

		AfterEach(func() {
			authPolicyList := &ztoperatorv1alpha1.AuthPolicyList{}
			if err := k8sClient.List(testCtx, authPolicyList, client.InNamespace(namespaceName)); err == nil {
				for _, authPolicy := range authPolicyList.Items {
					_ = k8sClient.Delete(testCtx, &authPolicy)
				}
			}
		})

		It("should reject updates when audience has both value and valueFrom", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			inlineValue := "inline-value"
			authPolicy.Spec.AllowedAudiences = []ztoperatorv1alpha1.AllowedAudience{
				{
					Value: &inlineValue,
					ValueFrom: &ztoperatorv1alpha1.ValueFrom{
						ConfigMapKeyRef: &ztoperatorv1alpha1.KeyRef{Name: "cm", Key: "KEY"},
					},
				},
			}

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("one audience cannot be defined from both 'value' and 'valueFrom'"))
		})

		It("should reject updates when audience valueFrom references both ConfigMap and Secret", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.AllowedAudiences = []ztoperatorv1alpha1.AllowedAudience{
				{
					ValueFrom: &ztoperatorv1alpha1.ValueFrom{
						ConfigMapKeyRef: &ztoperatorv1alpha1.KeyRef{Name: "cm", Key: "KEY"},
						SecretKeyRef:    &ztoperatorv1alpha1.KeyRef{Name: "secret", Key: "KEY"},
					},
				},
			}

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("cannot reference both a ConfigMap and a Secret"))
		})

		It("should reject updates when audience value is an empty string", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.AllowedAudiences = []ztoperatorv1alpha1.AllowedAudience{
				{Value: new(string)},
			}

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("field 'value' cannot be empty string"))
		})

		It("should reject updates when acceptedResources is missing for idporten test wellKnownURI", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.WellKnownURI = "https://test.idporten.no/.well-known/openid-configuration"

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("acceptedResources must be non-empty when using Ansattporten or ID-Porten"))
		})

		It("should reject updates when acceptedResources is missing for idporten prod wellKnownURI", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.WellKnownURI = "https://idporten.no/.well-known/openid-configuration"

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("acceptedResources must be non-empty when using Ansattporten or ID-Porten"))
		})

		It("should reject updates when acceptedResources is missing for ansattporten test wellKnownURI", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.WellKnownURI = "https://test.ansattporten.no/.well-known/openid-configuration"

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("acceptedResources must be non-empty when using Ansattporten or ID-Porten"))
		})

		It("should reject updates when acceptedResources is missing for ansattporten prod wellKnownURI", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.WellKnownURI = "https://ansattporten.no/.well-known/openid-configuration"

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("acceptedResources must be non-empty when using Ansattporten or ID-Porten"))
		})

		It("should reject updates when baselineAuth claims is an empty list", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.BaselineAuth = &ztoperatorv1alpha1.BaselineAuth{
				Claims: []ztoperatorv1alpha1.Condition{},
			}

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("claims must be a non-empty list"))
		})

		It("should reject updates when autoLogin is enabled without oAuthCredentials", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.AutoLogin = &ztoperatorv1alpha1.AutoLogin{
				Enabled: true,
				Scopes:  []string{"scope1", "scope2"},
			}

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("oAuthCredentials must be set when autoLogin is enabled"))
		})

		It("should reject updates when oAuthCredentials is set without autoLogin", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.OAuthCredentials = &ztoperatorv1alpha1.OAuthCredentials{
				SecretRef:       "oauth-secret",
				ClientIDKey:     "client-id",
				ClientSecretKey: "client-secret",
			}

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("oAuthCredentials cannot be set unless autoLogin is configured"))
		})

		It("should reject updates when autoLogin loginParams contains an invalid key", func() {
			authPolicy := getValidAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.AutoLogin = &ztoperatorv1alpha1.AutoLogin{
				Enabled: true,
				Scopes:  []string{"openid"},
				LoginParams: map[string]string{
					"invalid-key!": "value",
				},
			}
			authPolicy.Spec.OAuthCredentials = &ztoperatorv1alpha1.OAuthCredentials{
				SecretRef:       "oauth-secret",
				ClientIDKey:     "client-id",
				ClientSecretKey: "client-secret",
			}

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("loginParams keys must match ^[a-zA-Z_][a-zA-Z0-9_]*$"))
		})
	})
})
