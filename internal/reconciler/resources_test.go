package reconciler_test

import (
	"fmt"
	"time"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/names"
	"github.com/kartverket/ztoperator/internal/reconciler"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/labels"
	"github.com/kartverket/ztoperator/pkg/reconciliation"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/envoyfilter/configpatch"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/secret"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioapinetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istioapisecurityv1 "istio.io/api/security/v1"
	istioapisecurityv1beta1 "istio.io/api/security/v1beta1"
	istioapitypev1beta1 "istio.io/api/type/v1beta1"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("ControllerResources", func() {
	const authPolicyName = "test-app"

	scopeFor := func(namespace string) *state.Scope {
		return &state.Scope{
			AuthPolicy: ztoperatorv1alpha1.AuthPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      authPolicyName,
					Namespace: namespace,
				},
			},
			AutoLoginConfig: state.AutoLoginConfig{
				EnvoySecretName: names.EnvoySecret(authPolicyName),
			},
		}
	}

	It("returns resources with correct kinds", func() {
		resources := reconciler.ControllerResources(scopeFor("some-namespace"))
		kinds := make([]string, len(resources))
		for i, r := range resources {
			kinds[i] = r.GetResourceKind()
		}
		Expect(kinds).To(
			ConsistOf(
				"Secret",
				"EnvoyFilter",
				"RequestAuthentication",
				"AuthorizationPolicy",
				"AuthorizationPolicy",
				"AuthorizationPolicy",
			),
		)
	})

	It("returns resources with correct names", func() {
		resources := reconciler.ControllerResources(scopeFor("some-namespace"))

		resourceKindsAndNames := make([]string, len(resources))
		for i, r := range resources {
			resourceKindsAndNames[i] = fmt.Sprintf("%s/%s", r.GetResourceKind(), r.GetResourceName())
		}

		Expect(resourceKindsAndNames).To(
			ConsistOf(
				fmt.Sprintf("%s/%s", "Secret", names.EnvoySecret(authPolicyName)),
				fmt.Sprintf("%s/%s", "EnvoyFilter", names.EnvoyFilter(authPolicyName)),
				fmt.Sprintf("%s/%s", "RequestAuthentication", authPolicyName),
				fmt.Sprintf("%s/%s", "AuthorizationPolicy", names.DenyPolicy(authPolicyName)),
				fmt.Sprintf("%s/%s", "AuthorizationPolicy", names.IgnorePolicy(authPolicyName)),
				fmt.Sprintf("%s/%s", "AuthorizationPolicy", names.RequirePolicy(authPolicyName)),
			),
		)
	})
})

