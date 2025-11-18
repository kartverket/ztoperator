package private_key_jwt

import (
	"github.com/kartverket/ztoperator/internal/state"
	v3 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v3.ServiceAccount {
	if !scope.ShouldHaveTokenProxy() {
		return nil
	}
	return &v3.ServiceAccount{
		ObjectMeta: objectMeta,
	}
}
