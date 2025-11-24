package configpatch

import (
	"github.com/kartverket/ztoperator/internal/state"
)

func GetLuaScriptConfigPatch(scope state.Scope) map[string]interface{} {
	return map[string]interface{}{
		"name": "envoy.filters.http.lua",
		"typed_config": map[string]interface{}{
			"@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua",
			"default_source_code": map[string]interface{}{
				"inline_string": scope.AutoLoginConfig.LuaScriptConfig.LuaScript,
			},
		},
	}
}
