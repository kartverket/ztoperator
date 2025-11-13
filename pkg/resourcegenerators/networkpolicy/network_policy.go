package networkpolicy

import (
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/utilities"
	v3 "k8s.io/api/core/v1"
	v2 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	TokenServicePort = 8080
	IstioSidecarPort = 15020
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v2.NetworkPolicy {
	if scope.IsMisconfigured() ||
		scope.OAuthCredentials.ClientAuthMethod != state.PrivateKeyJWT ||
		scope.AppLabel == nil {
		return nil
	}

	return &v2.NetworkPolicy{
		ObjectMeta: objectMeta,
		Spec: v2.NetworkPolicySpec{
			PodSelector: v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": *scope.AppLabel,
				},
			},
			PolicyTypes: []v2.PolicyType{
				v2.PolicyTypeEgress,
			},
			Egress: []v2.NetworkPolicyEgressRule{
				{
					Ports: []v2.NetworkPolicyPort{
						{
							Port:     &intstr.IntOrString{IntVal: TokenServicePort},
							Protocol: utilities.Ptr(v3.ProtocolTCP),
						},
						{
							Port:     &intstr.IntOrString{IntVal: IstioSidecarPort},
							Protocol: utilities.Ptr(v3.ProtocolTCP),
						},
					},
					To: []v2.NetworkPolicyPeer{
						{
							NamespaceSelector: &v1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": scope.AuthPolicy.Namespace,
								},
							},
							PodSelector: &v1.LabelSelector{
								MatchLabels: map[string]string{
									"app": scope.AutoLoginConfig.TokenProxyServiceName,
								},
							},
						},
					},
				},
			},
		},
	}
}
