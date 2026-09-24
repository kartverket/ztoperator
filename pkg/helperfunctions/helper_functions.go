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
	v1 "k8s.io/api/core/v1"
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

func GetSecret(ctx context.Context, k8sClient client.Client, namespacedName types.NamespacedName) (v1.Secret, error) {
	secret := v1.Secret{}

	err := k8sClient.Get(ctx, namespacedName, &secret)

	return secret, err
}

func GetConfigMap(
	ctx context.Context,
	k8sClient client.Client,
	namespacedName types.NamespacedName,
) (v1.ConfigMap, error) {
	configMap := v1.ConfigMap{}

	err := k8sClient.Get(ctx, namespacedName, &configMap)

	return configMap, err
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

// GetMockKubernetesClient returns a fake Kubernetes client with the provided scheme and objects. Only used in testing.
func GetMockKubernetesClient(scheme *runtime.Scheme, objects ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}
