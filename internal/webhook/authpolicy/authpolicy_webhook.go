package authpolicy

import (
	"context"
	"errors"
	"fmt"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/config"
	"github.com/kartverket/ztoperator/pkg/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-ztoperator-kartverket-no-v1alpha1-authpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=ztoperator.kartverket.no,resources=authpolicies,verbs=create;update,versions=v1alpha1,name=vauthpolicy-v1alpha1.kb.io,admissionReviewVersions=v1

// AuthPolicyCustomValidator validates AuthPolicy objects before they are
// persisted by the Kubernetes API server.
type AuthPolicyCustomValidator struct{}

var _ admission.Validator[*ztoperatorv1alpha1.AuthPolicy] = &AuthPolicyCustomValidator{}

// SetupAuthPolicyWebhookWithManager registers the AuthPolicy validating webhook.
func SetupAuthPolicyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ztoperatorv1alpha1.AuthPolicy{}).
		WithValidator(&AuthPolicyCustomValidator{}).
		Complete()
}

func (v *AuthPolicyCustomValidator) ValidateCreate(
	_ context.Context,
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
) (admission.Warnings, error) {
	return nil, validateAuthPolicy(authPolicy)
}

func (v *AuthPolicyCustomValidator) ValidateUpdate(
	_ context.Context,
	_, newAuthPolicy *ztoperatorv1alpha1.AuthPolicy,
) (admission.Warnings, error) {
	return nil, validateAuthPolicy(newAuthPolicy)
}

func (v *AuthPolicyCustomValidator) ValidateDelete(
	_ context.Context,
	_ *ztoperatorv1alpha1.AuthPolicy,
) (admission.Warnings, error) {
	return nil, nil
}

func validateAuthPolicy(authPolicy *ztoperatorv1alpha1.AuthPolicy) error {
	if authPolicy == nil {
		return errors.New("AuthPolicy must not be nil")
	}

	provider, found := findOAuthProvider(authPolicy.Spec.WellKnownURI)
	if !found {
		return fmt.Errorf("wellKnownURI %q is not in the configured allowlist", authPolicy.Spec.WellKnownURI)
	}
	if provider.RequireAudience && len(authPolicy.Spec.AllowedAudiences) == 0 {
		return fmt.Errorf("allowedAudiences must be non-empty for wellKnownURI %q", authPolicy.Spec.WellKnownURI)
	}
	if authPolicy.Spec.AutoLogin != nil && authPolicy.Spec.AutoLogin.Enabled {
		if provider.AuthorizationEndpoint == "" || provider.EndSessionEndpoint == "" {
			return fmt.Errorf(
				"issuer %s for AuthPolicy with name %s/%s does not support authorization endpoint "+
					"or end session endpoint required for autologin",
				provider.Issuer,
				authPolicy.Namespace,
				authPolicy.Name,
			)
		}
	}
	if err := validation.ValidatePaths(authPolicy.GetPaths()); err != nil {
		return err
	}
	return nil
}

func findOAuthProvider(wellKnownURI string) (config.OAuthProvider, bool) {
	for _, provider := range config.Get().AllowedOAuthProviders {
		if provider.WellKnownURI == wellKnownURI {
			return provider, true
		}
	}
	return config.OAuthProvider{}, false
}
