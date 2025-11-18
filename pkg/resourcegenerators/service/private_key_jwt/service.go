package private_key_jwt

import (
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/utilities"
	v3 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v3.Service {
	if !scope.ShouldHaveTokenProxy() {
		return nil
	}
	return &v3.Service{
		ObjectMeta: objectMeta,
		Spec: v3.ServiceSpec{
			Type: v3.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": scope.AutoLoginConfig.TokenProxy.Name,
			},
			Ports: []v3.ServicePort{
				{
					Name:       "istio-metrics",
					Port:       utilities.IstioProxyPort,
					TargetPort: intstr.IntOrString{IntVal: utilities.IstioProxyPort},
					Protocol:   "TCP",
				},
				{
					Name:        "http",
					AppProtocol: utilities.Ptr("http"),
					Port:        utilities.TokenProxyPort,
					TargetPort:  intstr.IntOrString{IntVal: utilities.TokenProxyPort},
					Protocol:    "TCP",
				},
			},
		},
	}
}
