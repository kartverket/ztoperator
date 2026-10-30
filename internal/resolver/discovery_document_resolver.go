package resolver

import (
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/model"
)

func ResolveDiscoveryDocument(
	authPolicy ztoperatorv1alpha1.AuthPolicy,
) (*model.IdentityProviderUris, error) {
	identityProviderUris, err := model.ToIdentityProviderUris(authPolicy)
	if err != nil {
		return nil, err
	}
	return identityProviderUris, nil
}
