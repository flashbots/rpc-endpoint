package metrics

import "github.com/VictoriaMetrics/metrics"

var (
	privateTx = metrics.NewCounter("private_tx_total")
)

func IncPrivateTx() {
	privateTx.Inc()
}
