package require_test

import (
	"testing"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/model"
	requiregen "github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy/require"
	"github.com/stretchr/testify/require"
	istiov1beta1 "istio.io/api/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetDesiredReturnsNilWhenDisabledOrInvalid(t *testing.T) {
	for name, scope := range map[string]*model.Scope{
		"disabled": {AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{Enabled: false}}},
		"invalid":  {AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{Enabled: true}}, InvalidConfig: true},
	} {
		t.Run(name, func(t *testing.T) {
			require.Nil(t, requiregen.GetDesired(scope, metav1.ObjectMeta{Name: "require", Namespace: "default"}))
		})
	}
}

func TestGetDesiredBuildsAllowAllPolicyWithoutSpecificRules(t *testing.T) {
	scope := &model.Scope{
		AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{
			Enabled:  true,
			Selector: v1alpha1.WorkloadSelector{MatchLabels: map[string]string{"app": "demo"}},
		}},
		Audiences:            []string{"api://demo"},
		IdentityProviderUris: model.IdentityProviderUris{IssuerURI: "https://issuer.example.com"},
	}

	result := requiregen.GetDesired(scope, metav1.ObjectMeta{Name: "require", Namespace: "default"})

	require.NotNil(t, result)
	require.Equal(t, istiov1beta1.AuthorizationPolicy_ALLOW, result.Spec.Action)
	require.Len(t, result.Spec.Rules, 1)
	require.Equal(t, []string{"*"}, result.Spec.Rules[0].To[0].Operation.Paths)
	require.Len(t, result.Spec.Rules[0].When, 2)
}

func TestGetDesiredBuildsSpecificAndUnspecifiedRules(t *testing.T) {
	authRules := []v1alpha1.RequestAuthRule{
		{
			RequestMatcher: v1alpha1.RequestMatcher{
				Paths: []string{"/admin"}, Methods: []string{"GET"},
			},
			When: &[]v1alpha1.Condition{{Claim: "role", Values: []string{"admin"}}},
		},
	}
	scope := &model.Scope{
		AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{
			Enabled:         true,
			AuthRules:       &authRules,
			IgnoreAuthRules: &[]v1alpha1.RequestMatcher{{Paths: []string{"/public"}}},
		}},
		IdentityProviderUris: model.IdentityProviderUris{IssuerURI: "https://issuer.example.com"},
	}

	result := requiregen.GetDesired(scope, metav1.ObjectMeta{Name: "require", Namespace: "default"})

	require.NotNil(t, result)
	require.Len(t, result.Spec.Rules, 2)
	require.Equal(t, []string{"*"}, result.Spec.Rules[0].To[len(result.Spec.Rules[0].To)-1].Operation.Paths)
	require.Equal(t, []string{"/admin"}, result.Spec.Rules[1].To[0].Operation.Paths)
	require.Equal(t, "request.auth.claims[role]", result.Spec.Rules[1].When[1].Key)
}
