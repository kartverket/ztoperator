package private_key_jwt

import (
	"fmt"

	"github.com/kartverket/ztoperator/internal/state"
	"istio.io/api/security/v1beta1"
	v1beta2 "istio.io/api/type/v1beta1"
	istioclientsecurityv1 "istio.io/client-go/pkg/apis/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *istioclientsecurityv1.AuthorizationPolicy {
	if !scope.ShouldHaveTokenProxy() {
		return nil
	}

	var protectedAppPrincipals []string

	for _, serviceAccountName := range scope.AutoLoginConfig.TokenProxy.ProtectedPodsServiceAccounts {
		protectedAppPrincipals = append(
			protectedAppPrincipals,
			fmt.Sprintf(
				"cluster.local/ns/%s/sa/%s",
				scope.AuthPolicy.Namespace,
				serviceAccountName,
			),
		)
	}

	return &istioclientsecurityv1.AuthorizationPolicy{
		ObjectMeta: objectMeta,
		Spec: v1beta1.AuthorizationPolicy{
			Selector: &v1beta2.WorkloadSelector{
				MatchLabels: map[string]string{
					"app": scope.AutoLoginConfig.TokenProxy.Name,
				},
			},
			Action: v1beta1.AuthorizationPolicy_DENY,
			Rules: []*v1beta1.Rule{
				{
					From: []*v1beta1.Rule_From{
						{
							Source: &v1beta1.Source{
								NotPrincipals: protectedAppPrincipals,
							},
						},
					},
					To: []*v1beta1.Rule_To{
						{
							Operation: &v1beta1.Operation{
								Methods: []string{"POST"},
								Paths:   []string{"/token"},
							},
						},
					},
				},
			},
		},
	}
}
