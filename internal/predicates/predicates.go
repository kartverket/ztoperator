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

// SecretContentOrLabelsChanged passes update events only when a Secret's data or labels
// changed. Create, delete and generic events always pass.
func SecretContentOrLabelsChanged() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSecret, oldOk := e.ObjectOld.(*corev1.Secret)
			newSecret, newOk := e.ObjectNew.(*corev1.Secret)
			if !oldOk || !newOk {
				return true
			}
			if !maps.Equal(oldSecret.Labels, newSecret.Labels) {
				return true
			}
			return !maps.EqualFunc(oldSecret.Data, newSecret.Data, bytes.Equal)
		},
	}
}