var _ = Describe("auto-login Secret reconciliation", func() {
	var (
		testNamespace string
		scope         *state.Scope
	)

	// buildSecretAdapter wires up the auto-login Secret adapter exactly like the production factory does: the desired
	// state comes from the real generator and ownership is enforced by the shared reconciliation engine.
	buildSecretAdapter := func() reconciler.ControllerResourceAdapter[*corev1.Secret] {
		objectMeta := metav1.ObjectMeta{
			Name:      scope.AutoLoginConfig.EnvoySecretName,
			Namespace: scope.AuthPolicy.Namespace,
			Labels:    labels.AuthPolicyStandardLabels(),
		}
		desired := secret.GetDesired(scope, objectMeta)
		Expect(desired).NotTo(BeNil())

		return reconciler.ControllerResourceAdapter[*corev1.Secret]{
			ReconcilerAdapter: reconciliation.ReconcilerAdapter[*corev1.Secret]{
				Func: reconciliation.ResourceReconciler[*corev1.Secret]{
					ResourceKind:    "Secret",
					ResourceName:    objectMeta.Name,
					DesiredResource: &desired,
					Scope:           scope,
					ShouldUpdate:    reconciler.SecretShouldUpdate,
					UpdateFields:    reconciler.SecretUpdateFields,
				},
			},
		}
	}

	BeforeEach(func() {
		testNamespace = fmt.Sprintf("test-secret-resources-%d", time.Now().UnixNano())

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())

		scope = &state.Scope{
			AuthPolicy: ztoperatorv1alpha1.AuthPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: testNamespace,
					UID:       "test-uid-secret",
				},
				Spec: ztoperatorv1alpha1.AuthPolicySpec{
					Enabled:   true,
					AutoLogin: &ztoperatorv1alpha1.AutoLogin{Enabled: true},
				},
			},
			OAuthCredentials: state.OAuthCredentials{
				ClientSecret: helperfunctions.Ptr("super-secret"),
			},
			AutoLoginConfig: state.AutoLoginConfig{
				EnvoySecretName: names.EnvoySecret("test-app"),
			},
		}
	})

	AfterEach(func() {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		_ = k8sClient.Delete(ctx, ns)
	})

	It("creates a Secret when it does not exist", func() {
		adapter := buildSecretAdapter()

		_, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		createdSecret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      adapter.GetResourceName(),
			Namespace: testNamespace,
		}, createdSecret)).To(Succeed())
		Expect(createdSecret.Data).To(HaveKey(configpatch.TokenSecretFileName))
		Expect(createdSecret.Labels).To(SatisfyAll(
			HaveKeyWithValue(labels.ManagedByLabelKey, labels.ManagedByLabelValue),
			HaveKeyWithValue(labels.ControllerLabelKey, labels.AuthPolicyControllerLabelValue),
		))
		Expect(metav1.IsControlledBy(createdSecret, &scope.AuthPolicy)).To(BeTrue())
	})

	It("does NOT update a Secret when it needs updating but is not owned by AuthPolicy", func() {
		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      scope.AutoLoginConfig.EnvoySecretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{"placeholder": []byte("value")},
			Type: corev1.SecretTypeOpaque,
		}
		Expect(k8sClient.Create(ctx, existing)).To(Succeed())

		adapter := buildSecretAdapter()

		_, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cannot update"))
		Expect(err.Error()).To(ContainSubstring("as it is not owned by AuthPolicy"))

		updated := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      scope.AutoLoginConfig.EnvoySecretName,
			Namespace: testNamespace,
		}, updated)).To(Succeed())
		Expect(updated.Data).ToNot(HaveKey(configpatch.TokenSecretFileName))
	})

	It("updates a Secret when it needs updating and is owned by AuthPolicy", func() {
		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      scope.AutoLoginConfig.EnvoySecretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{"placeholder": []byte("value")},
			Type: corev1.SecretTypeOpaque,
		}
		Expect(ctrl.SetControllerReference(&scope.AuthPolicy, existing, scheme.Scheme)).To(Succeed())
		Expect(k8sClient.Create(ctx, existing)).To(Succeed())

		adapter := buildSecretAdapter()

		_, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		updated := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      scope.AutoLoginConfig.EnvoySecretName,
			Namespace: testNamespace,
		}, updated)).To(Succeed())
		Expect(updated.Data).To(HaveKey(configpatch.TokenSecretFileName))
	})

	It("does not update a Secret when the OAuth client secret is unchanged", func() {
		Expect(buildSecretAdapter().Reconcile(ctx, k8sClient, scheme.Scheme)).Error().NotTo(HaveOccurred())

		before := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      scope.AutoLoginConfig.EnvoySecretName,
			Namespace: testNamespace,
		}, before)).To(Succeed())
		rvBefore := before.ResourceVersion

		// A fresh adapter regenerates the desired Secret (including a new random HMAC key), but the OAuth client
		// secret is unchanged, so no update should be performed.
		Expect(buildSecretAdapter().Reconcile(ctx, k8sClient, scheme.Scheme)).Error().NotTo(HaveOccurred())

		after := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      scope.AutoLoginConfig.EnvoySecretName,
			Namespace: testNamespace,
		}, after)).To(Succeed())
		Expect(after.ResourceVersion).To(Equal(rvBefore))
	})
})

var _ = Describe("SecretShouldUpdate", func() {
	secretWithToken := func(token string, lbls map[string]string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Labels: lbls},
			Data:       map[string][]byte{configpatch.TokenSecretFileName: []byte(token)},
		}
	}

	It("returns false when the token secret and labels are unchanged", func() {
		current := secretWithToken("same", labels.AuthPolicyStandardLabels())
		desired := secretWithToken("same", labels.AuthPolicyStandardLabels())
		Expect(reconciler.SecretShouldUpdate(current, desired)).To(BeFalse())
	})

	It("returns true when the token secret value differs", func() {
		current := secretWithToken("old", labels.AuthPolicyStandardLabels())
		desired := secretWithToken("new", labels.AuthPolicyStandardLabels())
		Expect(reconciler.SecretShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when the current Secret is missing the token key", func() {
		current := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: labels.AuthPolicyStandardLabels()}}
		desired := secretWithToken("token", labels.AuthPolicyStandardLabels())
		Expect(reconciler.SecretShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when a desired label is missing on current", func() {
		current := secretWithToken("same", nil)
		desired := secretWithToken("same", labels.AuthPolicyStandardLabels())
		Expect(reconciler.SecretShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns false when desired labels are a subset of current labels", func() {
		currentLabels := labels.AuthPolicyStandardLabels()
		currentLabels["custom.example.com/team"] = "platform"
		current := secretWithToken("same", currentLabels)
		desired := secretWithToken("same", labels.AuthPolicyStandardLabels())
		Expect(reconciler.SecretShouldUpdate(current, desired)).To(BeFalse())
	})
})

