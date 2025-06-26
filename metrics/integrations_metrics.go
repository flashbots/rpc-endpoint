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

func customerConfigKey(customer string) string {
	return fmt.Sprintf(`customer_configuration_was_updated_total{customer="%s"}`, customer)
}

func InitCustomersConfigMetric(customers ...string) {
	for _, c := range customers {
		// just initialize metrics record
		metrics.GetOrCreateCounter(customerConfigKey(c))
	}
}

func ReportCustomerConfigWasUpdated(customer string) {
	metrics.GetOrCreateCounter(customerConfigKey(customer)).Inc()
}
