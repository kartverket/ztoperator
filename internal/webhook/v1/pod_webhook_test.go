package v1_test

import (
	"context"
	"errors"

	ztoperatorv1 "github.com/kartverket/ztoperator/api/v1alpha1"
	v1 "github.com/kartverket/ztoperator/internal/webhook/v1"
	"github.com/kartverket/ztoperator/pkg/constants"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type getErrorClient struct {
	client.Client
	err error
}

func (c getErrorClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return c.err
}

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
	})

	Describe("SetupPodWebhookWithManager", func() {
		It("panics when manager is nil (sanity coverage)", func() {
			// This is a lightweight coverage test. Proper webhook wiring is validated via chainsaw.
			Expect(func() { _ = v1.SetupPodWebhookWithManager(ctrl.Manager(nil)) }).To(Panic())
		})
	})

	Describe("GetPodAuthPolicyConfiguration", func() {
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
						constants.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}
			cfg, err := v1.GetPodAuthPolicyConfiguration(ctx, nil, pod)
			Expect(err).To(MatchError(Equal("webhook client is not configured")))
			Expect(cfg).To(BeNil())
		})

		It("returns PodAuthPolicyConfiguration with only AppName and CreatedFromSkiperatorApplication=true when pod is NOT annotated to verify nor to have any services", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      skiperatorAppName,
					Namespace: "ns",
					Labels: map[string]string{
						constants.SkiperatorApplicationRefLabel: skiperatorAppName,
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
						constants.SkiperatorApplicationRefLabel: skiperatorAppName,
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
						constants.SkiperatorApplicationRefLabel: skiperatorAppName,
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
						constants.SkiperatorApplicationRefLabel: skiperatorAppName,
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

	Describe("IsWebhookEligible", func() {
		newPod := func() corev1.Pod {
			return corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						constants.SkiperatorApplicationRefLabel: "app",
					},
					Annotations: map[string]string{
						v1.ZtoperatorVerifyAnnotationKey: v1.ZtoperatorVerifyAnnotationValue,
					},
				},
			}
		}

		newNamespace := func(labels map[string]string) *corev1.Namespace {
			return &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "ns",
					Labels: labels,
				},
			}
		}

		It("returns false when pod has no labels", func() {
			pod := newPod()
			pod.Labels = nil

			eligible, msg := v1.IsWebhookEligible(ctx, helperfunctions.GetMockKubernetesClient(scheme), pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("pod ns/p has no labels"))
		})

		It("returns false when pod is not created from a Skiperator Application", func() {
			pod := newPod()
			delete(pod.Labels, constants.SkiperatorApplicationRefLabel)

			eligible, msg := v1.IsWebhookEligible(ctx, helperfunctions.GetMockKubernetesClient(scheme), pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("pod ns/p is not created from a Skiperator Application"))
		})

		It("returns false when pod has no annotations", func() {
			pod := newPod()
			pod.Annotations = nil

			eligible, msg := v1.IsWebhookEligible(ctx, helperfunctions.GetMockKubernetesClient(scheme), pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("pod ns/p has no annotations"))
		})

		It("returns false when pod has no Ztoperator webhook annotations", func() {
			pod := newPod()
			pod.Annotations = map[string]string{"some.other/annotation": "value"}

			eligible, msg := v1.IsWebhookEligible(ctx, helperfunctions.GetMockKubernetesClient(scheme), pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("pod ns/p has no Ztoperator webhook annotations"))
		})

		It("returns false when namespace is not found", func() {
			pod := newPod()

			eligible, msg := v1.IsWebhookEligible(ctx, helperfunctions.GetMockKubernetesClient(scheme), pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("namespace ns not found"))
		})

		It("returns false when namespace lookup fails", func() {
			pod := newPod()
			mockClient := getErrorClient{
				Client: helperfunctions.GetMockKubernetesClient(scheme),
				err:    errors.New("boom"),
			}

			eligible, msg := v1.IsWebhookEligible(ctx, mockClient, pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("failed to get namespace ns: boom"))
		})

		It("returns false when namespace has no labels", func() {
			pod := newPod()
			mockClient := helperfunctions.GetMockKubernetesClient(scheme, newNamespace(nil))

			eligible, msg := v1.IsWebhookEligible(ctx, mockClient, pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("namespace ns has no labels"))
		})

		It("returns false when namespace does not have the created by SKIP label", func() {
			pod := newPod()
			mockClient := helperfunctions.GetMockKubernetesClient(scheme, newNamespace(map[string]string{"other": "label"}))

			eligible, msg := v1.IsWebhookEligible(ctx, mockClient, pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("namespace ns does not have the created by SKIP label"))
		})

		It("returns false when created by SKIP label has wrong value", func() {
			pod := newPod()
			mockClient := helperfunctions.GetMockKubernetesClient(scheme, newNamespace(map[string]string{v1.CreatedBySkipNamespaceLabel: "false"}))

			eligible, msg := v1.IsWebhookEligible(ctx, mockClient, pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("namespace ns does have the created by SKIP label, but it's value is not true"))
		})

		It("returns true when pod and namespace satisfy all webhook eligibility requirements", func() {
			pod := newPod()
			mockClient := helperfunctions.GetMockKubernetesClient(
				scheme,
				newNamespace(map[string]string{v1.CreatedBySkipNamespaceLabel: v1.CreatedBySkipNamespaceLabelValue}),
			)

			eligible, msg := v1.IsWebhookEligible(ctx, mockClient, pod)

			Expect(eligible).To(BeTrue())
			Expect(msg).To(BeEmpty())
		})
	})

	Describe("MountEnvoySecret", func() {
		const secretName = "auth-policy-envoy-secret"

		newAuthPolicy := func(enabled, autoLoginEnabled bool) ztoperatorv1.AuthPolicy {
			return ztoperatorv1.AuthPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "auth-policy", Namespace: "default"},
				Spec: ztoperatorv1.AuthPolicySpec{
					Enabled:   enabled,
					AutoLogin: &ztoperatorv1.AutoLogin{Enabled: autoLoginEnabled},
				},
			}
		}
		newPod := func(containers, initContainers []corev1.Container) *corev1.Pod {
			return &corev1.Pod{Spec: corev1.PodSpec{Containers: containers, InitContainers: initContainers}}
		}
		app := corev1.Container{Name: "app"}
		proxy := corev1.Container{Name: "istio-proxy"}

		DescribeTable("does not mutate the pod",
			func(pod *corev1.Pod, enabled, autoLoginEnabled bool) {
				before := pod.DeepCopy()
				v1.MountEnvoySecret(pod, newAuthPolicy(enabled, autoLoginEnabled))
				Expect(pod).To(Equal(before))
			},
			Entry("when the AuthPolicy is disabled", newPod([]corev1.Container{app, proxy}, nil), false, true),
			Entry("when auto-login is disabled", newPod([]corev1.Container{app, proxy}, nil), true, false),
			Entry("when istio has not injected istio-proxy yet", newPod([]corev1.Container{app}, nil), true, true),
			Entry("when the pod opts out of injection", func() *corev1.Pod {
				pod := newPod([]corev1.Container{app, proxy}, nil)
				pod.Labels = map[string]string{"sidecar.istio.io/inject": "false"}
				return pod
			}(), true, true),
			Entry("when the Istio mount annotations are set", func() *corev1.Pod {
				pod := newPod([]corev1.Container{app, proxy}, nil)
				pod.Annotations = map[string]string{"sidecar.istio.io/userVolumeMount": "[]"}
				return pod
			}(), true, true),
		)

		DescribeTable("mounts the secret read-only in istio-proxy",
			func(pod *corev1.Pod, proxyContainers func(*corev1.Pod) []corev1.Container) {
				v1.MountEnvoySecret(pod, newAuthPolicy(true, true))
				Expect(pod.Spec.Volumes).To(HaveLen(1))
				Expect(pod.Spec.Volumes[0].Secret.SecretName).To(Equal(secretName))
				Expect(proxyContainers(pod)).To(ContainElement(SatisfyAll(
					HaveField("Name", "istio-proxy"),
					HaveField("VolumeMounts", ConsistOf(SatisfyAll(
						HaveField("MountPath", "/etc/istio/config"),
						HaveField("ReadOnly", true),
					))),
				)))
			},
			Entry("regular container", newPod([]corev1.Container{app, proxy}, nil),
				func(pod *corev1.Pod) []corev1.Container { return pod.Spec.Containers }),
			Entry("native sidecar init container", newPod([]corev1.Container{app}, []corev1.Container{proxy}),
				func(pod *corev1.Pod) []corev1.Container { return pod.Spec.InitContainers }),
		)

		It("is idempotent when reinvoked", func() {
			pod := newPod([]corev1.Container{app, proxy}, nil)
			v1.MountEnvoySecret(pod, newAuthPolicy(true, true))
			v1.MountEnvoySecret(pod, newAuthPolicy(true, true))
			Expect(pod.Spec.Volumes).To(HaveLen(1))
			Expect(pod.Spec.Containers[1].VolumeMounts).To(HaveLen(1))
		})
	})
})
