package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// RequestsTotal counts total tool requests categorized by status, matched rule, and tool name.
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gatra_requests_total",
			Help: "Total number of evaluated tool execution requests processed by GATRA",
		},
		[]string{"status", "rule_id", "tool_name"},
	)

	// EvaluationDuration tracks proxy policy evaluation latency histograms in seconds.
	EvaluationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gatra_evaluation_duration_seconds",
			Help:    "Latency of proxy policy evaluation pipeline in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)
)

func init() {
	prometheus.MustRegister(RequestsTotal)
	prometheus.MustRegister(EvaluationDuration)
}

// Handler returns the HTTP handler for the /metrics Prometheus scraping endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}