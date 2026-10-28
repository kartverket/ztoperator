package model

import (
	"fmt"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
)

func ToIdentityProviderUris(
	authPolicy v1alpha1.AuthPolicy,
) (*IdentityProviderUris, error) {
	var identityProviderUris IdentityProviderUris

	for _, provider := range config.Get().AllowedOAuthProviders {
		if provider.WellKnownURI != authPolicy.Spec.WellKnownURI {
			continue
		}

		identityProviderUris.IssuerURI = provider.Issuer
		identityProviderUris.JwksURI = provider.JwksURI
		identityProviderUris.TokenURI = provider.TokenEndpoint
		if provider.AuthorizationEndpoint != "" {
			identityProviderUris.AuthorizationURI = provider.AuthorizationEndpoint
		}
		if provider.EndSessionEndpoint != "" {
			identityProviderUris.EndSessionURI = helperfunctions.Ptr(provider.EndSessionEndpoint)
		}

		return &identityProviderUris, nil
	}

	return nil, fmt.Errorf(
		"well-known URI %q is not in the configured allowlist",
		authPolicy.Spec.WellKnownURI,
	)
}
