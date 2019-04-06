package server

import "github.com/prometheus/client_golang/prometheus"

var (
	prometheusHandler = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "How many HTTP requests processed, partitioned by status code",
		},
		[]string{"code"},
	)
	promStatusOK       = prometheusHandler.WithLabelValues("200")
	promStatusNotFound = prometheusHandler.WithLabelValues("404")
)
