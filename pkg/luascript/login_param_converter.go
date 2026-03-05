package luascript

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// ConvertLoginParamsToLuaParams encodes a map of OAuth login parameters into a
// Lua table literal string suitable for embedding in the generated EnvoyFilter
// Lua script. Keys are emitted in sorted order for deterministic output. Values
// are URL-encoded with [url.QueryEscape] before embedding.
//
// An empty or nil map returns an empty Lua table.
//
// Example:
//
//	ConvertLoginParamsToLuaParams(map[string]string{
//	    "acr_values": "idporten-loa-high",
//	    "ui_locales": "nb",
//	})
//	→ {["acr_values"]="idporten-loa-high",["ui_locales"]="nb"}
func ConvertLoginParamsToLuaParams(rawLoginParams map[string]string) string {
	params := encodeLoginParams(rawLoginParams)

	if len(params) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for _, k := range keys {
		v := params[k]
		if !first {
			sb.WriteString(",")
		}
		first = false
		fmt.Fprintf(&sb, `["%s"]="%s"`, k, v)
	}
	sb.WriteString("}")
	return sb.String()
}

func encodeLoginParams(raw map[string]string) map[string]string {
	encoded := make(map[string]string, len(raw))
	for k, v := range raw {
		encoded[k] = url.QueryEscape(v)
	}
	return encoded
}
