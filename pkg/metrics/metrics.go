package metrics

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	AuthPolicyInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "info",
			Namespace: "ztoperator",
			Subsystem: "authpolicy",
			Help:      "AuthPolicy info: 1 per policy with labels name, namespace, state, owner, issuer, enabled, autoLoginEnabled",
		},
		[]string{"name", "namespace", "state", "owner", "issuer", "enabled", "autoLoginEnabled"},
	)
)

type AuthPolicyInfoVec struct {
	Name             string
	Namespace        string
	State            v1alpha1.Phase
	Owner            string
	Issuer           string
	Enabled          bool
	AutoLoginEnabled bool
}

func MustRegister() {
	prometheus.MustRegister(AuthPolicyInfo)
}

func StartAuthPolicyCollector(k8sClient client.Client, c cache.Cache, elected <-chan struct{}) error {
	ctx := context.Background()
	if ok := c.WaitForCacheSync(ctx); !ok {
		return errors.New("failed to wait for cache sync")
	}

	go func() {
		<-elected
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			refreshOnce(ctx, k8sClient)
			<-t.C
		}
	}()
	return nil
}

func RefreshAuthPolicyInfo(ctx context.Context, k8sClient client.Client, authPolicy v1alpha1.AuthPolicy) error {
	var namespace v1.Namespace
	_ = k8sClient.Get(ctx, client.ObjectKey{Name: authPolicy.Namespace}, &namespace)

	idpAsParsedURL, err := utils.GetParsedURL(authPolicy.Spec.WellKnownURI)
	if err != nil {
		panic("failed to get issuer hostname from issuer URI " + authPolicy.Spec.WellKnownURI + " due to the following error: " + err.Error())
	}

	var autoLoginEnabled = false
	if authPolicy.Spec.AutoLogin != nil {
		autoLoginEnabled = authPolicy.Spec.AutoLogin.Enabled
	}

	AuthPolicyInfo.WithLabelValues(
		authPolicy.Name,
		authPolicy.Namespace,
		string(authPolicy.Status.Phase),
		namespace.Labels["team"],
		idpAsParsedURL.Scheme+"://"+idpAsParsedURL.Hostname(),
		strconv.FormatBool(authPolicy.Spec.Enabled),
		strconv.FormatBool(autoLoginEnabled),
	).Set(1)
	return nil
}

func refreshOnce(ctx context.Context, k8sClient client.Client) {
	AuthPolicyInfo.Reset()
	var authPolicyList v1alpha1.AuthPolicyList

	_ = k8sClient.List(ctx, &authPolicyList)

	for _, authPolicy := range authPolicyList.Items {
		_ = RefreshAuthPolicyInfo(ctx, k8sClient, authPolicy)
	}
}
