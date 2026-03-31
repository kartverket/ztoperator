package v1_test

import (
	"context"

	skiperatorv1 "github.com/kartverket/skiperator/api/v1alpha1"
	ztoperatorv1 "github.com/kartverket/ztoperator/api/v1alpha1"
	v1 "github.com/kartverket/ztoperator/internal/webhook/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("pod_webhook.go unit tests", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(ztoperatorv1.AddToScheme(scheme)).To(Succeed())
		Expect(skiperatorv1.AddToScheme(scheme)).To(Succeed())
	})

	Describe("GetAuthPolicyForApplication", func() {
		It("errors when no AuthPolicy exists for the given application", func() {
			cfg, err := v1.GetAuthPolicyForApplication(
				ctx,
				k8sClient,
				client.ObjectKey{Namespace: "ns", Name: "nonexistent-app"},
			)
			Expect(err).To(MatchError(Equal("no AuthPolicy resource was found for the corresponding Application")))
			Expect(cfg).To(BeNil())
		})

		It("errors when multiple AuthPolicies exist for the given application", func() {
			cfg, err := v1.GetAuthPolicyForApplication(
				ctx,
				GetMockKubernetesClient(
					scheme,
					&ztoperatorv1.AuthPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "auth-policy-1",
							Namespace: "ns",
							Labels:    map[string]string{"app": "myapp"},
						},
						Spec: ztoperatorv1.AuthPolicySpec{
							Selector: ztoperatorv1.WorkloadSelector{
								MatchLabels: map[string]string{"app": "myapp"},
							},
						},
					},
					&ztoperatorv1.AuthPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "auth-policy",
							Namespace: "ns",
						},
						Spec: ztoperatorv1.AuthPolicySpec{
							Selector: ztoperatorv1.WorkloadSelector{
								MatchLabels: map[string]string{"app": "myapp"},
							},
						},
					},
				),
				client.ObjectKey{Namespace: "ns", Name: "myapp"},
			)
			Expect(err).To(MatchError(Equal("multiple AuthPolicy resources found for Application")))
			Expect(cfg).To(BeNil())
		})

		It("error when AuthPolicy is not ready", func() {
			cfg, err := v1.GetAuthPolicyForApplication(
				ctx,
				GetMockKubernetesClient(
					scheme,
					&ztoperatorv1.AuthPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "auth-policy",
							Namespace: "ns",
						},
						Spec: ztoperatorv1.AuthPolicySpec{
							Selector: ztoperatorv1.WorkloadSelector{
								MatchLabels: map[string]string{"app": "myapp"},
							},
						},
					},
				),
				client.ObjectKey{Namespace: "ns", Name: "myapp"},
			)
			Expect(err).To(MatchError(Equal("AuthPolicy resource for Application is not ready")))
			Expect(cfg).To(BeNil())
		})

		It("returns the AuthPolicy when exactly one exists for the given application and it is ready", func() {
			expectedAuthPolicy := &ztoperatorv1.AuthPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "auth-policy",
					Namespace: "ns",
				},
				Spec: ztoperatorv1.AuthPolicySpec{
					Selector: ztoperatorv1.WorkloadSelector{
						MatchLabels: map[string]string{"app": "myapp"},
					},
				},
			}
			mockClient := GetMockKubernetesClient(scheme, expectedAuthPolicy)
			Expect(mockClient.Get(ctx, client.ObjectKeyFromObject(expectedAuthPolicy), expectedAuthPolicy)).To(Succeed())
			// Simulate the AuthPolicy becoming ready after being created.
			expectedAuthPolicy.Status.Ready = true
			Expect(mockClient.Update(ctx, expectedAuthPolicy)).To(Succeed())

			cfg, err := v1.GetAuthPolicyForApplication(
				ctx,
				mockClient,
				client.ObjectKey{Namespace: "ns", Name: "myapp"},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).To(Equal(expectedAuthPolicy))
			Expect(cfg.Status.Ready).To(BeTrue())
		})
	})
})

func GetMockKubernetesClient(scheme *runtime.Scheme, objects ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}
