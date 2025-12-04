package resolver

import (
	"context"
	"fmt"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/utilities"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ResolveAudiences(ctx context.Context, k8sClient client.Client, namespace string, audiences []ztoperatorv1alpha1.Audience) (*[]string, error) {
	var resolvedAudiences []string
	for _, audience := range audiences {
		if audience.AudienceAsString != nil && audience.ValueFrom != nil {
			return nil, fmt.Errorf("cannot define an audience as both string and ConfigMap/Secret ref")
		} else if audience.AudienceAsString != nil {
			resolvedAudiences = append(resolvedAudiences, string(*audience.AudienceAsString))
		} else if audience.ValueFrom != nil {
			if audience.ValueFrom.ConfigMapKeyRef != nil && audience.ValueFrom.SecretKeyRef != nil {
				return nil, fmt.Errorf("cannot get value from both ConfigMap and Secret")
			} else if audience.ValueFrom.ConfigMapKeyRef != nil {
				configMap, err := utilities.GetConfigMap(ctx, k8sClient, types.NamespacedName{
					Namespace: namespace,
					Name:      audience.ValueFrom.ConfigMapKeyRef.Name,
				})

				if err != nil {
					return nil, fmt.Errorf("configmap %s/%s was not found", namespace, audience.ValueFrom.ConfigMapKeyRef.Name)
				}

				resolvedAudiences = append(resolvedAudiences, configMap.Data[audience.ValueFrom.ConfigMapKeyRef.Key])
			} else if audience.ValueFrom.SecretKeyRef != nil {
				secret, err := utilities.GetSecret(ctx, k8sClient, types.NamespacedName{
					Namespace: namespace,
					Name:      audience.ValueFrom.SecretKeyRef.Name,
				})

				if err != nil {
					return nil, fmt.Errorf("secret %s/%s was not found", namespace, audience.ValueFrom.SecretKeyRef.Name)
				}

				resolvedAudiences = append(resolvedAudiences, string(secret.Data[audience.ValueFrom.SecretKeyRef.Key]))
			}
		}
	}
	return &resolvedAudiences, nil
}
