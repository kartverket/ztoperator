package model

import (
	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/names"
)

type AutoLoginConfig struct {
	Enabled               bool
	LoginPath             *string
	RedirectPath          string
	LogoutPath            string
	PostLogoutRedirectURI *string
	Scopes                []string
	ResourceIndicators    []string
	LoginParams           map[string]string
	LuaScriptConfig       LuaScriptConfig
	EnvoySecretName       string
}

type LuaScriptGenerator func(
	authPolicy v1alpha1.AuthPolicy,
	autoLoginConfig AutoLoginConfig,
	identityProviderUris IdentityProviderUris,
) string

type LuaScriptConfig struct {
	LuaScript string
}

func ToAutoLoginConfig(
	authPolicy v1alpha1.AuthPolicy,
	identityProviderUris IdentityProviderUris,
	luaScriptGenerator LuaScriptGenerator,
) AutoLoginConfig {
	envoySecretName := names.EnvoySecret(authPolicy.Name)

	if authPolicy.Spec.AutoLogin == nil || !authPolicy.Spec.AutoLogin.Enabled {
		return AutoLoginConfig{
			Enabled:         false,
			EnvoySecretName: envoySecretName,
		}
	}

	autoLoginConfig := AutoLoginConfig{
		Enabled:               authPolicy.Spec.AutoLogin.Enabled,
		LoginPath:             authPolicy.Spec.AutoLogin.LoginPath,
		PostLogoutRedirectURI: authPolicy.Spec.AutoLogin.PostLogoutRedirectURI,
		Scopes:                authPolicy.Spec.AutoLogin.Scopes,
		LoginParams:           authPolicy.Spec.AutoLogin.LoginParams,
		EnvoySecretName:       envoySecretName,
	}

	if authPolicy.Spec.AcceptedResources != nil {
		autoLoginConfig.ResourceIndicators = *authPolicy.Spec.AcceptedResources
	}

	autoLoginConfig.SetSaneDefaults(*authPolicy.Spec.AutoLogin)

	autoLoginConfig.LuaScriptConfig = LuaScriptConfig{
		LuaScript: luaScriptGenerator(
			authPolicy,
			autoLoginConfig,
			identityProviderUris,
		),
	}

	return autoLoginConfig
}
