package metrics

import "github.com/VictoriaMetrics/metrics"

var (
	statusEndpointErr = metrics.NewCounter("status_endpoint_error_total")
)

func IncStatusEndpointErr() {
	statusEndpointErr.Inc()
}
