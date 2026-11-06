package lua

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	luav3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/lua/v3"
)

func Lua(luaScriptAsString string) (*luav3.Lua, error) {
	lua := luav3.Lua{
		DefaultSourceCode: &corev3.DataSource{
			Specifier: &corev3.DataSource_InlineString{
				InlineString: luaScriptAsString,
			},
		},
	}
	var err error
	if validateErr := lua.ValidateAll(); validateErr != nil {
		err = fmt.Errorf("error validating Lua filter: %w", validateErr)
	}
	return &lua, err
}
