package configmap

import (
	"github.com/kartverket/ztoperator/internal/state"
	v2 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const LuaScriptFileName = "ztoperator.lua"

func GetDesired(scope *state.Scope, objectMeta v1.ObjectMeta) *v2.ConfigMap {
	if scope.IsMisconfigured() || scope.AuthPolicy.Spec.AutoLogin == nil ||
		!scope.AuthPolicy.Spec.AutoLogin.Enabled {
		return nil
	}

	return &v2.ConfigMap{
		ObjectMeta: objectMeta,
		Data: map[string]string{
			LuaScriptFileName: scope.AutoLoginConfig.LuaScriptConfig.LuaScript,
		},
	}
}
