package ignore_test

import (
	"testing"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/model"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/authorizationpolicy/ignore"
	"github.com/stretchr/testify/require"
	istiov1beta1 "istio.io/api/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetDesiredReturnsNilWhenDisabledOrInvalid(t *testing.T) {
	for name, scope := range map[string]*model.Scope{
		"disabled":      {AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{Enabled: false}}},
		"invalid":       {AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{Enabled: true}}, InvalidConfig: true},
		"without rules": {AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{Enabled: true}}},
	} {
		t.Run(name, func(t *testing.T) {
			require.Nil(t, ignore.GetDesired(scope, metav1.ObjectMeta{Name: "ignore", Namespace: "default"}))
		})
	}
}

func TestGetDesiredBuildsAllowPolicyFromIgnoreRules(t *testing.T) {
	scope := &model.Scope{AuthPolicy: v1alpha1.AuthPolicy{Spec: v1alpha1.AuthPolicySpec{
		Enabled:  true,
		Selector: v1alpha1.WorkloadSelector{MatchLabels: map[string]string{"app": "demo"}},
		IgnoreAuthRules: &[]v1alpha1.RequestMatcher{{
			Paths: []string{"/public/{*}"}, Methods: []string{"GET"},
		}},
	}}}

	result := ignore.GetDesired(scope, metav1.ObjectMeta{Name: "ignore", Namespace: "default"})

	require.NotNil(t, result)
	require.Equal(t, "ignore", result.Name)
	require.Equal(t, map[string]string{"app": "demo"}, result.Spec.Selector.MatchLabels)
	require.Equal(t, istiov1beta1.AuthorizationPolicy_ALLOW, result.Spec.Action)
	require.Len(t, result.Spec.Rules, 1)
	require.Equal(t, []string{"/public/{*}"}, result.Spec.Rules[0].To[0].Operation.Paths)
	require.Equal(t, []string{"GET"}, result.Spec.Rules[0].To[0].Operation.Methods)
}
