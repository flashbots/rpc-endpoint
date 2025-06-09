package metrics

import (
	"fmt"

	"github.com/VictoriaMetrics/metrics"
)

var (
	metamaskInterceptorWrongNonce      = metrics.NewCounter("metamask_interceptor_wrong_nonce_total")
	uniswapInterceptorNonceDiffTooHigh = metrics.NewCounter("uniswap_interceptor_nonce_diff_too_high_total")
)

func IncMetamaskInterceptorWrongNonce() {
	metamaskInterceptorWrongNonce.Inc()
}

func IncUniswapInterceptorNonceDiffTooHigh() {
	uniswapInterceptorNonceDiffTooHigh.Inc()
}

func ReportCustomerConfigWasUpdated(customer string) {
	metrics.GetOrCreateCounter(fmt.Sprintf(`customer_configuration_was_updated_total{customer="%s"}`, customer)).Inc()
}
