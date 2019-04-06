package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	prometheusHandler = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iot_cicd_requests_total",
			Help: "How many HTTP requests processed, partitioned by status code",
		},
		[]string{"code"},
	)
	promStatusOK          = prometheusHandler.WithLabelValues("200")
	promStatusFailedBuild = prometheusHandler.WithLabelValues("424")
)
