package utilities

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KubernetesServiceURL struct {
	Name      string
	Namespace string
	Ports     []int32
}

var dnsLabelRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

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

func BuildObjectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{"type": "ztoperator.kartverket.no"},
	}
}

func Ptr[T any](v T) *T {
	return &v
}

func GetSecret(ctx context.Context, client client.Client, namespacedName types.NamespacedName) (v1.Secret, error) {
	secret := v1.Secret{}

	err := client.Get(ctx, namespacedName, &secret)

	return secret, err
}

func GetParsedURL(uri string) (*url.URL, error) {
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	return parsedURL, nil
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

func Base64EncodedSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	encoded := base64.StdEncoding.EncodeToString(sum[:])
	if len(encoded) < 6 {
		return encoded
	}
	return encoded[:6]
}

func GetProtectedPods(ctx context.Context, k8sClient client.Client, authPolicy v1alpha1.AuthPolicy) (*[]v1.Pod, error) {
	var podList v1.PodList
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

func ParseKubernetesServiceURL(ctx context.Context, k8sClient client.Client, u url.URL) (*KubernetesServiceURL, error) {
	if u.Scheme != "http" {
		return nil, fmt.Errorf("unsupported scheme %q: only http is allowed", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("hostname is required")
	}

	parts := strings.Split(host, ".")
	var service, namespace string

	switch {
	case len(parts) == 2:
		service, namespace = parts[0], parts[1]
	case len(parts) >= 4 && parts[2] == "svc":
		service, namespace = parts[0], parts[1]
	default:
		return nil, fmt.Errorf("hostname %q must be service.namespace or service.namespace.svc.<clusterDomain>", host)
	}

	if err := validateDNSLabel(service, "service"); err != nil {
		return nil, err
	}
	if err := validateDNSLabel(namespace, "namespace"); err != nil {
		return nil, err
	}

	var ports []int32
	portStr := u.Port()
	if portStr == "" {
		servicePorts, err := fetchServicePorts(ctx, k8sClient, namespace, service)
		if err != nil {
			return nil, err
		}
		ports = servicePorts
	} else {
		port, err := parsePort(portStr)
		if err != nil {
			return nil, err
		}
		ports = []int32{port}
	}

	return &KubernetesServiceURL{
		Name:      service,
		Namespace: namespace,
		Ports:     ports,
	}, nil
}

func validateDNSLabel(value, field string) error {
	if len(value) == 0 || len(value) > 63 || !dnsLabelRegex.MatchString(value) {
		return fmt.Errorf("%s %q must match %s and be 1-63 characters", field, value, dnsLabelRegex.String())
	}
	return nil
}

func parsePort(portStr string) (int32, error) {
	parsed, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil || parsed < 1 || parsed > 65535 {
		return 0, fmt.Errorf("invalid port %q", portStr)
	}
	return int32(parsed), nil
}

func fetchServicePorts(ctx context.Context, k8sClient client.Client, namespace, service string) ([]int32, error) {
	var svc v1.Service
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: service, Namespace: namespace}, &svc); err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s: %w", namespace, service, err)
	}
	if len(svc.Spec.Ports) == 0 {
		return nil, fmt.Errorf("service %s/%s has no ports defined", namespace, service)
	}

	ports := make([]int32, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		ports = append(ports, p.Port)
	}
	return ports, nil
}
