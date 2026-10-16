package authpolicy

import (
	"context"
	"errors"
	"fmt"

	ztoperatorv1alpha1 "github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-ztoperator-kartverket-no-v1alpha1-authpolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=ztoperator.kartverket.no,resources=authpolicies,verbs=create;update,versions=v1alpha1,name=vauthpolicy-v1alpha1.kb.io,admissionReviewVersions=v1

// AuthPolicyCustomValidator validates AuthPolicy objects before they are
// persisted by the Kubernetes API server.
type AuthPolicyCustomValidator struct {
	AllowedWellKnownURIs          []string
	AudienceRequiredWellKnownURIs []string
}

var _ admission.Validator[*ztoperatorv1alpha1.AuthPolicy] = &AuthPolicyCustomValidator{}

// SetupAuthPolicyWebhookWithManager registers the AuthPolicy validating
// webhook with the configured well-known URI allowlist.
func SetupAuthPolicyWebhookWithManager(
	mgr ctrl.Manager,
	allowedWellKnownURIs []string,
	audienceRequiredWellKnownURIs []string,
) error {
	return ctrl.NewWebhookManagedBy(mgr, &ztoperatorv1alpha1.AuthPolicy{}).
		WithValidator(&AuthPolicyCustomValidator{
			AllowedWellKnownURIs:          allowedWellKnownURIs,
			AudienceRequiredWellKnownURIs: audienceRequiredWellKnownURIs,
		}).
		Complete()
}

func (v *AuthPolicyCustomValidator) ValidateCreate(
	_ context.Context,
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
) (admission.Warnings, error) {
	return nil, validateAuthPolicy(authPolicy, v.AllowedWellKnownURIs, v.AudienceRequiredWellKnownURIs)
}

func (v *AuthPolicyCustomValidator) ValidateUpdate(
	_ context.Context,
	_, newAuthPolicy *ztoperatorv1alpha1.AuthPolicy,
) (admission.Warnings, error) {
	return nil, validateAuthPolicy(newAuthPolicy, v.AllowedWellKnownURIs, v.AudienceRequiredWellKnownURIs)
}

func (v *AuthPolicyCustomValidator) ValidateDelete(
	_ context.Context,
	_ *ztoperatorv1alpha1.AuthPolicy,
) (admission.Warnings, error) {
	return nil, nil
}

func validateAuthPolicy(
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
	allowedWellKnownURIs []string,
	audienceRequiredWellKnownURIs []string,
) error {
	if authPolicy == nil {
		return errors.New("AuthPolicy must not be nil")
	}

	if !containsWellKnownURI(allowedWellKnownURIs, authPolicy.Spec.WellKnownURI) {
		return fmt.Errorf("wellKnownURI %q is not in the configured allowlist", authPolicy.Spec.WellKnownURI)
	}
	if containsWellKnownURI(audienceRequiredWellKnownURIs, authPolicy.Spec.WellKnownURI) &&
		len(authPolicy.Spec.AllowedAudiences) == 0 {
		return fmt.Errorf("allowedAudiences must be non-empty for wellKnownURI %q", authPolicy.Spec.WellKnownURI)
	}
	if err := validation.ValidatePaths(authPolicy.GetPaths()); err != nil {
		return err
	}
	return nil
}

func containsWellKnownURI(allowedWellKnownURIs []string, uri string) bool {
	for _, allowedURI := range allowedWellKnownURIs {
		if allowedURI == uri {
			return true
		}
	}
	return false
}
