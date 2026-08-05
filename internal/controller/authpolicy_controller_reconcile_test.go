package controller_test

import (
	"context"
	"testing"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/controller"
	"github.com/kartverket/ztoperator/internal/names"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1alpha4 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	securityv1 "istio.io/client-go/pkg/apis/security/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sevents "k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
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

func TestAuthPolicyReconcileHappyPathWithoutAutoLogin(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, ztoperatorv1alpha1.AddToScheme(scheme))
	require.NoError(t, v1alpha4.AddToScheme(scheme))
	require.NoError(t, securityv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	const (
		namespace = "test-controller"
		apName    = "my-app"
	)

	authPolicy := &ztoperatorv1alpha1.AuthPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AuthPolicy",
			APIVersion: "ztoperator.kartverket.no/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       apName,
			Namespace:  namespace,
			Generation: 1,
		},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			Enabled:      true,
			WellKnownURI: "https://idp.example.com/.well-known/openid-configuration",
			Selector: ztoperatorv1alpha1.WorkloadSelector{
				MatchLabels: map[string]string{"app": apName},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(authPolicy).
		WithStatusSubresource(authPolicy).
		Build()

	reconciler := &controller.AuthPolicyReconciler{
		Client:                    k8sClient,
		Scheme:                    scheme,
		Recorder:                  k8sevents.NewFakeRecorder(100),
		DiscoveryDocumentResolver: newBasicDiscoveryResolver(),
	}

	result, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: apName, Namespace: namespace},
	})
	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)
	assert.False(t, result.Requeue)
	ra := &securityv1.RequestAuthentication{}
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: apName, Namespace: namespace}, ra))
	require.Len(t, ra.Spec.GetJwtRules(), 1)
	assert.Equal(t, "https://idp.example.com", ra.Spec.GetJwtRules()[0].GetIssuer())
	assert.Equal(t, "https://idp.example.com/jwks", ra.Spec.GetJwtRules()[0].GetJwksUri())
	assert.Equal(t, map[string]string{"app": apName}, ra.Spec.GetSelector().GetMatchLabels())
	assert.True(t, metav1.IsControlledBy(ra, authPolicy))

	requirePolicy := &securityv1.AuthorizationPolicy{}
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      names.RequirePolicy(apName),
		Namespace: namespace,
	}, requirePolicy))
	assert.True(t, metav1.IsControlledBy(requirePolicy, authPolicy))

	ef := &v1alpha4.EnvoyFilter{}
	efErr := k8sClient.Get(ctx, types.NamespacedName{Name: names.EnvoyFilter(apName), Namespace: namespace}, ef)
	assert.True(t, apierrors.IsNotFound(efErr))

	envoySecret := &corev1.Secret{}
	secretErr := k8sClient.Get(ctx, types.NamespacedName{Name: names.EnvoySecret(apName), Namespace: namespace}, envoySecret)
	assert.True(t, apierrors.IsNotFound(secretErr))

	denyPolicy := &securityv1.AuthorizationPolicy{}
	denyErr := k8sClient.Get(ctx, types.NamespacedName{Name: names.DenyPolicy(apName), Namespace: namespace}, denyPolicy)
	assert.True(t, apierrors.IsNotFound(denyErr))

	ignorePolicy := &securityv1.AuthorizationPolicy{}
	ignoreErr := k8sClient.Get(ctx, types.NamespacedName{Name: names.IgnorePolicy(apName), Namespace: namespace}, ignorePolicy)
	assert.True(t, apierrors.IsNotFound(ignoreErr))

	updatedPolicy := &ztoperatorv1alpha1.AuthPolicy{}
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: apName, Namespace: namespace}, updatedPolicy))
	assert.Equal(t, ztoperatorv1alpha1.PhaseReady, updatedPolicy.Status.Phase)
	assert.True(t, updatedPolicy.Status.Ready)
	assert.Equal(t, int64(1), updatedPolicy.Status.ObservedGeneration)
}
