package private_key_jwt

import (
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/utilities"
	"istio.io/api/networking/v1alpha3"
	v4 "istio.io/client-go/pkg/apis/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v4.ServiceEntry {
	if !scope.ShouldHaveTokenProxy() {
		return nil
	}

	return &v4.ServiceEntry{
		ObjectMeta: objectMeta,
		Spec: v1alpha3.ServiceEntry{
			ExportTo: []string{
				".",
				utilities.IstioDataplaneNamespace,
				utilities.IstioGatewaysNamespace,
			},
			Hosts: []string{
				scope.AutoLoginConfig.TokenProxy.TokenEndpointParsedAsUrl.Hostname(),
			},
			Ports: []*v1alpha3.ServicePort{
				{
					Name:     "https",
					Number:   443,
					Protocol: "HTTPS",
				},
			},
			Resolution: v1alpha3.ServiceEntry_DNS,
		},
	}
}
