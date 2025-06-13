package secret

import (
	"fmt"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func GetDesired(scope *state.Scope, objectMeta metav1.ObjectMeta) *v1.Secret {
	if !scope.AuthPolicy.Spec.Enabled || scope.AuthPolicy.Spec.AutoLogin == nil || !scope.AuthPolicy.Spec.AutoLogin.Enabled || scope.InvalidConfig {
		return nil
	}

	envoySecret, err := getEnvoySecret(objectMeta, *scope.OAuthCredentials.ClientSecret)
	if err != nil {
		return nil
	}
	return envoySecret
}

func getEnvoySecret(objectMeta metav1.ObjectMeta, clientSecret string) (*v1.Secret, error) {
	secretData := map[string][]byte{}

	tokenSecretDataValue, err := getEnvoySecretDataValue("token", clientSecret, "inline_string")
	if err != nil {
		return nil, err
	}
	secretData["token-secret.yaml"] = *tokenSecretDataValue

	hmacSecret, err := utils.GenerateHMACSecret(32)
	if err != nil {
		return nil, err
	}
	hmacSecretDataValue, err := getEnvoySecretDataValue("hmac", *hmacSecret, "inline_bytes")
	if err != nil {
		return nil, err
	}
	secretData["hmac-secret.yaml"] = *hmacSecretDataValue

	return &v1.Secret{
		ObjectMeta: objectMeta,
		Type:       v1.SecretTypeOpaque,
		Data:       secretData,
	}, nil
}

func getEnvoySecretDataValue(resourceName string, secret string, secretType string) (*[]byte, error) {
	data := map[string]interface{}{
		"resources": []map[string]interface{}{
			{
				"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.Secret",
				"name":  resourceName,
				"generic_secret": map[string]interface{}{
					"secret": map[string]string{
						secretType: secret,
					},
				},
			},
		},
	}
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal yaml: %w", err)
	}
	return &yamlData, nil
}
