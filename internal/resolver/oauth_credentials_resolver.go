package resolver

import (
	"context"
	"fmt"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"github.com/kartverket/ztoperator/pkg/model"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolveOAuthCredentials retrieves and validates OAuth client credentials from a Kubernetes Secret.
func ResolveOAuthCredentials(
	ctx context.Context,
	k8sClient client.Client,
	authPolicy ztoperatorv1alpha1.AuthPolicy,
) (*model.OAuthCredentials, error) {
	if authPolicy.Spec.OAuthCredentials == nil ||
		authPolicy.Spec.AutoLogin == nil ||
		!authPolicy.Spec.AutoLogin.Enabled {
		return &model.OAuthCredentials{}, nil
	}

	oAuthSecret, err := helperfunctions.GetSecret(ctx, k8sClient, types.NamespacedName{
		Namespace: authPolicy.Namespace,
		Name:      authPolicy.Spec.OAuthCredentials.SecretRef,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get OAuth credentials secret %s/%s: %w",
			authPolicy.Namespace,
			authPolicy.Spec.OAuthCredentials.SecretRef,
			err,
		)
	}

	oAuthProvider, found := config.GetOAuthProvider(authPolicy.Spec.WellKnownURI)
	if !found {
		return nil, fmt.Errorf(
			"wellKnownURI %q is not in the configured allowlist",
			authPolicy.Spec.WellKnownURI,
		)
	}

	return model.ToOAuthCredentials(
		authPolicy,
		oAuthSecret.Data,
		oAuthProvider.PrivateKeyJwtAssertionAudience,
	)
}
