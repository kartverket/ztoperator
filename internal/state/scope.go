package state

import (
	"fmt"
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Scope struct {
	AuthPolicy                 *ztoperatorv1alpha1.AuthPolicy
	Descendants                []Descendant[client.Object]
	HasValidPaths              bool
	PathValidationErrorMessage *string
}

type Descendant[T client.Object] struct {
	ID             string
	Object         T
	ErrorMessage   *string
	SuccessMessage *string
}

func (s *Scope) GetErrors() []string {
	var errs []string
	if s != nil {
		for _, d := range s.Descendants {
			if d.ErrorMessage != nil {
				errs = append(errs, *d.ErrorMessage)
			}
		}
	}
	return errs
}

func (s *Scope) ReplaceDescendant(obj client.Object, errorMessage *string, successMessage *string, resourceKind, resourceName string) {
	if s != nil {
		for i, d := range s.Descendants {
			if reflect.TypeOf(d) == reflect.TypeOf(obj) && d.ID == obj.GetName() {
				s.Descendants[i] = Descendant[client.Object]{
					Object:         obj,
					ErrorMessage:   errorMessage,
					SuccessMessage: successMessage,
				}
				return
			}
		}
		s.Descendants = append(s.Descendants, Descendant[client.Object]{
			ID:             GetID(resourceKind, resourceName),
			Object:         obj,
			ErrorMessage:   errorMessage,
			SuccessMessage: successMessage,
		})
	}
}

func GetID(resourceKind, resourceName string) string {
	return fmt.Sprintf("%s-%s", resourceKind, resourceName)
}
