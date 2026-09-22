package resolver

import (
	"context"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/model"
	"github.com/kartverket/ztoperator/pkg/rest"
)

func ResolveDiscoveryDocument(
	ctx context.Context,
	authPolicy ztoperatorv1alpha1.AuthPolicy,
	resolver rest.DiscoveryDocumentResolver,
) (*model.IdentityProviderUris, error) {
	identityProviderUris, err := model.ToIdentityProviderUris(ctx, authPolicy, resolver)
	if err != nil {
		return nil, err
	}
	return identityProviderUris, nil
}
