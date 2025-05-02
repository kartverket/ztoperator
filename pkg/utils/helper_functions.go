package utils

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func LowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.Requeue:
		return i
	case j.Requeue:
		return j
	case i.RequeueAfter < j.RequeueAfter:
		return i
	default:
		return j
	}
}

func BuildObjectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{"type": "ztoperator.kartverket.no"},
	}
}

func GetSecret(ctx context.Context, k8sClient client.Client, objectKey client.ObjectKey) (corev1.Secret, error) {
	secretData := corev1.Secret{}

	if err := k8sClient.Get(ctx, objectKey, &secretData); err != nil {
		if apierrors.IsNotFound(err) {
			return secretData, fmt.Errorf("secret %s/%s not found", objectKey.Namespace, objectKey.Name)
		}
		return secretData, err
	}
	return secretData, nil
}
