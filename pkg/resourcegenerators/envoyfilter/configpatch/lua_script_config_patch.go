package configpatch

import (
	"fmt"
)

const (
	BypassOauthLoginHeaderName = "x-bypass-login"
)

func GetLuaScript() map[string]interface{} {
	luaScript := fmt.Sprintf(`function envoy_on_request(request_handle)
  local p = request_handle:headers():get(":path")
  local m = request_handle:headers():get(":method")
  if p == nil or p == "" or m == nil or m == "" then
    request_handle:headers():add("%s", "false")
  else
    request_handle:headers():add("%s", m .. ":" .. p)
  end
end
`, BypassOauthLoginHeaderName, BypassOauthLoginHeaderName)

	return map[string]interface{}{
		"name": "envoy.filters.http.lua",
		"typed_config": map[string]interface{}{
			"@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua",
			"default_source_code": map[string]interface{}{
				"inline_string": luaScript,
			},
		},
	}
}
