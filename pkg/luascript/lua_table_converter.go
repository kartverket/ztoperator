package luascript

import "github.com/kartverket/ztoperator/api/v1alpha1"

// BuildLuaRulesFromMatchers converts raw RequestMatchers into a Lua table
// string by first converting paths to Lua patterns and then building the rules.
func BuildLuaRulesFromMatchers(matchers []v1alpha1.RequestMatcher) string {
	return BuildLuaRules(convertToLuaPatterns(matchers))
}

func convertToLuaPatterns(requestMatchers []v1alpha1.RequestMatcher) []v1alpha1.RequestMatcher {
	result := make([]v1alpha1.RequestMatcher, 0, len(requestMatchers))
	for _, matcher := range requestMatchers {
		pathAsLuaPattern := make([]string, 0, len(matcher.Paths))
		for _, path := range matcher.Paths {
			pathAsLuaPattern = append(pathAsLuaPattern, ConvertRequestMatcherPathToRegex(path))
		}
		result = append(result, v1alpha1.RequestMatcher{
			Paths:   pathAsLuaPattern,
			Methods: matcher.Methods,
		})
	}
	return result
}