var _ = Describe("EnvoyFilterShouldUpdate", func() {
	It("returns false when current and desired are equal", func() {
		Expect(reconciler.EnvoyFilterShouldUpdate(&v1alpha4.EnvoyFilter{}, &v1alpha4.EnvoyFilter{})).To(BeFalse())
	})

	It("returns true when the workload selector differs", func() {
		current := &v1alpha4.EnvoyFilter{}
		desired := &v1alpha4.EnvoyFilter{
			Spec: istioapinetworkingv1alpha3.EnvoyFilter{
				WorkloadSelector: &istioapinetworkingv1alpha3.WorkloadSelector{
					Labels: map[string]string{"app": "test"},
				},
			},
		}
		Expect(reconciler.EnvoyFilterShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when the config patches differ", func() {
		current := &v1alpha4.EnvoyFilter{}
		desired := &v1alpha4.EnvoyFilter{
			Spec: istioapinetworkingv1alpha3.EnvoyFilter{
				ConfigPatches: []*istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
					{ApplyTo: istioapinetworkingv1alpha3.EnvoyFilter_HTTP_FILTER},
				},
			},
		}
		Expect(reconciler.EnvoyFilterShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when a desired label is missing on current", func() {
		current := &v1alpha4.EnvoyFilter{}
		desired := &v1alpha4.EnvoyFilter{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labels.ManagedByLabelKey: labels.ManagedByLabelValue}},
		}
		Expect(reconciler.EnvoyFilterShouldUpdate(current, desired)).To(BeTrue())
	})
})

var _ = Describe("RequestAuthenticationShouldUpdate", func() {
	It("returns false when current and desired are equal", func() {
		Expect(reconciler.RequestAuthenticationShouldUpdate(
			&istioclientsecurityv1.RequestAuthentication{},
			&istioclientsecurityv1.RequestAuthentication{},
		)).To(BeFalse())
	})

	It("returns true when the selector differs", func() {
		current := &istioclientsecurityv1.RequestAuthentication{}
		desired := &istioclientsecurityv1.RequestAuthentication{
			Spec: istioapisecurityv1.RequestAuthentication{
				Selector: &istioapitypev1beta1.WorkloadSelector{MatchLabels: map[string]string{"app": "test"}},
			},
		}
		Expect(reconciler.RequestAuthenticationShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when the JWT rules differ", func() {
		current := &istioclientsecurityv1.RequestAuthentication{}
		desired := &istioclientsecurityv1.RequestAuthentication{
			Spec: istioapisecurityv1.RequestAuthentication{
				JwtRules: []*istioapisecurityv1.JWTRule{{Issuer: "https://issuer.example.com"}},
			},
		}
		Expect(reconciler.RequestAuthenticationShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when a desired label is missing on current", func() {
		current := &istioclientsecurityv1.RequestAuthentication{}
		desired := &istioclientsecurityv1.RequestAuthentication{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labels.ManagedByLabelKey: labels.ManagedByLabelValue}},
		}
		Expect(reconciler.RequestAuthenticationShouldUpdate(current, desired)).To(BeTrue())
	})
})

var _ = Describe("AuthorizationPolicyShouldUpdate", func() {
	It("returns false when current and desired are equal", func() {
		Expect(reconciler.AuthorizationPolicyShouldUpdate(
			&istioclientsecurityv1.AuthorizationPolicy{},
			&istioclientsecurityv1.AuthorizationPolicy{},
		)).To(BeFalse())
	})

	It("returns true when the selector differs", func() {
		current := &istioclientsecurityv1.AuthorizationPolicy{}
		desired := &istioclientsecurityv1.AuthorizationPolicy{
			Spec: istioapisecurityv1beta1.AuthorizationPolicy{
				Selector: &istioapitypev1beta1.WorkloadSelector{MatchLabels: map[string]string{"app": "test"}},
			},
		}
		Expect(reconciler.AuthorizationPolicyShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when the rules differ", func() {
		current := &istioclientsecurityv1.AuthorizationPolicy{}
		desired := &istioclientsecurityv1.AuthorizationPolicy{
			Spec: istioapisecurityv1beta1.AuthorizationPolicy{
				Rules: []*istioapisecurityv1beta1.Rule{{}},
			},
		}
		Expect(reconciler.AuthorizationPolicyShouldUpdate(current, desired)).To(BeTrue())
	})

	It("returns true when a desired label is missing on current", func() {
		current := &istioclientsecurityv1.AuthorizationPolicy{}
		desired := &istioclientsecurityv1.AuthorizationPolicy{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labels.ManagedByLabelKey: labels.ManagedByLabelValue}},
		}
		Expect(reconciler.AuthorizationPolicyShouldUpdate(current, desired)).To(BeTrue())
	})
})

