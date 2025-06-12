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

	rpcNodeProxyClientErr = metrics.NewCounter("rpc_node_proxy_client_error_total")
	rpcNodeProxyServerErr = metrics.NewCounter("rpc_node_proxy_server_error_total")

	relayServerErr = metrics.NewCounter("relay_server_error_total")
	relayClientErr = metrics.NewCounter("relay_client_error_total")
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

// IncRPCNodeProxyClientErr increments client total errors counter when error caused on the client side/request building
func IncRPCNodeProxyClientErr() {
	rpcNodeProxyClientErr.Inc()
}

// IncRPCNodeProxyServerErr increments server total errors counter when error caused on the server/transport layer
func IncRPCNodeProxyServerErr() {
	rpcNodeProxyServerErr.Inc()
}

func IncRelayServerErr() {
	relayServerErr.Inc()
}

func IncRelayClientErr() {
	relayClientErr.Inc()
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
