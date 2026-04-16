package v1_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	skiperatorv1 "github.com/kartverket/skiperator/api/v1alpha1"
	ztoperatorv1 "github.com/kartverket/ztoperator/api/v1alpha1"
	v1 "github.com/kartverket/ztoperator/internal/webhook/v1"
	"github.com/kartverket/ztoperator/pkg/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	k8sClient client.Client
	cfg       *rest.Config
	testEnv   *envtest.Environment
)

const skiperatorAppName = "skiperator-app"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Webhook Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = skiperatorv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = ztoperatorv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// Load environment variables
	err = os.Setenv("ZTOPERATOR_CLUSTER_NAME", "test-cluster")
	Expect(err).NotTo(HaveOccurred())
	err = config.Load()
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "..", "hack", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,

		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook", "bases")},
		},
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// start webhook server using Manager.
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookInstallOptions.LocalServingHost,
			Port:    webhookInstallOptions.LocalServingPort,
			CertDir: webhookInstallOptions.LocalServingCertDir,
		}),
		LeaderElection: false,
		Metrics:        metricsserver.Options{BindAddress: "0"},
	})
	Expect(err).NotTo(HaveOccurred())

	err = v1.SetupPodWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:webhook

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	// wait for the webhook server to get ready.
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}

		return conn.Close()
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

func getWebhookEnabledNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"ztoperator-webhooks": "enabled",
			},
		},
	}
}

var _ = Describe("Pod validating webhook", func() {
	It("does not create when pod is annotated correctly and authpolicy does not exist", func() {
		ns := getWebhookEnabledNamespace("pod-webhook-create-fail-ns")
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		skiperatorAppName := skiperatorAppName
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-webhook-create",
				Namespace: ns.Name,
				Annotations: map[string]string{
					v1.ZtoperatorVerifyAnnotationKey: v1.ZtoperatorVerifyAnnotationValue,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  skiperatorAppName,
					Image: "nginx:stable",
				}},
			},
		}
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		pod.Labels[v1.SkiperatorApplicationRefLabel] = skiperatorAppName

		Expect(k8sClient.Create(ctx, pod)).To(MatchError(ContainSubstring("no AuthPolicy resource was found for the corresponding Application")))

		Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
	})

	It("creates when pod is annotated correctly and authpolicy exists", func() {
		ns := getWebhookEnabledNamespace("pod-webhook-create-succeed-ns")
		skiperatorAppName := skiperatorAppName
		authPolicyName := "auth-policy"
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })

		authPolicy := ztoperatorv1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      authPolicyName,
				Namespace: ns.GetName(),
			},
			Spec: ztoperatorv1.AuthPolicySpec{
				Selector: ztoperatorv1.WorkloadSelector{
					MatchLabels: map[string]string{"app": skiperatorAppName},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &authPolicy)).To(Succeed())
		authPolicy.Status.Ready = true
		Expect(k8sClient.Status().Update(ctx, &authPolicy)).To(Succeed())

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-webhook-create",
				Namespace: ns.Name,
				Annotations: map[string]string{
					v1.ZtoperatorVerifyAnnotationKey: v1.ZtoperatorVerifyAnnotationValue,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  skiperatorAppName,
					Image: "nginx:stable",
				}},
			},
		}
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		pod.Labels[v1.SkiperatorApplicationRefLabel] = skiperatorAppName

		Expect(k8sClient.Create(ctx, pod)).To(Succeed())

		Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
	})

	It("does not create when pod is annotated correctly and authpolicy for different app exists", func() {
		ns := getWebhookEnabledNamespace("pod-webhook-different-app-ns")
		skiperatorAppName := skiperatorAppName
		authPolicyName := "auth-policy"
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })

		authPolicy := ztoperatorv1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      authPolicyName,
				Namespace: ns.GetName(),
			},
			Spec: ztoperatorv1.AuthPolicySpec{
				Selector: ztoperatorv1.WorkloadSelector{
					MatchLabels: map[string]string{"app": skiperatorAppName + "not"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &authPolicy)).To(Succeed())
		authPolicy.Status.Ready = true
		Expect(k8sClient.Status().Update(ctx, &authPolicy)).To(Succeed())

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-webhook-create",
				Namespace: ns.Name,
				Annotations: map[string]string{
					v1.ZtoperatorVerifyAnnotationKey: v1.ZtoperatorVerifyAnnotationValue,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  skiperatorAppName,
					Image: "nginx:stable",
				}},
			},
		}
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}
		pod.Labels[v1.SkiperatorApplicationRefLabel] = skiperatorAppName

		Expect(k8sClient.Create(ctx, pod)).To(MatchError(ContainSubstring("no AuthPolicy resource was found for the corresponding Application")))

		Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
	})
})