var _ = Describe("SecretUpdateFields", func() {
	It("copies the data and labels from desired to current and leaves the type untouched", func() {
		current := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"stale": "label"}},
			Data:       map[string][]byte{"old": []byte("value")},
			Type:       corev1.SecretTypeOpaque,
		}
		desired := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Labels: labels.AuthPolicyStandardLabels()},
			Data:       map[string][]byte{"new": []byte("value")},
		}

		reconciler.SecretUpdateFields(current, desired)

		Expect(current.Data).To(Equal(desired.Data))
		Expect(current.Labels).To(Equal(desired.Labels))
		// Type is intentionally not managed by the update.
		Expect(current.Type).To(Equal(corev1.SecretTypeOpaque))
	})
})

var _ = Describe("EnvoyFilterUpdateFields", func() {
	It("copies the workload selector, config patches and labels from desired to current", func() {
		current := &v1alpha4.EnvoyFilter{}
		desired := &v1alpha4.EnvoyFilter{
			ObjectMeta: metav1.ObjectMeta{Labels: labels.AuthPolicyStandardLabels()},
			Spec: istioapinetworkingv1alpha3.EnvoyFilter{
				WorkloadSelector: &istioapinetworkingv1alpha3.WorkloadSelector{
					Labels: map[string]string{"app": "test"},
				},
				ConfigPatches: []*istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
					{ApplyTo: istioapinetworkingv1alpha3.EnvoyFilter_HTTP_FILTER},
				},
			},
		}

		reconciler.EnvoyFilterUpdateFields(current, desired)

		Expect(current.Spec.GetWorkloadSelector()).To(Equal(desired.Spec.GetWorkloadSelector()))
		Expect(current.Spec.GetConfigPatches()).To(Equal(desired.Spec.GetConfigPatches()))
		Expect(current.Labels).To(Equal(desired.Labels))
	})
})

var _ = Describe("RequestAuthenticationUpdateFields", func() {
	It("copies the selector, JWT rules and labels from desired to current", func() {
		current := &istioclientsecurityv1.RequestAuthentication{}
		desired := &istioclientsecurityv1.RequestAuthentication{
			ObjectMeta: metav1.ObjectMeta{Labels: labels.AuthPolicyStandardLabels()},
			Spec: istioapisecurityv1.RequestAuthentication{
				Selector: &istioapitypev1beta1.WorkloadSelector{MatchLabels: map[string]string{"app": "test"}},
				JwtRules: []*istioapisecurityv1.JWTRule{{Issuer: "https://issuer.example.com"}},
			},
		}

		reconciler.RequestAuthenticationUpdateFields(current, desired)

		Expect(current.Spec.GetSelector()).To(Equal(desired.Spec.GetSelector()))
		Expect(current.Spec.GetJwtRules()).To(Equal(desired.Spec.GetJwtRules()))
		Expect(current.Labels).To(Equal(desired.Labels))
	})
})

var _ = Describe("AuthorizationPolicyUpdateFields", func() {
	It("copies the selector, rules and labels from desired to current", func() {
		current := &istioclientsecurityv1.AuthorizationPolicy{}
		desired := &istioclientsecurityv1.AuthorizationPolicy{
			ObjectMeta: metav1.ObjectMeta{Labels: labels.AuthPolicyStandardLabels()},
			Spec: istioapisecurityv1beta1.AuthorizationPolicy{
				Selector: &istioapitypev1beta1.WorkloadSelector{MatchLabels: map[string]string{"app": "test"}},
				Rules:    []*istioapisecurityv1beta1.Rule{{}},
			},
		}

		reconciler.AuthorizationPolicyUpdateFields(current, desired)

		Expect(current.Spec.GetSelector()).To(Equal(desired.Spec.GetSelector()))
		Expect(current.Spec.GetRules()).To(Equal(desired.Spec.GetRules()))
		Expect(current.Labels).To(Equal(desired.Labels))
	})
})
