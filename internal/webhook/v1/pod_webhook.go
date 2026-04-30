package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/validation"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	CreatedBySkipNamespaceLabel      = "skip.kartverket.no/skip-managed"
	CreatedBySkipNamespaceLabelValue = "true"

	SkiperatorApplicationRefLabel = "application.skiperator.no/app-name"

	ZtoperatorWebhookAnnotationPrefix = "ztoperator.kartverket.no/"
	ZtoperatorVerifyAnnotationKey     = ZtoperatorWebhookAnnotationPrefix + "verify-authpolicy"
	ZtoperatorVerifyAnnotationValue   = "true"
)

// nolint:unused
var podlog = logf.Log.WithName("pod-webhook")

// SetupPodWebhookWithManager registers the webhook for Pod in the manager.
func SetupPodWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &corev1.Pod{}).
		WithValidator(&PodCustomValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate--v1-pod,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=vpod-v1.kb.io,admissionReviewVersions=v1

// PodCustomValidator is responsible for validating Pods on create and update.
type PodCustomValidator struct {
	Client client.Client
}

var _ admission.Validator[*corev1.Pod] = &PodCustomValidator{}

func (v *PodCustomValidator) ValidateCreate(ctx context.Context, pod *corev1.Pod) (admission.Warnings, error) {
	return validatePod(ctx, v.Client, pod)
}

func (v *PodCustomValidator) ValidateUpdate(ctx context.Context, _, newPod *corev1.Pod) (admission.Warnings, error) {
	return validatePod(ctx, v.Client, newPod)
}

func (v *PodCustomValidator) ValidateDelete(_ context.Context, pod *corev1.Pod) (admission.Warnings, error) {
	podlog.Info("Validation for Pod upon deletion", "name", pod.GetName())
	return nil, nil
}

func validatePod(ctx context.Context, k8sClient client.Client, pod *corev1.Pod) (admission.Warnings, error) {
	podlog.Info("Validating for Pod", "name", pod.GetName())

	isPodWebhookEligible, errMsg := IsWebhookEligible(ctx, k8sClient, *pod)
	if !isPodWebhookEligible {
		podlog.Error(
			fmt.Errorf(
				"webhook eligibility check failed: %s",
				errMsg,
			),
			"received validating webhook request for pod that is not eligible for Ztoperator webhook processing",
			"pod",
			types.NamespacedName{
				Namespace: pod.GetNamespace(),
				Name:      pod.GetName(),
			},
		)
		return nil, nil
	}

	podAuthPolicyConfiguration, err := GetPodAuthPolicyConfiguration(ctx, k8sClient, pod)
	if err != nil {
		return nil, err
	}
	if !podAuthPolicyConfiguration.CreatedFromSkiperatorApplication {
		// Only validate Pods that are created from Skiperator Applications.
		podlog.Info("Pod is not created from Skiperator Application, skipping validation", "pod", types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		})
		return nil, nil
	}

	// Validate that the pod is correctly configured given the AuthPolicy.
	validationErr := validation.ValidatePodAnnotations(pod, podAuthPolicyConfiguration.AuthPolicy)
	return nil, validationErr
}

// PodAuthPolicyConfiguration holds all resolved security context for a Pod,
// used by both the mutating and validating webhooks.
type PodAuthPolicyConfiguration struct {
	AuthPolicy                       v1alpha1.AuthPolicy
	AppName                          string
	CreatedFromSkiperatorApplication bool
}

// GetPodAuthPolicyConfiguration resolves the full security configuration for a Pod.
// It returns a non-nil PodAuthPolicyConfiguration in all non-error cases.
func GetPodAuthPolicyConfiguration(
	ctx context.Context,
	k8sClient client.Client,
	pod *corev1.Pod,
) (*PodAuthPolicyConfiguration, error) {
	if pod.Labels == nil {
		return &PodAuthPolicyConfiguration{CreatedFromSkiperatorApplication: false}, nil
	}

	appName, isSkiperatorPod := pod.Labels[SkiperatorApplicationRefLabel]
	if !isSkiperatorPod {
		return &PodAuthPolicyConfiguration{CreatedFromSkiperatorApplication: false}, nil
	}

	if k8sClient == nil {
		return nil, fmt.Errorf("webhook client is not configured")
	}

	verifyAnnotation, hasVerify := pod.Annotations[ZtoperatorVerifyAnnotationKey]

	shouldFetchAuthPolicy := hasVerify && verifyAnnotation == ZtoperatorVerifyAnnotationValue
	if !shouldFetchAuthPolicy {
		return &PodAuthPolicyConfiguration{
			AppName:                          appName,
			CreatedFromSkiperatorApplication: true,
		}, nil
	}

	authPolicy, err := GetAuthPolicyForApplication(ctx, k8sClient, client.ObjectKey{
		Namespace: pod.Namespace,
		Name:      appName,
	})
	if err != nil {
		return nil, err
	}

	return &PodAuthPolicyConfiguration{
		AuthPolicy:                       *authPolicy,
		AppName:                          appName,
		CreatedFromSkiperatorApplication: true,
	}, nil
}

func IsWebhookEligible(ctx context.Context, k8sClient client.Client, pod corev1.Pod) (bool, string) {
	// Verify that pod is created from skiperator app
	if pod.Labels == nil {
		return false, fmt.Sprintf("pod %s/%s has no labels", pod.Namespace, pod.Name)
	}
	_, isSkiperatorPod := pod.Labels[SkiperatorApplicationRefLabel]
	if !isSkiperatorPod {
		return false, fmt.Sprintf("pod %s/%s is not created from a Skiperator Application", pod.Namespace, pod.Name)
	}

	// Verify that pod has annotations with the Ztoperator webhook annotation prefix
	if pod.Annotations == nil {
		return false, fmt.Sprintf("pod %s/%s has no annotations", pod.Namespace, pod.Name)
	}
	hasZtoperatorWebhookAnnotationPrefix := false
	for annotation := range pod.Annotations {
		if strings.HasPrefix(annotation, ZtoperatorWebhookAnnotationPrefix) {
			hasZtoperatorWebhookAnnotationPrefix = true
		}
	}
	if !hasZtoperatorWebhookAnnotationPrefix {
		return false, fmt.Sprintf("pod %s/%s has no Ztoperator webhook annotations", pod.Namespace, pod.Name)
	}

	// Verify that the pod lies in a SKIP managed namespace
	ns := &corev1.Namespace{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: pod.Namespace}, ns); err != nil {
		if errors.IsNotFound(err) {
			return false, fmt.Sprintf("namespace %s not found", pod.Namespace)
		}
		return false, fmt.Sprintf("failed to get namespace %s: %s", pod.Namespace, err.Error())
	}
	if ns.Labels == nil {
		return false, fmt.Sprintf("namespace %s has no labels", pod.Namespace)
	}
	value, hasLabel := ns.Labels[CreatedBySkipNamespaceLabel]
	if !hasLabel {
		return false, fmt.Sprintf("namespace %s does not have the created by SKIP label", pod.Namespace)
	}
	if value != CreatedBySkipNamespaceLabelValue {
		return false, fmt.Sprintf("namespace %s does have the created by SKIP label, but it's value is not %s", pod.Namespace, CreatedBySkipNamespaceLabelValue)
	}
	return true, ""
}
