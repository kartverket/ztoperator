package eventhandler

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	authPolicyGroup   = "ztoperator.kartverket.no"
	authPolicyVersion = "v1alpha1"
	authPolicyKind    = "AuthPolicy"
)

// IsOwnedByAuthPolicy returns true if the object has an owner reference pointing to an AuthPolicy.
func IsOwnedByAuthPolicy(obj client.Object) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.APIVersion == authPolicyGroup+"/"+authPolicyVersion && ref.Kind == authPolicyKind {
			return true
		}
	}
	return false
}

// EnqueueAuthPoliciesInNamespace lists all AuthPolicy resources in the given namespace
// and returns a reconcile request for each one.
func EnqueueAuthPoliciesInNamespace(ctx context.Context, c client.Client, namespace string) []reconcile.Request {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   authPolicyGroup,
		Version: authPolicyVersion,
		Kind:    authPolicyKind + "List",
	})

	if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return nil
	}

	reqs := make([]reconcile.Request, 0, len(list.Items))
	for _, item := range list.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: item.GetNamespace(),
				Name:      item.GetName(),
			},
		})
	}

	return reqs
}
