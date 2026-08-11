package controller_test

import (
	"context"
	"errors"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/controller"
	"github.com/kartverket/ztoperator/internal/names"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/rest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	securityv1 "istio.io/client-go/pkg/apis/security/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sevents "k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakeDiscoveryDocumentResolver struct {
	document *rest.DiscoveryDocument
	err      error
}

func (f *fakeDiscoveryDocumentResolver) GetOAuthDiscoveryDocument(
	_ string,
	_ log.Logger,
) (*rest.DiscoveryDocument, error) {
	return f.document, f.err
}

func newBasicDiscoveryResolver() *fakeDiscoveryDocumentResolver {
	return &fakeDiscoveryDocumentResolver{
		document: &rest.DiscoveryDocument{
			Issuer:        helperfunctions.Ptr("https://idp.example.com"),
			JwksURI:       helperfunctions.Ptr("https://idp.example.com/jwks"),
			TokenEndpoint: helperfunctions.Ptr("https://idp.example.com/token"),
		},
	}
}

var _ = Describe("AuthPolicy Controller Reconcile", func() {
	var (
		testCtx    context.Context
		testScheme *runtime.Scheme
		fakeClient client.Client
		reconciler *controller.AuthPolicyReconciler
	)

	const (
		namespace = "test-controller"
		appName   = "my-app"
	)

	BeforeEach(func() {
		testCtx = context.Background()

		testScheme = runtime.NewScheme()
		Expect(ztoperatorv1alpha1.AddToScheme(testScheme)).To(Succeed())
		Expect(v1alpha4.AddToScheme(testScheme)).To(Succeed())
		Expect(securityv1.AddToScheme(testScheme)).To(Succeed())
		Expect(corev1.AddToScheme(testScheme)).To(Succeed())

		authPolicy := &ztoperatorv1alpha1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:       appName,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: ztoperatorv1alpha1.AuthPolicySpec{
				Enabled:      true,
				WellKnownURI: "https://idp.example.com/.well-known/openid-configuration",
				Selector: ztoperatorv1alpha1.WorkloadSelector{
					MatchLabels: map[string]string{"app": appName},
				},
			},
		}

		fakeClient = fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(authPolicy).
			WithStatusSubresource(authPolicy).
			Build()

		reconciler = &controller.AuthPolicyReconciler{
			Client:                    fakeClient,
			Scheme:                    testScheme,
			Recorder:                  k8sevents.NewFakeRecorder(100),
			DiscoveryDocumentResolver: newBasicDiscoveryResolver(),
		}
	})

	Context("a simple enabled AuthPolicy without auto-login", func() {
		It("creates auth resources, skips auto-login resources, and reports Ready status", func() {
			result, err := reconciler.Reconcile(testCtx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: appName, Namespace: namespace},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			ra := &securityv1.RequestAuthentication{}
			Expect(fakeClient.Get(testCtx, types.NamespacedName{Name: appName, Namespace: namespace}, ra)).To(Succeed())
			Expect(ra.Spec.GetJwtRules()).To(HaveLen(1))
			Expect(ra.Spec.GetJwtRules()[0].GetIssuer()).To(Equal("https://idp.example.com"))
			Expect(ra.Spec.GetJwtRules()[0].GetJwksUri()).To(Equal("https://idp.example.com/jwks"))
			Expect(ra.Spec.GetSelector().GetMatchLabels()).To(Equal(map[string]string{"app": appName}))

			owner := &ztoperatorv1alpha1.AuthPolicy{}
			Expect(fakeClient.Get(testCtx, types.NamespacedName{Name: appName, Namespace: namespace}, owner)).To(Succeed())
			Expect(metav1.IsControlledBy(ra, owner)).To(BeTrue())

			requirePolicy := &securityv1.AuthorizationPolicy{}
			Expect(fakeClient.Get(testCtx, types.NamespacedName{
				Name:      names.RequirePolicy(appName),
				Namespace: namespace,
			}, requirePolicy)).To(Succeed())
			Expect(metav1.IsControlledBy(requirePolicy, owner)).To(BeTrue())

			ef := &v1alpha4.EnvoyFilter{}
			efErr := fakeClient.Get(testCtx, types.NamespacedName{Name: names.EnvoyFilter(appName), Namespace: namespace}, ef)
			Expect(apierrors.IsNotFound(efErr)).To(BeTrue())

			envoySecret := &corev1.Secret{}
			secretErr := fakeClient.Get(
				testCtx,
				types.NamespacedName{Name: names.EnvoySecret(appName), Namespace: namespace},
				envoySecret,
			)
			Expect(apierrors.IsNotFound(secretErr)).To(BeTrue())

			denyPolicy := &securityv1.AuthorizationPolicy{}
			denyErr := fakeClient.Get(testCtx, types.NamespacedName{Name: names.DenyPolicy(appName), Namespace: namespace}, denyPolicy)
			Expect(apierrors.IsNotFound(denyErr)).To(BeTrue())

			ignorePolicy := &securityv1.AuthorizationPolicy{}
			ignoreErr := fakeClient.Get(testCtx, types.NamespacedName{Name: names.IgnorePolicy(appName), Namespace: namespace}, ignorePolicy)
			Expect(apierrors.IsNotFound(ignoreErr)).To(BeTrue())

			updatedPolicy := &ztoperatorv1alpha1.AuthPolicy{}
			Expect(fakeClient.Get(testCtx, types.NamespacedName{Name: appName, Namespace: namespace}, updatedPolicy)).To(Succeed())
			Expect(updatedPolicy.Status.Phase).To(Equal(ztoperatorv1alpha1.PhaseReady))
			Expect(updatedPolicy.Status.Ready).To(BeTrue())
			Expect(updatedPolicy.Status.ObservedGeneration).To(Equal(int64(1)))
		})
	})

	Context("when the discovery document resolver returns an error", func() {
		It("returns the error, sets status to Failed, and does not create child resources", func() {
			By("configuring the resolver to return an error")
			resolveErr := errors.New("discovery resolver failed")
			reconciler.DiscoveryDocumentResolver = &fakeDiscoveryDocumentResolver{err: resolveErr}

			By("reconciling the AuthPolicy")
			result, err := reconciler.Reconcile(testCtx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: appName, Namespace: namespace},
			})
			Expect(err).To(MatchError(ContainSubstring(resolveErr.Error())))
			Expect(result).To(Equal(ctrl.Result{}))

			By("verifying status is set to Failed")
			updatedPolicy := &ztoperatorv1alpha1.AuthPolicy{}
			Expect(fakeClient.Get(testCtx, types.NamespacedName{Name: appName, Namespace: namespace}, updatedPolicy)).To(Succeed())
			Expect(updatedPolicy.Status.Phase).To(Equal(ztoperatorv1alpha1.PhaseFailed))
			Expect(updatedPolicy.Status.Ready).To(BeFalse())
			Expect(updatedPolicy.Status.Message).To(ContainSubstring(resolveErr.Error()))
			Expect(updatedPolicy.Status.ObservedGeneration).To(Equal(int64(1)))

			By("verifying no child resources were created")
			ra := &securityv1.RequestAuthentication{}
			Expect(apierrors.IsNotFound(
				fakeClient.Get(testCtx, types.NamespacedName{Name: appName, Namespace: namespace}, ra),
			)).To(BeTrue())

			requirePolicy := &securityv1.AuthorizationPolicy{}
			Expect(apierrors.IsNotFound(
				fakeClient.Get(testCtx, types.NamespacedName{Name: names.RequirePolicy(appName), Namespace: namespace}, requirePolicy),
			)).To(BeTrue())
		})
	})
})
