package secret

import (
	"fmt"

	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/kartverket/ztoperator/pkg/constants"
	"github.com/kartverket/ztoperator/pkg/model"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func GetDesired(scope *model.Scope, objectMeta metav1.ObjectMeta) *corev1.Secret {
	if !scope.AuthPolicy.Spec.Enabled || scope.InvalidConfig || scope.AuthPolicy.Spec.AutoLogin == nil ||
		!scope.AuthPolicy.Spec.AutoLogin.Enabled {
		return nil
	}

	hmacSecretBytes, err := marshalSdsSecret(scope.OAuthCredentials.HmacSecret.Value)
	if err != nil {
		panic(
			fmt.Sprintf(
				"failed to marshal hmac secret for AuthPolicy %s/%s: %v",
				scope.AuthPolicy.Namespace,
				scope.AuthPolicy.Name,
				err,
			),
		)
	}

	tokenSecretBytes, err := marshalSdsSecret(scope.OAuthCredentials.TokenSecret.Value)
	if err != nil {
		panic(
			fmt.Sprintf(
				"failed to marshal token secret for AuthPolicy %s/%s: %v",
				scope.AuthPolicy.Namespace,
				scope.AuthPolicy.Name,
				err,
			),
		)
	}

	return &corev1.Secret{
		ObjectMeta: objectMeta,
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			constants.EnvoyOAuth2HmacSecretFileName:  hmacSecretBytes,
			constants.EnvoyOAuth2TokenSecretFileName: tokenSecretBytes,
		},
	}
}

func marshalSdsSecret(secret *tlsv3.Secret) ([]byte, error) {
	res, err := anypb.New(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap secret in protobuf Any: %w", err)
	}

	jsonBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(
		&discoveryv3.DiscoveryResponse{Resources: []*anypb.Any{res}},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SDS response: %w", err)
	}

	return yaml.JSONToYAML(jsonBytes)
}
