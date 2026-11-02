package helperfunctions

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type ParsedHttpURL struct {
	Host string
	Port int32
	Tls  bool
}

func LowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero() && j.IsZero():
		return ctrl.Result{}
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.RequeueAfter != 0 && j.RequeueAfter != 0:
		if i.RequeueAfter < j.RequeueAfter {
			return i
		}
		return j
	case i.RequeueAfter != 0:
		return i
	case j.RequeueAfter != 0:
		return j
	default:
		return ctrl.Result{RequeueAfter: 0 * time.Second}
	}
}

func Ptr[T any](v T) *T {
	return &v
}

func GetSecret(
	ctx context.Context,
	k8sClient client.Client,
	namespacedName types.NamespacedName,
) (corev1.Secret, error) {
	secret := corev1.Secret{}

	err := k8sClient.Get(ctx, namespacedName, &secret)

	return secret, err
}

func GetConfigMap(
	ctx context.Context,
	k8sClient client.Client,
	namespacedName types.NamespacedName,
) (corev1.ConfigMap, error) {
	configMap := corev1.ConfigMap{}

	err := k8sClient.Get(ctx, namespacedName, &configMap)

	return configMap, err
}

func GetNamespace(
	ctx context.Context,
	k8sClient client.Client,
	namespaceName string,
) (corev1.Namespace, error) {
	namespace := corev1.Namespace{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: namespaceName}, &namespace)
	return namespace, err
}

func GetParsedHttpURL(raw string) (*ParsedHttpURL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse URL %q: %w", raw, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("URL %q has no host (did you forget the scheme, e.g. 'http://'?)", raw)
	}
	var tls bool
	switch u.Scheme {
	case "http":
		tls = false
	case "https":
		tls = true
	default:
		return nil, fmt.Errorf("upstream scheme %q not supported (use http or https)", u.Scheme)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
		if tls {
			portStr = "443"
		} else {
			portStr = "80"
		}
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("parse upstream port %q: %w", portStr, err)
	}
	return &ParsedHttpURL{Host: host, Port: int32(port), Tls: tls}, nil
}

func GenerateHMACSecret(size int) (*string, error) {
	secret := make([]byte, size)
	_, err := rand.Read(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HMAC secret: %w", err)
	}
	base64EncodedSecret := base64.StdEncoding.EncodeToString(secret)
	return &base64EncodedSecret, nil
}

func GetProtectedPods(
	ctx context.Context,
	k8sClient client.Client,
	authPolicy v1alpha1.AuthPolicy,
) (*[]corev1.Pod, error) {
	var podList corev1.PodList
	if listErr := k8sClient.List(
		ctx,
		&podList,
		client.InNamespace(authPolicy.Namespace),
		client.MatchingLabels(authPolicy.Spec.Selector.MatchLabels),
	); listErr != nil {
		return nil, fmt.Errorf(
			"failed to get list of pods with the label: %s from authpolicy {%s, %s} due to the following error: %w",
			authPolicy.Spec.Selector.MatchLabels,
			authPolicy.Namespace,
			authPolicy.Name,
			listErr,
		)
	}
	return &podList.Items, nil
}

func GetProtectedDeployment(
	ctx context.Context,
	k8sClient client.Client,
	authPolicy v1alpha1.AuthPolicy,
) (*appsv1.Deployment, error) {
	var deploymentList appsv1.DeploymentList
	if listErr := k8sClient.List(
		ctx,
		&deploymentList,
		client.InNamespace(authPolicy.Namespace),
	); listErr != nil {
		return nil, fmt.Errorf(
			"list deployments in namespace %s: %w",
			authPolicy.Namespace,
			listErr,
		)
	}

	selector := labels.SelectorFromSet(authPolicy.Spec.Selector.MatchLabels)
	matchingDeployments := make([]appsv1.Deployment, 0, len(deploymentList.Items))
	for _, deployment := range deploymentList.Items {
		if selector.Matches(labels.Set(deployment.Spec.Template.Labels)) {
			matchingDeployments = append(matchingDeployments, deployment)
		}
	}

	if len(matchingDeployments) > 1 {
		return nil, fmt.Errorf(
			"more than 1 deployment matched the selector %v in namespace %s",
			authPolicy.Spec.Selector.MatchLabels,
			authPolicy.Namespace,
		)
	}
	if len(matchingDeployments) == 0 {
		return nil, fmt.Errorf(
			"no deployments matched the selector %v in namespace %s",
			authPolicy.Spec.Selector.MatchLabels,
			authPolicy.Namespace,
		)
	}

	return &matchingDeployments[0], nil
}

