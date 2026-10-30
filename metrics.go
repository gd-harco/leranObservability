package main

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var httpRequestCounter = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "the total number of corresponding http request",
	},
	[]string{"method", "path", "status"},
)

func recordHTTPMetrics(r *http.Request, statusCode int) {
	path := r.URL.Path
	if path == "" {
		path = "unmatched"
	}
	httpRequestCounter.WithLabelValues(r.Method, path, strconv.Itoa(statusCode)).Inc()
}
