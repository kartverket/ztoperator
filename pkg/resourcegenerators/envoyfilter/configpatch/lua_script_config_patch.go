package configpatch

func GetLuaScriptFilterConfig(luaScript string) map[string]any {
	return map[string]any{
		"name": "envoy.filters.http.lua",
		"typed_config": map[string]any{
			"@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua",
			"default_source_code": map[string]any{
				"inline_string": luaScript,
			},
		},
	}
}
