package luascript

import _ "embed"

//go:embed ztoperator.lua
var luaScript string

func GetLuaScript() string {
	return luaScript
}
