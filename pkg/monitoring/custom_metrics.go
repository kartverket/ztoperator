package monitoring

import "github.com/prometheus/client_golang/prometheus"

var (
	AuthPolicyInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "status",
			Namespace: "ztoperator",

		}
	)
)
