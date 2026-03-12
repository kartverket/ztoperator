package names

func EnvoyFilter(base string) string   { return base + "-login" }
func EnvoySecret(base string) string   { return base + "-envoy-secret" }
func DenyPolicy(base string) string    { return base + "-deny-auth-rules" }
func IgnorePolicy(base string) string  { return base + "-ignore-auth" }
func RequirePolicy(base string) string { return base + "-require-auth" }
