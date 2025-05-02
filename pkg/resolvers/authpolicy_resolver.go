package resolvers

import (
	"context"
	"fmt"
	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/internal/state"
	"github.com/kartverket/ztoperator/pkg/utils"
	"github.com/nais/digdirator/pkg/secrets"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"istio.io/api/security/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var AcceptedHttpMethods = []string{
	"GET",
	"POST",
	"PUT",
	"PATCH",
	"DELETE",
	"HEAD",
	"OPTIONS",
	"TRACE",
	"CONNECT",
}

func ResolveAuthPolicy(ctx context.Context, k8sClient client.Client, authPolicy *ztoperatorv1alpha1.AuthPolicy) (*state.ResolvedAuthPolicy, error) {
	var resolvedRules []state.ResolvedRule
	for _, rule := range authPolicy.Spec.Rules {
		resolvedAuthPolicyRule, err := resolveAuthPolicyRule(ctx, k8sClient, authPolicy.Namespace, rule)
		if err != nil {
			return nil, err
		}
		resolvedRules = append(resolvedRules, *resolvedAuthPolicyRule)
	}
	return &state.ResolvedAuthPolicy{
		AuthPolicy:    authPolicy,
		ResolvedRules: ignorePathsFromOtherRules(resolvedRules),
	}, nil
}

func resolveAuthPolicyRule(ctx context.Context, k8sClient client.Client, namespace string, authPolicyRule ztoperatorv1alpha1.RequestAuth) (*state.ResolvedRule, error) {
	jwtSecret, err := utils.GetSecret(ctx, k8sClient, types.NamespacedName{Namespace: namespace, Name: authPolicyRule.SecretName})
	if err != nil {
		return nil, err
	}
	var authRules []ztoperatorv1alpha1.RequestAuthRule
	if authPolicyRule.AuthRules != nil {
		for _, authRule := range *authPolicyRule.AuthRules {
			authRules = append(authRules, authRule)
		}
	}
	var ignoreAuthRules []ztoperatorv1alpha1.RequestMatcher
	if authPolicyRule.IgnoreAuthRules != nil {
		for _, ignoreAuthRule := range *authPolicyRule.IgnoreAuthRules {
			ignoreAuthRules = append(ignoreAuthRules, ignoreAuthRule)
		}
	}
	authPolicyRule.AuthRules = &authRules
	authPolicyRule.IgnoreAuthRules = &ignoreAuthRules
	resolvedAuthPolicyRule := &state.ResolvedRule{
		Rule: authPolicyRule,
	}

	switch jwtSecret.Annotations["type"] {
	case "digdirator.nais.io":
		{
			// JWT-secret was created by IdPortenClient
			resolvedAuthPolicyRule.Audiences = append(*authPolicyRule.AcceptedResources, string(jwtSecret.Data[secrets.IDPortenIssuerKey]))
			resolvedAuthPolicyRule.JwksUri = string(jwtSecret.Data[secrets.IDPortenJwksUriKey])
			resolvedAuthPolicyRule.IssuerUri = string(jwtSecret.Data[secrets.IDPortenIssuerKey])
			return resolvedAuthPolicyRule, nil
		}
	case "maskinporten.digdirator.nais.io":
		{
			// JWT-secret was created by MaskinportenClient
			resolvedAuthPolicyRule.Audiences = append(*authPolicyRule.AcceptedResources, string(jwtSecret.Data[secrets.MaskinportenClientIDKey]))
			resolvedAuthPolicyRule.JwksUri = string(jwtSecret.Data[secrets.MaskinportenJwksUriKey])
			resolvedAuthPolicyRule.IssuerUri = string(jwtSecret.Data[secrets.MaskinportenIssuerKey])
			return resolvedAuthPolicyRule, nil
		}
	case "azurerator.nais.io":
		{
			// JWT-secret was created by AzureAdApplication
			// AzureAdApplication supports Secret Key Prefix
			secretKeyPrefix := "AZURE"
			if authPolicyRule.SecretPrefix != nil {
				secretKeyPrefix = *authPolicyRule.SecretPrefix
			}

			// Entra ID does not implement RFC8707, which introduces resource indicator for OAuth 2.0
			resolvedAuthPolicyRule.Audiences = []string{string(jwtSecret.Data[fmt.Sprintf("%s_APP_CLIENT_ID", secretKeyPrefix)])}
			resolvedAuthPolicyRule.JwksUri = string(jwtSecret.Data[fmt.Sprintf("%s_OPENID_CONFIG_JWKS_URI", secretKeyPrefix)])
			resolvedAuthPolicyRule.IssuerUri = string(jwtSecret.Data[fmt.Sprintf("%s_OPENID_CONFIG_ISSUER", secretKeyPrefix)])
			return resolvedAuthPolicyRule, nil
		}
	default:
		{
			return nil, fmt.Errorf("JWT secret annotated with unknown type %s", jwtSecret.Annotations["type"])
		}
	}
}

func ignorePathsFromOtherRules(resolvedRules state.ResolvedRuleList) state.ResolvedRuleList {
	for index, resolvedRule := range resolvedRules {
		jwtRule := resolvedRule.Rule
		requireAuthRequestMatchers := ztoperatorv1alpha1.GetRequestMatchers(jwtRule.AuthRules)
		ignoredRequestMatchers := flattenOnPaths(*jwtRule.IgnoreAuthRules)
		authorizedRequestMatchers := flattenOnPaths(requireAuthRequestMatchers)
		for otherIndex, otherResolvedRule := range resolvedRules {
			if index != otherIndex {
				otherJwtRule := otherResolvedRule.Rule
				otherRequireAuthRequestMatchers := ztoperatorv1alpha1.GetRequestMatchers(otherJwtRule.AuthRules)
				otherAuthorizedRequestMatchers := flattenOnPaths(otherRequireAuthRequestMatchers)
				for otherPath, otherRequestMapper := range otherAuthorizedRequestMatchers {
					if !slices.Contains(maps.Keys(ignoredRequestMatchers), otherPath) &&
						!slices.Contains(maps.Keys(authorizedRequestMatchers), otherPath) {
						*jwtRule.IgnoreAuthRules = append(*jwtRule.IgnoreAuthRules, ztoperatorv1alpha1.RequestMatcher{
							Paths:   otherRequestMapper.Operation.Paths,
							Methods: otherRequestMapper.Operation.Methods,
						})
					}
				}
			}
		}
		resolvedRules[index] = resolvedRule
	}
	return resolvedRules
}

func flattenOnPaths(requestMatchers []ztoperatorv1alpha1.RequestMatcher) map[string]*v1beta1.Rule_To {
	requestMatchersMap := make(map[string]*v1beta1.Rule_To)
	if requestMatchers != nil {
		for _, requestMatcher := range requestMatchers {
			for _, path := range requestMatcher.Paths {
				if existingMatcher, exists := requestMatchersMap[path]; exists {
					// Combine methods if the path key already exists and remove duplicates
					uniqueMethods := make(map[string]struct{})
					for _, method := range append(existingMatcher.Operation.Methods, requestMatcher.Methods...) {
						uniqueMethods[method] = struct{}{}
					}
					existingMatcher.Operation.Methods = maps.Keys(uniqueMethods)
					requestMatchersMap[path] = existingMatcher
				} else {
					methods := requestMatcher.Methods
					if len(methods) == 0 {
						methods = AcceptedHttpMethods
					}
					requestMatchersMap[path] = &v1beta1.Rule_To{
						Operation: &v1beta1.Operation{
							Paths:   []string{path},
							Methods: methods,
						},
					}
				}
			}
		}
	}
	return requestMatchersMap
}
