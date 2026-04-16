package v1

import (
	"context"
	"fmt"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetAuthPolicyForApplication fetches the single ready AuthPolicy for the given application.
// Returns an error if none, multiple, or an unready AuthPolicy is found.
func GetAuthPolicyForApplication(
	ctx context.Context,
	k8sClient client.Client,
	appKey client.ObjectKey,
) (*v1alpha1.AuthPolicy, error) {
	var list v1alpha1.AuthPolicyList
	podlog.Info("Fetching AuthPolicy resources", "namespacedName", appKey)
	if err := k8sClient.List(ctx, &list, client.InNamespace(appKey.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to fetch AuthPolicy resources: %w", err)
	}

	var matches []v1alpha1.AuthPolicy
	for _, sc := range list.Items {
		if sc.Spec.Selector.MatchLabels["app"] == appKey.Name {
			matches = append(matches, sc)
		}
	}

	switch len(matches) {
	case 0:
		podlog.Info("No AuthPolicy found for Application", "namespacedName", appKey)
		return nil, fmt.Errorf("no AuthPolicy resource was found for the corresponding Application")
	case 1:
		// expected
	default:
		podlog.Info("Multiple AuthPolicy found for Application", "namespacedName", appKey)
		return nil, fmt.Errorf("multiple AuthPolicy resources found for Application")
	}

	sc := &matches[0]
	if !sc.Status.Ready {
		podlog.Info("AuthPolicy is not ready", "namespacedName", appKey)
		return nil, fmt.Errorf("AuthPolicy resource for Application is not ready")
	}

	return sc, nil
}
