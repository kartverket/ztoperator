package configmap

import (
	"context"

	"github.com/kartverket/ztoperator/internal/eventhandler"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func EventHandler(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		configMap, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil
		}

		// Owned configmaps already trigger reconcile
		if eventhandler.IsOwnedByAuthPolicy(configMap) {
			return nil
		}

		return eventhandler.EnqueueAuthPoliciesInNamespace(ctx, c, configMap.Namespace)
	})
}
