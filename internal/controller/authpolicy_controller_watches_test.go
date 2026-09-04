package controller_test

import (
	"context"
	"fmt"
	"time"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/controller"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	k8sevents "k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var _ = Describe("AuthPolicy Controller Watches", Ordered, func() {
	const (
		namespace      = "watches-test"
		authPolicyName = "watches-app"
	)

	var (
		mgrCtx        context.Context
		mgrCancel     context.CancelFunc
		authPolicyKey = types.NamespacedName{Name: authPolicyName, Namespace: namespace}
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

		By("creating an AuthPolicy")
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
			g.Expect(k8sClient.Get(ctx, authPolicyKey, ap)).To(Succeed())
			g.Expect(ap.Status.Phase).To(Equal(ztoperatorv1alpha1.PhaseReady))
		}, 30*time.Second, 200*time.Millisecond).Should(Succeed())

		By("waiting for reconcile activity to settle so the counter is stable")
		waitForReconcilesToSettle()
	})

	AfterAll(func() {
		if mgrCancel != nil {
			mgrCancel()
		}
	})

	Context("when an unrelated Secret changes in the namespace", func() {
		var secretKey types.NamespacedName

		BeforeEach(func() {
			By("creating an unrelated, unowned Secret in the namespace")
			s := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("unrelated-secret-%d", time.Now().UnixNano()),
					Namespace: namespace,
				},
				Data: map[string][]byte{"some-key": []byte("some-value")},
			}
			Expect(k8sClient.Create(ctx, s)).To(Succeed())
			secretKey = client.ObjectKeyFromObject(s)

			waitForReconcilesToSettle()
		})

		It("does not enqueue a reconcile for an annotation-only update", func() {
			before := reconcileTotal()

			By("patching only an annotation on the unrelated Secret (metadata-only RawPatch)")
			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, secretKey, s)).To(Succeed())
			annotationPatch := client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(
				`{"metadata":{"annotations":{"ztoperator.test/marker":"%d"}}}`,
				time.Now().UnixNano(),
			)))
			Expect(k8sClient.Patch(ctx, s, annotationPatch)).To(Succeed())

			By("asserting no AuthPolicy in the namespace is reconciled")
			Consistently(func() float64 {
				return reconcileTotal() - before
			}, 2*time.Second, 200*time.Millisecond).Should(BeZero(),
				"annotation-only update on an unrelated Secret should be filtered by "+
					"predicates.SecretContentOrLabelsChanged before it ever reaches "+
					"eventhandler.EnqueueAuthPoliciesInNamespace")
		})

		It("enqueues a reconcile when the Secret's data changes", func() {
			before := reconcileTotal()

			By("changing the Secret's data")
			s := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, secretKey, s)).To(Succeed())
			modified := s.DeepCopy()
			modified.Data["some-key"] = []byte("some-other-value")
			Expect(k8sClient.Update(ctx, modified)).To(Succeed())

			By("asserting the AuthPolicy in the namespace is reconciled")
			Eventually(func() float64 {
				return reconcileTotal() - before
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1.0),
				"a data change on an unrelated Secret should pass predicates.SecretContentOrLabelsChanged "+
					"and fan out to a reconcile of the AuthPolicy in the namespace")
		})
	})

	Context("when an unrelated ConfigMap changes in the namespace", func() {
		var configMapKey types.NamespacedName

		BeforeEach(func() {
			By("creating an unrelated, unowned ConfigMap in the namespace")
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("unrelated-configmap-%d", time.Now().UnixNano()),
					Namespace: namespace,
				},
				Data: map[string]string{"AUDIENCE": "some-audience"},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())
			configMapKey = client.ObjectKeyFromObject(cm)

			waitForReconcilesToSettle()
		})

		It("does not enqueue a reconcile for an annotation-only update", func() {
			before := reconcileTotal()

			By("patching only an annotation on the unrelated ConfigMap (metadata-only RawPatch)")
			cm := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, configMapKey, cm)).To(Succeed())
			annotationPatch := client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(
				`{"metadata":{"annotations":{"ztoperator.test/marker":"%d"}}}`,
				time.Now().UnixNano(),
			)))
			Expect(k8sClient.Patch(ctx, cm, annotationPatch)).To(Succeed())

			By("asserting no AuthPolicy in the namespace is reconciled")
			Consistently(func() float64 {
				return reconcileTotal() - before
			}, 2*time.Second, 200*time.Millisecond).Should(BeZero(),
				"annotation-only update on an unrelated ConfigMap should be filtered by "+
					"predicates.ConfigMapContentOrLabelsChanged before it ever reaches "+
					"eventhandler.EnqueueAuthPoliciesInNamespace")
		})

		It("enqueues a reconcile when the ConfigMap's data changes", func() {
			before := reconcileTotal()

			By("changing the ConfigMap's data")
			cm := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, configMapKey, cm)).To(Succeed())
			modified := cm.DeepCopy()
			modified.Data["AUDIENCE"] = "some-other-audience"
			Expect(k8sClient.Update(ctx, modified)).To(Succeed())

			By("asserting the AuthPolicy in the namespace is reconciled")
			Eventually(func() float64 {
				return reconcileTotal() - before
			}, 5*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1.0),
				"a data change on an unrelated ConfigMap should pass "+
					"predicates.ConfigMapContentOrLabelsChanged and fan out to a reconcile of the "+
					"AuthPolicy in the namespace")
		})
	})
})