func GetSkiperatorAppPort(deployment appsv1.Deployment) (int, error) {
	skiperatorAppName, hasLabel := deployment.Labels[constants.SkiperatorApplicationRefLabel]
	if !hasLabel {
		return 0, fmt.Errorf(
			"deployment %s/%s does not have the label %s",
			deployment.Namespace,
			deployment.Name,
			constants.SkiperatorApplicationRefLabel,
		)
	}
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == skiperatorAppName {
			if len(container.Ports) == 0 {
				return 0, fmt.Errorf(
					"container %s in deployment %s/%s does not have any ports defined",
					skiperatorAppName,
					deployment.Namespace,
					deployment.Name,
				)
			}
			for _, port := range container.Ports {
				if port.Name == constants.SkiperatorAppContainerPortName {
					return int(port.ContainerPort), nil
				}
			}
			return 0, fmt.Errorf("container %s in deployment %s/%s does not have a port named %s",
				skiperatorAppName,
				deployment.Namespace,
				deployment.Name,
				constants.SkiperatorAppContainerPortName,
			)
		}
	}
	return 0, fmt.Errorf(" 	container %s not found in deployment %s/%s",
		skiperatorAppName,
		deployment.Namespace,
		deployment.Name,
	)
}

// GetMockKubernetesClient returns a fake Kubernetes client with the provided scheme and objects. Only used in testing.
func GetMockKubernetesClient(scheme *runtime.Scheme, objects ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}

func PodIsIstioAmbientEligible(pod corev1.Pod) bool {
	if pod.Labels[constants.IstioSidecarInjectLabelKey] == constants.IstioSidecarInjectTrueValue {
		return false
	}
	if _, hasRevision := pod.Labels[constants.IstioRevisionLabelKey]; hasRevision {
		return false
	}
	if pod.Labels[constants.IstioAmbientLabelKey] == constants.IstioAmbientOptOutValue {
		return false
	}
	if pod.Annotations[constants.IstioAmbientRedirectionKey] == constants.IstioAmbientRedirectionOptOut {
		return false
	}
	if pod.Spec.HostNetwork {
		return false
	}
	return true
}

func DeploymentIsIstioAmbientEligible(deployment appsv1.Deployment) bool {
	templateSpec := deployment.Spec.Template
	if templateSpec.Labels[constants.IstioSidecarInjectLabelKey] == constants.IstioSidecarInjectTrueValue {
		return false
	}
	if _, hasRevision := templateSpec.Labels[constants.IstioRevisionLabelKey]; hasRevision {
		return false
	}
	if templateSpec.Labels[constants.IstioAmbientLabelKey] == constants.IstioAmbientOptOutValue {
		return false
	}
	if templateSpec.Annotations[constants.IstioAmbientRedirectionKey] == constants.IstioAmbientRedirectionOptOut {
		return false
	}
	return true
}

func NamespaceIsIstioAmbientEligible(namespace corev1.Namespace) bool {
	if namespace.Labels[constants.IstioAmbientLabelKey] != constants.IstioAmbientLabelValue {
		return false
	}
	if namespace.Labels[constants.IstioInjectionLabelKey] == constants.IstioInjectionEnabledValue {
		return false
	}
	if _, hasRevision := namespace.Labels[constants.IstioRevisionLabelKey]; hasRevision {
		return false
	}
	return true
}
