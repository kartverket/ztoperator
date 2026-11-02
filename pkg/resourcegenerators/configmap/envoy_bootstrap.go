package configmap

import (
	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	"github.com/kartverket/ztoperator/pkg/constants"
	"google.golang.org/protobuf/encoding/protojson"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(envoyBootstrap *bootstrapv3.Bootstrap, objectMeta v1.ObjectMeta) *corev1.ConfigMap {
	bootstrapJSON, err := protojson.MarshalOptions{Indent: "  "}.Marshal(envoyBootstrap)
	if err != nil {
		panic("failed to marshal envoy bootstrap config to JSON: " + err.Error())
	}

	return &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		Data:       map[string]string{constants.EnvoyBootstrapConfigMapKey: string(bootstrapJSON)},
	}
}
