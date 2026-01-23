package resolver

import (
	"context"
	"fmt"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/rest"
)

func ResolveDiscoveryDocument(
	ctx context.Context,
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
	resolver rest.DiscoveryDocumentResolver,
) (*state.IdentityProviderUris, error) {
	rLog := log.GetLogger(ctx)
	var identityProviderUris state.IdentityProviderUris
	discoveryDocument, err := resolver.GetOAuthDiscoveryDocument(authPolicy.Spec.WellKnownURI, rLog)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve discovery document from well-known uri: %s for AuthPolicy with name %s/%s: %w",
			authPolicy.Spec.WellKnownURI,
			authPolicy.Namespace,
			authPolicy.Name,
			err,
		)
	}

	if discoveryDocument.Issuer == nil || discoveryDocument.JwksURI == nil || discoveryDocument.TokenEndpoint == nil {
		return nil, fmt.Errorf(
			"failed to parse discovery document from well-known uri: %s for AuthPolicy with name %s/%s",
			authPolicy.Spec.WellKnownURI,
			authPolicy.Namespace,
			authPolicy.Name,
		)
	}

	if authPolicy.Spec.AutoLogin != nil && authPolicy.Spec.AutoLogin.Enabled {
		if discoveryDocument.AuthorizationEndpoint == nil || discoveryDocument.EndSessionEndpoint == nil {
			return nil, fmt.Errorf(
				"issuer %s for AuthPolicy with name %s/%s does not support authorization endpoint or end session endpoint required for autologin",
				*discoveryDocument.Issuer,
				authPolicy.Namespace,
				authPolicy.Name,
			)
		}
	}

	identityProviderUris.IssuerURI = *discoveryDocument.Issuer
	identityProviderUris.JwksURI = *discoveryDocument.JwksURI
	identityProviderUris.TokenURI = *discoveryDocument.TokenEndpoint
	if discoveryDocument.AuthorizationEndpoint != nil {
		identityProviderUris.AuthorizationURI = *discoveryDocument.AuthorizationEndpoint
	}
	if discoveryDocument.EndSessionEndpoint != nil {
		identityProviderUris.EndSessionURI = discoveryDocument.EndSessionEndpoint
	}

	return &identityProviderUris, nil
}
