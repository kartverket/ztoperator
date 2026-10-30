package deny_test

import (
	"testing"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/model"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy/deny"
	"github.com/stretchr/testify/require"
	istiov1beta1 "istio.io/api/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetDesiredReturnsNilWhenDisabledOrWithoutAuthRules(t *testing.T) {
	for name, scope := range map[string]*model.Scope{
		"disabled":      {AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{Enabled: false}}},
		"without rules": {AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{Enabled: true}}},
	} {
		t.Run(name, func(t *testing.T) {
			require.Nil(t, deny.GetDesired(scope, metav1.ObjectMeta{Name: "deny", Namespace: "default"}))
		})
	}
}

func TestGetDesiredDeniesAllPathsForInvalidConfig(t *testing.T) {
	scope := &model.Scope{
		AuthPolicy:    v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{Enabled: true}},
		InvalidConfig: true,
	}

	result := deny.GetDesired(scope, metav1.ObjectMeta{Name: "deny", Namespace: "default"})

	require.NotNil(t, result)
	require.Equal(t, istiov1beta1.AuthorizationPolicy_DENY, result.Spec.Action)
	require.Equal(t, []string{"*"}, result.Spec.Rules[0].To[0].Operation.Paths)
}

func TestGetDesiredBuildsDenyRulesFromAuthRulesAndConditions(t *testing.T) {
	authRules := []v1alpha1.RequestAuthRule{{
		RequestMatcher: v1alpha1.RequestMatcher{Paths: []string{"/admin/{*}"}, Methods: []string{"POST"}},
		When:           &[]v1alpha1.Condition{{Claim: "role", Values: []string{"admin"}}},
	}}
	scope := &model.Scope{
		AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{
			Enabled:   true,
			AuthRules: &authRules,
		}},
		Audiences:            []string{"api://demo"},
		IdentityProviderUris: model.IdentityProviderUris{IssuerURI: "https://issuer.example.com"},
	}

	result := deny.GetDesired(scope, metav1.ObjectMeta{Name: "deny", Namespace: "default"})

	require.NotNil(t, result)
	require.Equal(t, istiov1beta1.AuthorizationPolicy_DENY, result.Spec.Action)
	require.Len(t, result.Spec.Rules, 3)
	require.Equal(t, []string{"/admin/{*}"}, result.Spec.Rules[0].To[0].Operation.Paths)
	require.Equal(t, []string{"POST"}, result.Spec.Rules[0].To[0].Operation.Methods)
	require.Equal(t, "request.auth.claims[iss]", result.Spec.Rules[0].When[0].Key)
	require.Equal(t, []string{"https://issuer.example.com"}, result.Spec.Rules[0].When[0].NotValues)
	require.Equal(t, "request.auth.claims[aud]", result.Spec.Rules[1].When[0].Key)
	require.Equal(t, []string{"api://demo"}, result.Spec.Rules[1].When[0].NotValues)
	require.Equal(t, "request.auth.claims[role]", result.Spec.Rules[2].When[0].Key)
	require.Equal(t, []string{"admin"}, result.Spec.Rules[2].When[0].NotValues)
}
