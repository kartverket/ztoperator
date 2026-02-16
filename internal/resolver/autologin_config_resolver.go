package resolver

import (
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/luascript"
)

// ResolveAutoLoginConfig constructs the AutoLoginConfig from the AuthPolicy spec and resolved identity provider URIs.
func ResolveAutoLoginConfig(
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
	identityProviderUris state.IdentityProviderUris,
) state.AutoLoginConfig {
	if authPolicy.Spec.AutoLogin == nil || !authPolicy.Spec.AutoLogin.Enabled {
		return state.AutoLoginConfig{
			Enabled: false,
		}
	}

	envoySecretName := authPolicy.Name + "-envoy-secret"

	autoLoginConfig := state.AutoLoginConfig{
		Enabled:               authPolicy.Spec.AutoLogin.Enabled,
		LoginPath:             authPolicy.Spec.AutoLogin.LoginPath,
		PostLogoutRedirectURI: authPolicy.Spec.AutoLogin.PostLogoutRedirectURI,
		Scopes:                authPolicy.Spec.AutoLogin.Scopes,
		LoginParams:           authPolicy.Spec.AutoLogin.LoginParams,
		EnvoySecretName:       envoySecretName,
	}

	autoLoginConfig.SetSaneDefaults(*authPolicy.Spec.AutoLogin)

	autoLoginConfig.LuaScriptConfig = state.LuaScriptConfig{
		LuaScript: luascript.GetLuaScript(
			authPolicy,
			autoLoginConfig,
			identityProviderUris,
		),
	}

	return autoLoginConfig
}
