package bases

import _ "embed"

//go:embed ztoperator.kartverket.no_authpolicies.yaml
var AuthPolicyCRD []byte
