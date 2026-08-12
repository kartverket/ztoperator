package v1alpha1_test

import (
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getValidAuthPolicy(namespace, name string) *ztoperatorv1alpha1.AuthPolicy {
	return &ztoperatorv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ztoperatorv1alpha1.AuthPolicySpec{
			Enabled:      true,
			WellKnownURI: "http://mock-oauth2.auth:8080/entraid/.well-known/openid-configuration",
			Selector: ztoperatorv1alpha1.WorkloadSelector{
				MatchLabels: map[string]string{"app": "application"},
			},
		},
	}
}

func stringPtr(value string) *string {
	return &value
}
