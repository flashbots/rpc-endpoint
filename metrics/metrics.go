package metrics

import (
	"net/http"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

var (
	databaseErr = metrics.NewCounter("postgres_error_total")
	redisErr    = metrics.NewCounter("redis_error_total")
	ethNodeErr  = metrics.NewCounter("eth_node_cluster_error_total")
)

func IncDatabaseErr() {
	databaseErr.Inc()
}

func IncRedisErr() {
	redisErr.Inc()
}

func IncEthNodeClusterErr() {
	ethNodeErr.Inc()
}

func DefaultServer(addr string) *http.Server {
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.WritePrometheus(w, true)
	})

	return &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		Handler:           metricsMux,
	}
}
