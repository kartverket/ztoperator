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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	Describe("SetupPodWebhookWithManager", func() {
		It("panics when manager is nil (sanity coverage)", func() {
			// This is a lightweight coverage test. Proper webhook wiring is validated via chainsaw.
			Expect(func() { _ = v1.SetupPodWebhookWithManager(ctrl.Manager(nil)) }).To(Panic())
		})
	})

	Describe("GetPodAuthPolicyuration", func() {
		It("returns CreatedFromSkiperatorApplication=false when Pod is not created from Skiperator Application", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: skiperatorAppName, Namespace: "ns"}}
			cfg, err := v1.GetPodAuthPolicyConfiguration(ctx, nil, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(v1.PodAuthPolicyConfiguration{CreatedFromSkiperatorApplication: false}))
		})

		It("returns error when Pod is created from Skiperator Application, but k8sClient is nil", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      skiperatorAppName,
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}
			cfg, err := v1.GetPodAuthPolicyConfiguration(ctx, nil, pod)
			Expect(err).To(MatchError(Equal("webhook client is not configured")))
			Expect(cfg).To(BeNil())
		})

		It("returns PodAuthPolicyuration with only AppName and CreatedFromSkiperatorApplication=true when pod is NOT annotated to verify nor to have any services", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      skiperatorAppName,
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}
			cfg, err := v1.GetPodAuthPolicyConfiguration(ctx, GetMockKubernetesClient(scheme), pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(
				v1.PodAuthPolicyConfiguration{
					AppName:                          skiperatorAppName,
					CreatedFromSkiperatorApplication: true,
				},
			))
		})

		It("returns error when no AuthPolicy resource was found for a given pod with correct annotation", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      skiperatorAppName,
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						v1.ZtoperatorVerifyAnnotationKey: v1.ZtoperatorVerifyAnnotationValue,
					},
				},
			}
			cfg, err := v1.GetPodAuthPolicyConfiguration(
				ctx,
				GetMockKubernetesClient(scheme),
				pod,
			)
			Expect(err).To(MatchError(Equal("no AuthPolicy resource was found for the corresponding Application")))
			Expect(cfg).To(BeNil())
		})

		It("returns error when multiple AuthPolicies was found all referencing the same Skiperator Application", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      skiperatorAppName,
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						v1.ZtoperatorVerifyAnnotationKey: v1.ZtoperatorVerifyAnnotationValue,
					},
				},
			}
			cfg, err := v1.GetPodAuthPolicyConfiguration(
				ctx,
				GetMockKubernetesClient(
					scheme,
					&ztoperatorv1.AuthPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "auth-policy",
							Namespace: pod.Namespace,
						},
						Spec: ztoperatorv1.AuthPolicySpec{
							Selector: ztoperatorv1.WorkloadSelector{
								MatchLabels: map[string]string{"app": skiperatorAppName},
							},
						},
					},
					&ztoperatorv1.AuthPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "another-auth-policy",
							Namespace: pod.Namespace,
						},
						Spec: ztoperatorv1.AuthPolicySpec{
							Selector: ztoperatorv1.WorkloadSelector{
								MatchLabels: map[string]string{"app": skiperatorAppName},
							},
						},
					},
				),
				pod,
			)
			Expect(err).To(MatchError(Equal("multiple AuthPolicy resources found for Application")))
			Expect(cfg).To(BeNil())
		})

		It("returns PodAuthPolicyConfiguratiuration with AuthPolicy when pod is annotated to verify and a AuthPolicy referencing the original Skiperator application exists", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      skiperatorAppName,
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						v1.ZtoperatorVerifyAnnotationKey: v1.ZtoperatorVerifyAnnotationValue,
					},
				},
			}

			AuthPolicy := ztoperatorv1.AuthPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: pod.Namespace,
				},
				Spec: ztoperatorv1.AuthPolicySpec{
					Selector: ztoperatorv1.WorkloadSelector{
						MatchLabels: map[string]string{"app": skiperatorAppName},
					},
				},
			}

			mockClient := GetMockKubernetesClient(scheme, &AuthPolicy)
			Expect(mockClient.Get(ctx, client.ObjectKeyFromObject(&AuthPolicy), &AuthPolicy)).To(Succeed())
			// Simulate the AuthPolicy becoming ready after being created.
			AuthPolicy.Status.Ready = true
			Expect(mockClient.Update(ctx, &AuthPolicy)).To(Succeed())

			cfg, err := v1.GetPodAuthPolicyConfiguration(
				ctx,
				mockClient,
				pod,
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(
				v1.PodAuthPolicyConfiguration{
					AuthPolicy:                       AuthPolicy,
					AppName:                          skiperatorAppName,
					CreatedFromSkiperatorApplication: true,
				},
			))
		})
	})
})
