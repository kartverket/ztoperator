package resolver

import (
	"context"
	"errors"
	"fmt"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ResolveAudiences(
	ctx context.Context,
	k8sClient client.Client,
	namespace string,
	allowedAudiences []ztoperatorv1alpha1.AllowedAudience,
) (*[]string, error) {
	var resolvedAudiences []string

	for _, audience := range allowedAudiences {
		if audience.Value != nil && audience.ValueFrom != nil {
			return nil, errors.New("cannot define an audience as both string and ConfigMap/Secret ref")
		}
		if audience.Value != nil {
			if *audience.Value == "" {
				return nil, errors.New("audience value cannot be empty")
			}
			resolvedAudiences = append(resolvedAudiences, *audience.Value)
		} else if audience.ValueFrom != nil {
			resolvedAudienceRef, resolvedAudienceRefErr := resolveAudienceRef(
				ctx,
				k8sClient,
				namespace,
				*audience.ValueFrom,
			)
			if resolvedAudienceRefErr != nil {
				return nil, fmt.Errorf("failed to resolve audience reference: %w", resolvedAudienceRefErr)
			}
			resolvedAudiences = append(resolvedAudiences, *resolvedAudienceRef)
		}
	}
	return &resolvedAudiences, nil
}

func resolveAudienceRef(
	ctx context.Context,
	k8sClient client.Client,
	namespace string,
	valueFrom ztoperatorv1alpha1.ValueFrom,
) (*string, error) {
	if valueFrom.ConfigMapKeyRef != nil && valueFrom.SecretKeyRef != nil {
		return nil, errors.New("cannot get value from both ConfigMap and Secret")
	}
	if valueFrom.ConfigMapKeyRef != nil {
		configMap, err := helperfunctions.GetConfigMap(ctx, k8sClient, types.NamespacedName{
			Namespace: namespace,
			Name:      valueFrom.ConfigMapKeyRef.Name,
		})

		if err != nil {
			return nil, fmt.Errorf("configmap %s/%s was not found", namespace, valueFrom.ConfigMapKeyRef.Name)
		}

		value := configMap.Data[valueFrom.ConfigMapKeyRef.Key]
		if value == "" {
			return nil, fmt.Errorf(
				"audience value from configmap %s/%s key %s is empty or missing",
				namespace,
				valueFrom.ConfigMapKeyRef.Name,
				valueFrom.ConfigMapKeyRef.Key,
			)
		}

		return helperfunctions.Ptr(value), nil
	}
	if valueFrom.SecretKeyRef == nil {
		return nil, errors.New("both configMapKeyRef and secretKeyRef cannot be nil")
	}

	secret, err := helperfunctions.GetSecret(ctx, k8sClient, types.NamespacedName{
		Namespace: namespace,
		Name:      valueFrom.SecretKeyRef.Name,
	})

	if err != nil {
		return nil, fmt.Errorf("secret %s/%s was not found", namespace, valueFrom.SecretKeyRef.Name)
	}

	value := string(secret.Data[valueFrom.SecretKeyRef.Key])
	if value == "" {
		return nil, fmt.Errorf(
			"audience value from secret %s/%s key %s is empty or missing",
			namespace,
			valueFrom.SecretKeyRef.Name,
			valueFrom.SecretKeyRef.Key,
		)
	}

	return helperfunctions.Ptr(value), nil
}
