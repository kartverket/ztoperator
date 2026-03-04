package luascript

import (
	"strings"

	"github.com/kartverket/ztoperator/api/v1alpha1"
)

// ConvertRequestMatchersToLuaTableString converts RequestMatchers into a Lua table string.
// Each path/methods pair becomes one entry of the form:
//
//	{regex="^/some%-path$",methods={["GET"]=true, ...}}
//
// An empty methods slice means all HTTP methods are permitted.
func ConvertRequestMatchersToLuaTableString(requestMatchers []v1alpha1.RequestMatcher) string {
	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for _, matcher := range requestMatchers {
		for _, path := range matcher.Paths {
			convertedPath := ConvertRequestMatcherPathToLuaPattern(path)
			if !first {
				sb.WriteString(",")
			}
			first = false

			sb.WriteString(`{regex="`)
			sb.WriteString(escapeLuaString(convertedPath))
			sb.WriteString(`",methods={`)

			if len(matcher.Methods) > 0 {
				for idx, method := range matcher.Methods {
					if idx > 0 {
						sb.WriteString(",")
					}
					sb.WriteString(`["`)
					sb.WriteString(method)
					sb.WriteString(`"]=true`)
				}
			}
			sb.WriteString("}}")
		}
	}
	sb.WriteString("}")
	return sb.String()
}

// escapeLuaString ensures any back‑slashes or quotes in the regex are safe for Lua source.
func escapeLuaString(s string) string {
	if s == "/" {
		return "^/$"
	}
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
