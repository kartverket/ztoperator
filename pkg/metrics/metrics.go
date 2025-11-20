package metrics

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/kartverket/ztoperator/api/v1alpha1"
	"github.com/kartverket/ztoperator/pkg/log"
	"github.com/kartverket/ztoperator/pkg/utilities"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const collectorInterval = 30 * time.Second

var (
	authPolicyInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:      "info",
			Namespace: "ztoperator",
			Subsystem: "authpolicy",
			Help:      "AuthPolicy info: 1 per policy with labels name, namespace, state, owner, issuer, enabled, auto_login_enabled, protected_pod",
		},
		[]string{
			"name",
			"namespace",
			"state",
			"owner",
			"issuer",
			"enabled",
			"auto_login_enabled",
			"protected_pod",
		},
	)
	logger = log.Logger{Logger: ctrl.Log.WithName("metrics")}
)

func MustRegister() {
	metrics.Registry.MustRegister(authPolicyInfo)
}

func StartAuthPolicyCollector(k8sClient client.Client, c cache.Cache, elected <-chan struct{}) error {
	logger.Debug("Starting auth policy metrics collector")
	ctx := context.Background()
	if ok := c.WaitForCacheSync(ctx); !ok {
		return errors.New("failed to wait for cache sync")
	}

	go func() {
		<-elected
		t := time.NewTicker(collectorInterval)
		defer t.Stop()
		for {
			refreshOnce(ctx, k8sClient)
			<-t.C
		}
	}()
	return nil
}

func RefreshAuthPolicyInfo(ctx context.Context, k8sClient client.Client, authPolicy v1alpha1.AuthPolicy) error {
	logger.Debug(
		"Refreshing auth policy info",
		"namespace", authPolicy.Namespace,
		"name", authPolicy.Name,
	)
	var namespace v1.Namespace
	_ = k8sClient.Get(ctx, client.ObjectKey{Name: authPolicy.Namespace}, &namespace)

	idpAsParsedURL, err := utilities.GetParsedURL(authPolicy.Spec.WellKnownURI)
	if err != nil {
		return fmt.Errorf(
			"failed to get issuer hostname from issuer URI %s due to the following error: %w",
			authPolicy.Spec.WellKnownURI,
			err,
		)
	}

	protectedPods, getProtectedPodsErr := utilities.GetProtectedPods(ctx, k8sClient, authPolicy)
	if getProtectedPodsErr != nil {
		return err
	}

	var autoLoginEnabled = false
	if authPolicy.Spec.AutoLogin != nil {
		autoLoginEnabled = authPolicy.Spec.AutoLogin.Enabled
	}

	if len(*protectedPods) == 0 {
		authPolicyInfo.WithLabelValues(
			authPolicy.Name,
			authPolicy.Namespace,
			string(authPolicy.Status.Phase),
			namespace.Labels["team"],
			idpAsParsedURL.Scheme+"://"+idpAsParsedURL.Hostname(),
			strconv.FormatBool(authPolicy.Spec.Enabled),
			strconv.FormatBool(autoLoginEnabled),
			"",
		).Set(1)
	}

	for _, pod := range *protectedPods {
		authPolicyInfo.WithLabelValues(
			authPolicy.Name,
			authPolicy.Namespace,
			string(authPolicy.Status.Phase),
			namespace.Labels["team"],
			idpAsParsedURL.Scheme+"://"+idpAsParsedURL.Hostname(),
			strconv.FormatBool(authPolicy.Spec.Enabled),
			strconv.FormatBool(autoLoginEnabled),
			pod.Name,
		).Set(1)
	}

	logger.Debug(
		"Successfully refreshed auth policy",
		"namespace", authPolicy.Namespace,
		"name", authPolicy.Name,
	)
	return nil
}

func DeleteAuthPolicyInfo(namespacedName types.NamespacedName) {
	authPolicyInfo.DeletePartialMatch(map[string]string{
		"name":      namespacedName.Name,
		"namespace": namespacedName.Namespace,
	})
}

func refreshOnce(ctx context.Context, k8sClient client.Client) {
	logger.Debug("Clearing the metrics")
	authPolicyInfo.Reset()
	var authPolicyList v1alpha1.AuthPolicyList

	logger.Debug("Fetching AuthPolicies")
	_ = k8sClient.List(ctx, &authPolicyList)

	for _, authPolicy := range authPolicyList.Items {
		err := RefreshAuthPolicyInfo(ctx, k8sClient, authPolicy)
		if err != nil {
			logger.Error(
				err,
				"failed refreshing auth policy info",
				"namespace", authPolicy.Namespace,
				"name", authPolicy.Name,
			)
		}
	}
}
