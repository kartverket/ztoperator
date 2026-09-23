package resolver

import (
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/luascript"
	"github.com/kartverket/ztoperator/pkg/model"
)

// ResolveAutoLoginConfig constructs the AutoLoginConfig from the AuthPolicy spec and resolved identity provider URIs.
func ResolveAutoLoginConfig(
	authPolicy ztoperatorv1alpha1.AuthPolicy,
	identityProviderUris model.IdentityProviderUris,
) model.AutoLoginConfig {
	autoLoginConfig := model.ToAutoLoginConfig(
		authPolicy,
		identityProviderUris,
		luascript.GenerateLuaScript,
	)
	return autoLoginConfig
}
