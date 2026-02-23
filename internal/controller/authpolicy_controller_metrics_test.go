package controller_test

import (
	"context"
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/metrics"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var _ = Describe("AuthPolicy Controller", func() {

	const (
		authPolicyName      = "metrics-test-auth-policy"
		authPolicyNamespace = "metrics-test-ns"
		wellKnownURI        = "https://login.example.com/.well-known/openid-configuration"
		metricsURL          = "http://localhost:8181/metrics"
	)

	var (
		mgrCancel context.CancelFunc
		mgrCtx    context.Context
	)

	BeforeEach(func() {
		By("creating the test namespace")
		ns := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: authPolicyNamespace,
				Labels: map[string]string{
					"team": "test-team",
				},
			},
		}
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ns), ns)
		if err != nil {
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		}

		By("starting a manager with metrics on :8181")
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme.Scheme,
			Metrics: metricsserver.Options{
				BindAddress:   ":8181",
				SecureServing: false,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		mgrCtx, mgrCancel = context.WithCancel(ctx)
		go func() {
			defer GinkgoRecover()
			err := mgr.Start(mgrCtx)
			Expect(err).NotTo(HaveOccurred())
		}()

		By("waiting for metrics endpoint to be ready")
		Eventually(func() error {
			_, err := getMetrics(metricsURL)
			if err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
	})

	AfterEach(func() {
		By("stopping the manager")
		mgrCancel()
	})

	It("should expose ztoperator_authpolicy_info metric at :8181/metrics after applying an AuthPolicy", func() {
		By("creating an AuthPolicy resource")
		authPolicy := &ztoperatorv1alpha1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      authPolicyName,
				Namespace: authPolicyNamespace,
			},
			Spec: ztoperatorv1alpha1.AuthPolicySpec{
				Enabled:      true,
				WellKnownURI: wellKnownURI,
				Selector: ztoperatorv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{
						"app": "test-app",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, authPolicy)).To(Succeed())

		By("refreshing metrics for the AuthPolicy")
		Expect(metrics.RefreshAuthPolicyInfo(ctx, k8sClient, *authPolicy)).To(Succeed())

		By("fetching metrics from :8181/metrics")
		metricsBody, err := getMetrics(metricsURL)
		Expect(err).NotTo(HaveOccurred())

		By("verifying ztoperator_authpolicy_info metric is present with correct labels")
		Expect(metricsBody).To(ContainSubstring("ztoperator_authpolicy_info"))
		Expect(metricsBody).To(ContainSubstring(fmt.Sprintf(`name="%s"`, authPolicyName)))
		Expect(metricsBody).To(ContainSubstring(fmt.Sprintf(`namespace="%s"`, authPolicyNamespace)))
		Expect(metricsBody).To(ContainSubstring(`enabled="true"`))
		Expect(metricsBody).To(ContainSubstring(`auto_login_enabled="false"`))
		Expect(metricsBody).To(ContainSubstring(`issuer="https://login.example.com"`))

		By("cleaning up")
		Expect(k8sClient.Delete(ctx, authPolicy)).To(Succeed())
	})

	It("should include protected_pod label when a matching pod exists", func() {
		By("creating a pod that matches the AuthPolicy selector")
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "protected-test-pod",
				Namespace: authPolicyNamespace,
				Labels: map[string]string{
					"app": "test-app-with-pod",
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "test-container",
						Image: "busybox",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())

		By("creating an AuthPolicy targeting the pod")
		authPolicy := &ztoperatorv1alpha1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-pod", authPolicyName),
				Namespace: authPolicyNamespace,
			},
			Spec: ztoperatorv1alpha1.AuthPolicySpec{
				Enabled:      true,
				WellKnownURI: wellKnownURI,
				Selector: ztoperatorv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{
						"app": "test-app-with-pod",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, authPolicy)).To(Succeed())

		By("refreshing metrics for the AuthPolicy")
		Expect(metrics.RefreshAuthPolicyInfo(ctx, k8sClient, *authPolicy)).To(Succeed())

		By("fetching metrics from :8181/metrics")
		metricsBody, err := getMetrics(metricsURL)
		Expect(err).NotTo(HaveOccurred())

		By("verifying the protected_pod label is set to the pod name")
		Expect(metricsBody).To(ContainSubstring(`protected_pod="protected-test-pod"`))

		By("cleaning up")
		Expect(k8sClient.Delete(ctx, authPolicy)).To(Succeed())
		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
	})

	It("should remove metrics from :8181/metrics when the AuthPolicy is deleted", func() {
		By("creating an AuthPolicy")
		authPolicy := &ztoperatorv1alpha1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-del", authPolicyName),
				Namespace: authPolicyNamespace,
			},
			Spec: ztoperatorv1alpha1.AuthPolicySpec{
				Enabled:      true,
				WellKnownURI: wellKnownURI,
				Selector: ztoperatorv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{
						"app": "test-app-delete",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, authPolicy)).To(Succeed())

		By("refreshing metrics for the AuthPolicy")
		Expect(metrics.RefreshAuthPolicyInfo(ctx, k8sClient, *authPolicy)).To(Succeed())

		By("fetching metrics from :8181/metrics")
		metricsBody, err := getMetrics(metricsURL)
		Expect(err).NotTo(HaveOccurred())

		Expect(metricsBody).To(ContainSubstring(fmt.Sprintf(`name="%s-del"`, authPolicyName)))

		By("deleting the AuthPolicy metrics info")
		metrics.DeleteAuthPolicyInfo(client.ObjectKeyFromObject(authPolicy))

		By("verifying metric is removed from :8181/metrics")
		metricsBody, err = getMetrics(metricsURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(metricsBody).NotTo(ContainSubstring(fmt.Sprintf(`name="%s-del"`, authPolicyName)))

		By("cleaning up")
		Expect(k8sClient.Delete(ctx, authPolicy)).To(Succeed())
	})
})

func getMetrics(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
