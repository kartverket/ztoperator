package controller_test

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/controller"
	"github.com/kartverket/ztoperator/internal/names"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	securityv1 "istio.io/client-go/pkg/apis/security/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	k8sevents "k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const controllerName = "authpolicy"

var _ = Describe("AuthPolicy Controller Owns", Ordered, func() {
	const namespace = "owns-test"

	var (
		mgrCtx    context.Context
		mgrCancel context.CancelFunc
	)

	BeforeAll(func() {
		By("installing the real Istio CRDs into envtest")
		installIstioCRDs()

		By("creating the test namespace")
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, ns))).To(Succeed())

		By("starting a manager with the real AuthPolicyReconciler wiring")
		mgr, err := manager.New(cfg, manager.Options{
			Scheme:  scheme.Scheme,
			Metrics: metricsserver.Options{BindAddress: "0"},
			// Without SkipNameValidation, the second manager to start in the test binary
			// would otherwise fail with "controller with name authpolicy already exists".
			Controller: config.Controller{SkipNameValidation: helperfunctions.Ptr(true)},
		})
		Expect(err).NotTo(HaveOccurred())

		reconciler := &controller.AuthPolicyReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
			// Large buffer so Eventf never blocks the reconcile
			Recorder:                  k8sevents.NewFakeRecorder(10000),
			DiscoveryDocumentResolver: newBasicDiscoveryResolver(),
		}
		Expect(reconciler.SetupWithManager(mgr)).To(Succeed())

		mgrCtx, mgrCancel = context.WithCancel(ctx)
		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrCtx)).To(Succeed())
		}()

		Expect(mgr.GetCache().WaitForCacheSync(mgrCtx)).To(BeTrue())
	})

	AfterAll(func() {
		if mgrCancel != nil {
			mgrCancel()
		}
	})

	Context("when the AuthPolicy has no autoLogin", Ordered, func() {
		const authPolicyName = "predicate-app"
		requestAuthKey := types.NamespacedName{Name: authPolicyName, Namespace: namespace}

		BeforeAll(func() {
			By("creating an AuthPolicy so an owned RequestAuthentication is produced")
			authPolicy := &ztoperatorv1alpha1.AuthPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      authPolicyName,
					Namespace: namespace,
				},
				Spec: ztoperatorv1alpha1.AuthPolicySpec{
					Enabled:      true,
					WellKnownURI: "https://idp.example.com/.well-known/openid-configuration",
					Selector: ztoperatorv1alpha1.WorkloadSelector{
						MatchLabels: map[string]string{"app": authPolicyName},
					},
				},
			}
			Expect(k8sClient.Create(ctx, authPolicy)).To(Succeed())

			By("waiting for the AuthPolicy to reach Ready phase")
			Eventually(func(g Gomega) {
				ap := &ztoperatorv1alpha1.AuthPolicy{}
				g.Expect(k8sClient.Get(ctx, requestAuthKey, ap)).To(Succeed())
				g.Expect(ap.Status.Phase).To(Equal(ztoperatorv1alpha1.PhaseReady))
			}, 30*time.Second, 200*time.Millisecond).Should(Succeed())

			By("waiting for reconcile activity to settle so the counter is stable")
			waitForReconcilesToSettle()
		})

		It("does not enqueue a reconcile when an owned Istio resource has an annotation-only update", func() {
			By("capturing the RA generation and reconcile counter before the patch")
			raBefore := &securityv1.RequestAuthentication{}
			Expect(k8sClient.Get(ctx, requestAuthKey, raBefore)).To(Succeed())
			before := reconcileTotal()

			By("patching only an annotation on the owned RequestAuthentication (metadata-only RawPatch)")
			annotationPatch := client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(
				`{"metadata":{"annotations":{"ztoperator.test/marker":"%d"}}}`,
				time.Now().UnixNano(),
			)))
			Expect(k8sClient.Patch(ctx, raBefore, annotationPatch)).To(Succeed())

			By("confirming the API server did not bump metadata.generation for the metadata-only patch")
			raAfter := &securityv1.RequestAuthentication{}
			Expect(k8sClient.Get(ctx, requestAuthKey, raAfter)).To(Succeed())
			Expect(raAfter.GetGeneration()).To(Equal(raBefore.GetGeneration()),
				"the API server is expected to leave metadata.generation unchanged for a "+
					"metadata-only update; if this fails, the vendored Istio CRD's status "+
					"subresource is not correctly configured (see hack/crd/bases)")

			By("asserting no reconcile is enqueued for the annotation change")
			Consistently(func() float64 {
				return reconcileTotal() - before
			}, 2*time.Second, 200*time.Millisecond).Should(BeZero(),
				"an annotation-only update on an owned RequestAuthentication should not "+
					"enqueue a reconcile")
		})

		It("enqueues a reconcile when an owned Istio resource has a label change", func() {
			By("re-settling reconciles from the previous case before sampling the baseline")
			waitForReconcilesToSettle()
			before := reconcileTotal()

			By("patching a label on the owned RequestAuthentication (metadata-only RawPatch)")
			ra := &securityv1.RequestAuthentication{}
			Expect(k8sClient.Get(ctx, requestAuthKey, ra)).To(Succeed())
			labelPatch := client.RawPatch(types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"ztoperator.test/injected":"value"}}}`))
			Expect(k8sClient.Patch(ctx, ra, labelPatch)).To(Succeed())

			By("asserting at least one reconcile fires because labels changed")
			Eventually(func() float64 {
				return reconcileTotal() - before
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1.0),
				"a label change on an owned RequestAuthentication should enqueue a reconcile")
		})

		It("enqueues a reconcile when an owned Istio resource has a spec change", func() {
			By("re-settling reconciles from the previous case before sampling the baseline")
			waitForReconcilesToSettle()

			By("capturing the RA generation before the spec change")
			raBefore := &securityv1.RequestAuthentication{}
			Expect(k8sClient.Get(ctx, requestAuthKey, raBefore)).To(Succeed())
			before := reconcileTotal()

			By("modifying spec.selector.matchLabels and updating the owned RequestAuthentication")
			Expect(raBefore.Spec.Selector).NotTo(BeNil(),
				"the fixture RA is expected to have a non-nil WorkloadSelector")
			Expect(raBefore.Spec.Selector.MatchLabels).NotTo(BeNil(),
				"the fixture RA is expected to have a non-nil MatchLabels map")
			modified := raBefore.DeepCopy()
			modified.Spec.Selector.MatchLabels["ztoperator.test/drift"] = "yes"
			// A full PUT via Update unambiguously constitutes a spec change and is not subject to
			// merge-patch semantics that could otherwise be no-op'd by API server defaulting/
			// normalization on the real Istio schema.
			Expect(k8sClient.Update(ctx, modified)).To(Succeed())

			By("confirming the API server bumped metadata.generation for the spec change")
			raAfter := &securityv1.RequestAuthentication{}
			Expect(k8sClient.Get(ctx, requestAuthKey, raAfter)).To(Succeed())
			Expect(raAfter.GetGeneration()).To(BeNumerically(">", raBefore.GetGeneration()),
				"the spec change must actually bump generation, otherwise this case is not "+
					"exercising the behavior it claims to")

			By("asserting at least one reconcile fires because generation changed")
			Eventually(func() float64 {
				return reconcileTotal() - before
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1.0),
				"a spec change on an owned RequestAuthentication should enqueue a reconcile so "+
					"the operator can drive the child back to its desired state")
		})

		It("enqueues a reconcile when the owned require-AuthorizationPolicy has a label change", func() {
			By("re-settling reconciles from the previous case before sampling the baseline")
			waitForReconcilesToSettle()
			before := reconcileTotal()

			By("patching a label on the owned require-AuthorizationPolicy (metadata-only RawPatch)")
			requireAP := &securityv1.AuthorizationPolicy{}
			requireAPKey := types.NamespacedName{
				Name:      names.RequirePolicy(authPolicyName),
				Namespace: namespace,
			}
			Expect(k8sClient.Get(ctx, requireAPKey, requireAP)).To(Succeed())
			labelPatch := client.RawPatch(types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"ztoperator.test/injected":"value"}}}`))
			Expect(k8sClient.Patch(ctx, requireAP, labelPatch)).To(Succeed())

			By("asserting at least one reconcile fires because labels changed")
			Eventually(func() float64 {
				return reconcileTotal() - before
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1.0),
				"a label change on the owned require-AuthorizationPolicy should enqueue a "+
					"reconcile - if this fails but the RequestAuthentication cases pass, the "+
					"AuthorizationPolicy binding is likely broken")
		})
	})

	Context("when the AuthPolicy has autoLogin enabled", Ordered, func() {
		const (
			authPolicyName  = "autologin-app"
			oauthSecretName = "autologin-app-oauth-creds"
		)
		var (
			authPolicyKey  = types.NamespacedName{Name: authPolicyName, Namespace: namespace}
			envoyFilterKey = types.NamespacedName{Name: names.EnvoyFilter(authPolicyName), Namespace: namespace}
			envoySecretKey = types.NamespacedName{Name: names.EnvoySecret(authPolicyName), Namespace: namespace}
		)

		BeforeAll(func() {
			By("creating the OAuth client-credentials Secret consumed by autoLogin")
			oauthSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      oauthSecretName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"clientId":     []byte("test-client-id"),
					"clientSecret": []byte("test-client-secret"),
				},
			}
			Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, oauthSecret))).To(Succeed())

			By("creating an AuthPolicy with autoLogin enabled")
			authPolicy := &ztoperatorv1alpha1.AuthPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      authPolicyName,
					Namespace: namespace,
				},
				Spec: ztoperatorv1alpha1.AuthPolicySpec{
					Enabled:      true,
					WellKnownURI: "https://idp.example.com/.well-known/openid-configuration",
					Selector: ztoperatorv1alpha1.WorkloadSelector{
						MatchLabels: map[string]string{"app": authPolicyName},
					},
					AutoLogin: &ztoperatorv1alpha1.AutoLogin{
						Enabled: true,
						Scopes:  []string{"openid"},
					},
					OAuthCredentials: &ztoperatorv1alpha1.OAuthCredentials{
						SecretRef:       oauthSecretName,
						ClientIDKey:     "clientId",
						ClientSecretKey: "clientSecret",
					},
				},
			}
			Expect(k8sClient.Create(ctx, authPolicy)).To(Succeed())

			By("waiting for the AuthPolicy to reach Ready phase")
			Eventually(func(g Gomega) {
				ap := &ztoperatorv1alpha1.AuthPolicy{}
				g.Expect(k8sClient.Get(ctx, authPolicyKey, ap)).To(Succeed())
				g.Expect(ap.Status.Phase).To(Equal(ztoperatorv1alpha1.PhaseReady))
			}, 30*time.Second, 200*time.Millisecond).Should(Succeed())

			By("confirming both autoLogin child resources were created")
			Expect(k8sClient.Get(ctx, envoyFilterKey, &v1alpha4.EnvoyFilter{})).To(Succeed())
			Expect(k8sClient.Get(ctx, envoySecretKey, &corev1.Secret{})).To(Succeed())

			By("waiting for reconcile activity to settle so the counter is stable")
			waitForReconcilesToSettle()
		})

		It("enqueues a reconcile when the owned EnvoyFilter has a label change", func() {
			By("re-settling reconciles from any prior case before sampling the baseline")
			waitForReconcilesToSettle()
			before := reconcileTotal()

			By("patching a label on the owned EnvoyFilter (metadata-only RawPatch)")
			ef := &v1alpha4.EnvoyFilter{}
			Expect(k8sClient.Get(ctx, envoyFilterKey, ef)).To(Succeed())
			labelPatch := client.RawPatch(types.MergePatchType,
				[]byte(`{"metadata":{"labels":{"ztoperator.test/injected":"value"}}}`))
			Expect(k8sClient.Patch(ctx, ef, labelPatch)).To(Succeed())

			By("asserting at least one reconcile fires because labels changed")
			Eventually(func() float64 {
				return reconcileTotal() - before
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1.0),
				"a label change on the owned EnvoyFilter should enqueue a reconcile - if this "+
					"fails but the RequestAuthentication cases pass, the EnvoyFilter binding is "+
					"likely broken")
		})

		It("does not enqueue a reconcile when the owned envoy-secret has an annotation-only update", func() {
			By("re-settling reconciles from the previous case before sampling the baseline")
			waitForReconcilesToSettle()
			before := reconcileTotal()

			By("patching only an annotation on the owned envoy-secret (metadata-only RawPatch)")
			envoySecret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, envoySecretKey, envoySecret)).To(Succeed())
			annotationPatch := client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(
				`{"metadata":{"annotations":{"ztoperator.test/marker":"%d"}}}`,
				time.Now().UnixNano(),
			)))
			Expect(k8sClient.Patch(ctx, envoySecret, annotationPatch)).To(Succeed())

			By("asserting no reconcile is enqueued for the annotation change")
			Consistently(func() float64 {
				return reconcileTotal() - before
			}, 2*time.Second, 200*time.Millisecond).Should(BeZero(),
				"an annotation-only update on the owned envoy-secret should not enqueue a reconcile")
		})

		It("enqueues a reconcile when the owned envoy-secret has a data change", func() {
			By("re-settling reconciles from the previous case before sampling the baseline")
			waitForReconcilesToSettle()
			before := reconcileTotal()

			By("adding an unrelated data key on the owned envoy-secret")
			envoySecret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, envoySecretKey, envoySecret)).To(Succeed())
			modified := envoySecret.DeepCopy()
			if modified.Data == nil {
				modified.Data = map[string][]byte{}
			}

			modified.Data["ztoperator-test-drift"] = []byte("yes")
			Expect(k8sClient.Update(ctx, modified)).To(Succeed())

			By("asserting at least one reconcile fires because secret data changed")
			Eventually(func() float64 {
				return reconcileTotal() - before
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1.0),
				"a data change on the owned envoy-secret should enqueue a reconcile - if this "+
					"fails but the other cases pass, the Secret binding is likely broken")
		})
	})
})

func reconcileTotal() float64 {
	mfs, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		return -1
	}
	var total float64
	for _, mf := range mfs {
		if mf.GetName() != "controller_runtime_reconcile_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "controller" && l.GetValue() == controllerName {
					total += m.GetCounter().GetValue()
					break
				}
			}
		}
	}
	return total
}

// waitForReconcilesToSettle polls the reconcile counter until it stops changing for a one-second
// quiescent window, so subsequent assertions have a stable baseline.
func waitForReconcilesToSettle() {
	const quiescentWindow = 1 * time.Second
	var lastCount float64
	Eventually(func() bool {
		current := reconcileTotal()
		if current == lastCount {
			return true
		}
		lastCount = current
		return false
	}, 30*time.Second, quiescentWindow).Should(BeTrue(),
		"reconciles never settled; either the controller is stuck in a loop or the "+
			"quiescent window is too short")
}

func installIstioCRDs() {
	_, err := envtest.InstallCRDs(cfg, envtest.CRDInstallOptions{
		Paths:              []string{filepath.Join("..", "..", "hack", "crd", "bases")},
		ErrorIfPathMissing: true,
	})
	Expect(err).NotTo(HaveOccurred())
}
