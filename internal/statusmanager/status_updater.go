package statusmanager

import (
	"context"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/metrics"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateStatus updates the AuthPolicy status in Kubernetes, with retry logic and metrics updates.
func UpdateStatus(
	ctx context.Context,
	k8sClient client.Client,
	authPolicy ztoperatorv1alpha1.AuthPolicy,
) error {
	metrics.DeleteAuthPolicyInfo(types.NamespacedName{
		Name:      authPolicy.Name,
		Namespace: authPolicy.Namespace,
	})

	if err := metrics.RefreshAuthPolicyInfo(ctx, k8sClient, authPolicy); err != nil {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &ztoperatorv1alpha1.AuthPolicy{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&authPolicy), latest); err != nil {
			return err
		}
		latest.Status = authPolicy.Status
		return k8sClient.Status().Update(ctx, latest)
	})
}
