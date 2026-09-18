package v1

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
type AuthPolicyCustomValidator struct {
	DiscoveryDocumentCache *config.DiscoveryDocumentCache
}

var _ admission.Validator[*ztoperatorv1alpha1.AuthPolicy] = &AuthPolicyCustomValidator{}

// SetupAuthPolicyWebhookWithManager registers the AuthPolicy validating
// webhook with the configured well-known URI cache.
func SetupAuthPolicyWebhookWithManager(mgr ctrl.Manager, discoveryCache *config.DiscoveryDocumentCache) error {
	return ctrl.NewWebhookManagedBy(mgr, &ztoperatorv1alpha1.AuthPolicy{}).
		WithValidator(&AuthPolicyCustomValidator{DiscoveryDocumentCache: discoveryCache}).
		Complete()
}

func (v *AuthPolicyCustomValidator) ValidateCreate(
	_ context.Context,
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
) (admission.Warnings, error) {
	return nil, validateAuthPolicy(authPolicy, v.DiscoveryDocumentCache)
}

func (v *AuthPolicyCustomValidator) ValidateUpdate(
	_ context.Context,
	_, newAuthPolicy *ztoperatorv1alpha1.AuthPolicy,
) (admission.Warnings, error) {
	return nil, validateAuthPolicy(newAuthPolicy, v.DiscoveryDocumentCache)
}

func (v *AuthPolicyCustomValidator) ValidateDelete(
	_ context.Context,
	_ *ztoperatorv1alpha1.AuthPolicy,
) (admission.Warnings, error) {
	return nil, nil
}

func validateAuthPolicy(
	authPolicy *ztoperatorv1alpha1.AuthPolicy,
	discoveryCache *config.DiscoveryDocumentCache,
) error {
	if authPolicy == nil {
		return errors.New("AuthPolicy must not be nil")
	}

	if err := validation.ValidateWellKnownURI(authPolicy.Spec.WellKnownURI); err != nil {
		return err
	}
	if !discoveryCache.IsAllowed(authPolicy.Spec.WellKnownURI) {
		return fmt.Errorf("wellKnownURI %q is not in the configured allowlist", authPolicy.Spec.WellKnownURI)
	}
	return validation.ValidatePaths(authPolicy.GetPaths())
}
