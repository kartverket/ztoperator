package v1

import (
	"context"
	"fmt"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	SkiperatorApplicationRefLabel = "application.skiperator.no/app-name"

	ZtoperatorVerifyAnnotationKey   = "ztoperator.kartverket.no/verify-authpolicy"
	ZtoperatorVerifyAnnotationValue = "true"
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

	podAuthPolicy, err := GetPodAuthPolicyConfiguration(ctx, k8sClient, pod)
	if err != nil {
		return nil, err
	}
	if !podAuthPolicy.CreatedFromSkiperatorApplication {
		// Only validate Pods that are created from Skiperator Applications.
		podlog.Info("Pod is not created from Skiperator Application, skipping validation", "pod", types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		})
		return nil, nil
	}

	return nil, nil
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
