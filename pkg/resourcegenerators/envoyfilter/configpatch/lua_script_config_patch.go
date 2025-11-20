package configpatch

import (
	"fmt"

	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/resourcegenerators/configmap"
)

const (
	LuaScriptDirectory = "/etc/envoy/lua"
)

func GetLuaScriptConfigPatch(scope state.Scope) map[string]interface{} {
	return map[string]interface{}{
		"name": "envoy.filters.http.lua",
		"typed_config": map[string]interface{}{
			"@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua",
			"default_source_code": getLuaScriptSourceCode(
				scope.AutoLoginConfig.LuaScriptConfig.InjectLuaScriptAsInlineCode,
				scope.AutoLoginConfig.LuaScriptConfig.LuaScript,
			),
		},
	}
}

func getLuaScriptSourceCode(injectAsInlineCode bool, luaScript string) map[string]interface{} {
	if injectAsInlineCode {
		return map[string]interface{}{
			"inline_string": luaScript,
		}
	}
	return map[string]interface{}{
		"filename": fmt.Sprintf("%s/%s", LuaScriptDirectory, configmap.LuaScriptFileName),
	}
}
