package deployments

import (
	"context"

	"github.com/kartverket/ztoperator/internal/eventhandler"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func EventHandler(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		deployment, ok := obj.(*appsv1.Deployment)
		if !ok {
			return nil
		}

		return eventhandler.EnqueueAuthPoliciesInNamespace(ctx, c, deployment.Namespace)
	})
}
