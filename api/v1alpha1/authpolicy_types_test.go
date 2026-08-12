package v1alpha1_test

import (
	"context"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("AuthPolicy CRD", func() {
	Context("When applying an AuthPolicy resource", func() {
		const (
			authPolicyName = "auth-policy"
			namespaceName  = "default"
		)

		testCtx := context.Background()

		makeAuthPolicy := func() *ztoperatorv1alpha1.AuthPolicy {
			return getValidAuthPolicy(namespaceName, authPolicyName)
		}

		AfterEach(func() {
			authPolicyList := &ztoperatorv1alpha1.AuthPolicyList{}
			if err := k8sClient.List(testCtx, authPolicyList, client.InNamespace(namespaceName)); err == nil {
				for _, authPolicy := range authPolicyList.Items {
					_ = k8sClient.Delete(testCtx, &authPolicy)
				}
			}
		})

		It("should reject updates when audience has both value and valueFrom", func() {
			authPolicy := makeAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.AllowedAudiences = []ztoperatorv1alpha1.AllowedAudience{
				{
					Value: stringPtr("inline-value"),
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
			authPolicy := makeAuthPolicy()
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
			authPolicy := makeAuthPolicy()
			Expect(k8sClient.Create(testCtx, authPolicy)).To(Succeed())

			authPolicy.Spec.AllowedAudiences = []ztoperatorv1alpha1.AllowedAudience{
				{Value: stringPtr("")},
			}

			err := k8sClient.Update(testCtx, authPolicy)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("field 'value' cannot be empty string"))
		})
	})
})
