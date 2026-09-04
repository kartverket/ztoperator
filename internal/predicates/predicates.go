package predicates

import (
	"bytes"
	"maps"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SpecOrLabelsChanged passes update events only when the object's spec (as signalled by a bumped
// metadata.generation) or its labels changed. Status-only writes are dropped. Create, delete and
// generic events always pass.
func SpecOrLabelsChanged() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return true
			}
			if e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() {
				return true
			}
			return !maps.Equal(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels())
		},
	}
}
