package controller_test

import (
	"context"
	"os"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/controller"
	"github.com/kartverket/ztoperator/internal/names"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/constants"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	securityv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
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
		Expect(appsv1.AddToScheme(testScheme)).To(Succeed())

		authPolicy := &ztoperatorv1alpha1.AuthPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:       appName,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: ztoperatorv1alpha1.AuthPolicySpec{
				Enabled:      true,
				WellKnownURI: "http://mock-oauth2.auth:8080/entraid/.well-known/openid-configuration",
				AllowedAudiences: []ztoperatorv1alpha1.AllowedAudience{{
					Value: helperfunctions.Ptr("test-audience"),
				}},
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
			Client:   fakeClient,
			Scheme:   testScheme,
			Recorder: k8sevents.NewFakeRecorder(100),
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
			Expect(ra.Spec.GetJwtRules()[0].GetIssuer()).To(Equal("http://mock-oauth2.auth:8080/entraid"))
			Expect(ra.Spec.GetJwtRules()[0].GetJwksUri()).To(Equal("http://mock-oauth2.auth:8080/entraid/jwks"))
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

	Context("when a resolved audience reference is deleted", func() {
		It("sets Failed status and keeps the last successfully reconciled resources", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "audience-secret", Namespace: namespace},
				Data:       map[string][]byte{"audience": []byte("resolved-audience")},
			}
			Expect(fakeClient.Create(testCtx, secret)).To(Succeed())

			authPolicy := &ztoperatorv1alpha1.AuthPolicy{}
			policyKey := types.NamespacedName{Name: appName, Namespace: namespace}
			Expect(fakeClient.Get(testCtx, policyKey, authPolicy)).To(Succeed())
			authPolicy.Spec.AllowedAudiences = []ztoperatorv1alpha1.AllowedAudience{{
				ValueFrom: &ztoperatorv1alpha1.ValueFrom{
					SecretKeyRef: &ztoperatorv1alpha1.KeyRef{Name: secret.Name, Key: "audience"},
				},
			}}
			Expect(fakeClient.Update(testCtx, authPolicy)).To(Succeed())

			request := ctrl.Request{NamespacedName: policyKey}
			_, err := reconciler.Reconcile(testCtx, request)
			Expect(err).NotTo(HaveOccurred())

			requestAuthentication := &securityv1.RequestAuthentication{}
			Expect(fakeClient.Get(testCtx, policyKey, requestAuthentication)).To(Succeed())
			Expect(requestAuthentication.Spec.JwtRules[0].Audiences).To(Equal([]string{"resolved-audience"}))

			Expect(fakeClient.Delete(testCtx, secret)).To(Succeed())

			_, err = reconciler.Reconcile(testCtx, request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to resolve audiences"))

			updatedPolicy := &ztoperatorv1alpha1.AuthPolicy{}
			Expect(fakeClient.Get(testCtx, policyKey, updatedPolicy)).To(Succeed())
			Expect(updatedPolicy.Status.Phase).To(Equal(ztoperatorv1alpha1.PhaseFailed))

			unchangedRequestAuthentication := &securityv1.RequestAuthentication{}
			Expect(fakeClient.Get(testCtx, policyKey, unchangedRequestAuthentication)).To(Succeed())
			Expect(unchangedRequestAuthentication.Spec.JwtRules[0].Audiences).
				To(Equal([]string{"resolved-audience"}))
		})
	})

	Context("when the AuthPolicy selects an Istio Ambient enrolled workload", func() {
		policyKey := types.NamespacedName{Name: appName, Namespace: namespace}

		BeforeEach(func() {
			Expect(os.Setenv("ZTOPERATOR_FEATURE_ISTIO_AMBIENT", "true")).To(Succeed())
			Expect(config.Load()).To(Succeed())
			DeferCleanup(func() {
				Expect(os.Unsetenv("ZTOPERATOR_FEATURE_ISTIO_AMBIENT")).To(Succeed())
				Expect(config.Load()).To(Succeed())
			})
		})

		createAmbientWorkload := func() {
			Expect(fakeClient.Create(testCtx, &corev1.Namespace{
				Name:   namespace,
				Labels: map[string]string{constants.IstioAmbientLabelKey: constants.IstioAmbientLabelValue},
			})).To(Succeed())

			Expect(fakeClient.Create(testCtx, &appsv1.Deployment{
				Name:      appName,
				Namespace: namespace,
				Labels:    map[string]string{constants.SkiperatorApplicationRefLabel: appName},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": appName}},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": appName}},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  appName,
								Image: "empty",
								Ports: []corev1.ContainerPort{{
									Name:          constants.SkiperatorAppContainerPortName,
									ContainerPort: 8080,
								}},
							}},
						},
					},
				},
			})).To(Succeed())
		}

		expectNotFound := func(obj client.Object, name string) {
			err := fakeClient.Get(testCtx, types.NamespacedName{Name: name, Namespace: namespace}, obj)
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected %T %s to not exist", obj, name)
		}

		expectSidecarResourcesAbsent := func() {
			expectNotFound(&securityv1.RequestAuthentication{}, appName)
			expectNotFound(&securityv1.AuthorizationPolicy{}, names.DenyPolicy(appName))
			expectNotFound(&securityv1.AuthorizationPolicy{}, names.IgnorePolicy(appName))
			expectNotFound(&securityv1.AuthorizationPolicy{}, names.RequirePolicy(appName))
			expectNotFound(&v1alpha4.EnvoyFilter{}, names.EnvoyFilter(appName))
		}

		expectBootstrapConfigMap := func(owner *ztoperatorv1alpha1.AuthPolicy) {
			cm := &corev1.ConfigMap{}
			Expect(fakeClient.Get(testCtx, types.NamespacedName{
				Name:      names.EnvoyBootstrapConfigMap(appName),
				Namespace: namespace,
			}, cm)).To(Succeed())
			Expect(cm.Data).To(HaveKey(constants.EnvoyBootstrapConfigMapKey))
			Expect(cm.Data[constants.EnvoyBootstrapConfigMapKey]).NotTo(BeEmpty())
			Expect(metav1.IsControlledBy(cm, owner)).To(BeTrue())
		}

		reconcileAndGetPolicy := func() *ztoperatorv1alpha1.AuthPolicy {
			result, err := reconciler.Reconcile(testCtx, ctrl.Request{NamespacedName: policyKey})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			updated := &ztoperatorv1alpha1.AuthPolicy{}
			Expect(fakeClient.Get(testCtx, policyKey, updated)).To(Succeed())
			return updated
		}

		When("and auto login is disabled", func() {
			It("creates a ONLY a ConfigMap holding the Envoy bootstrap configuration", func() {
				createAmbientWorkload()

				updated := reconcileAndGetPolicy()

				expectBootstrapConfigMap(updated)
				expectNotFound(&corev1.Secret{}, names.EnvoySecret(appName))
				expectSidecarResourcesAbsent()

				Expect(updated.Status.Phase).To(Equal(ztoperatorv1alpha1.PhaseReady))
				Expect(updated.Status.IstioAmbientConfig.Enabled).To(BeTrue())
				Expect(updated.Status.IstioAmbientConfig.EnvoyBootstrap.ConfigMapName).
					To(Equal(names.EnvoyBootstrapConfigMap(appName)))
			})
		})

		When("and auto login is enabled", func() {
			It("creates a ConfigMap holding the Envoy bootstrap configuration AND the Secret holding the OAuth credentials", func() {
				createAmbientWorkload()

				Expect(fakeClient.Create(testCtx, &corev1.Secret{
					Name: "oauth-credentials", Namespace: namespace,
					Data: map[string][]byte{
						"CLIENT_ID":     []byte("my-client-id"),
						"CLIENT_SECRET": []byte("my-client-secret"),
					},
				})).To(Succeed())

				authPolicy := &ztoperatorv1alpha1.AuthPolicy{}
				Expect(fakeClient.Get(testCtx, policyKey, authPolicy)).To(Succeed())
				authPolicy.Spec.AutoLogin = &ztoperatorv1alpha1.AutoLogin{Enabled: true}
				authPolicy.Spec.OAuthCredentials = &ztoperatorv1alpha1.OAuthCredentials{
					SecretRef:       "oauth-credentials",
					ClientIDKey:     "CLIENT_ID",
					ClientSecretKey: "CLIENT_SECRET",
				}
				Expect(fakeClient.Update(testCtx, authPolicy)).To(Succeed())

				updated := reconcileAndGetPolicy()

				expectBootstrapConfigMap(updated)

				envoySecret := &corev1.Secret{}
				Expect(fakeClient.Get(testCtx, types.NamespacedName{
					Name:      names.EnvoySecret(appName),
					Namespace: namespace,
				}, envoySecret)).To(Succeed())
				Expect(envoySecret.Data).To(HaveKey(constants.EnvoyOAuth2TokenSecretFileName))
				Expect(envoySecret.Data).To(HaveKey(constants.EnvoyOAuth2HmacSecretFileName))
				Expect(metav1.IsControlledBy(envoySecret, updated)).To(BeTrue())

				expectSidecarResourcesAbsent()

				Expect(updated.Status.Phase).To(Equal(ztoperatorv1alpha1.PhaseReady))
				Expect(updated.Status.IstioAmbientConfig.Enabled).To(BeTrue())
			})
		})
	})
})
