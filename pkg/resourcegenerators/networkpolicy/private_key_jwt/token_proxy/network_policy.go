package token_proxy_outbound

import (
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/utilities"
	v3 "k8s.io/api/core/v1"
	v2 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v2.NetworkPolicy {
	if !scope.ShouldHaveTokenProxy() {
		return nil
	}

	return &v2.NetworkPolicy{
		ObjectMeta: objectMeta,
		Spec: v2.NetworkPolicySpec{
			PodSelector: v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": scope.AutoLoginConfig.TokenProxy.Name,
				},
			},
			PolicyTypes: []v2.PolicyType{
				v2.PolicyTypeIngress,
			},
			Ingress: []v2.NetworkPolicyIngressRule{
				{
					Ports: []v2.NetworkPolicyPort{
						{
							Port:     &intstr.IntOrString{IntVal: utilities.TokenProxyPort},
							Protocol: utilities.Ptr(v3.ProtocolTCP),
						},
						{
							Port:     &intstr.IntOrString{IntVal: utilities.IstioProxyPort},
							Protocol: utilities.Ptr(v3.ProtocolTCP),
						},
					},
					From: []v2.NetworkPolicyPeer{
						{
							NamespaceSelector: &v1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": scope.AuthPolicy.Namespace,
								},
							},
							PodSelector: &v1.LabelSelector{
								MatchLabels: scope.AuthPolicy.Spec.Selector.MatchLabels,
							},
						},
					},
				},
			},
		},
	}
}
