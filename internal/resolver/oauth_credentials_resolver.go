package resolver

import (
	"context"
	"fmt"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/helperfunctions"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolveOAuthCredentials retrieves and validates OAuth client credentials from a Kubernetes Secret.
func ResolveOAuthCredentials(
	ctx context.Context,
	k8sClient client.Client,
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
) (*state.OAuthCredentials, error) {
	if authPolicy.Spec.OAuthCredentials == nil ||
		authPolicy.Spec.AutoLogin == nil ||
		!authPolicy.Spec.AutoLogin.Enabled {
		return &state.OAuthCredentials{}, nil
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

	clientID := string(oAuthSecret.Data[authPolicy.Spec.OAuthCredentials.ClientIDKey])
	if clientID == "" {
		return nil, fmt.Errorf(
			"client id with key: %s was nil or empty when retrieving it from Secret with name %s/%s",
			authPolicy.Spec.OAuthCredentials.ClientIDKey,
			authPolicy.Namespace,
			authPolicy.Spec.OAuthCredentials.SecretRef,
		)
	}

	clientSecret := string(oAuthSecret.Data[authPolicy.Spec.OAuthCredentials.ClientSecretKey])
	if clientSecret == "" {
		return nil, fmt.Errorf(
			"client secret with key: %s was nil or empty when retrieving it from Secret with name %s/%s",
			authPolicy.Spec.OAuthCredentials.ClientSecretKey,
			authPolicy.Namespace,
			authPolicy.Spec.OAuthCredentials.SecretRef,
		)
	}

	return &state.OAuthCredentials{
		ClientID:     &clientID,
		ClientSecret: &clientSecret,
	}, nil
}
