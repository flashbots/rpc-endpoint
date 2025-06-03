package metrics

import "github.com/VictoriaMetrics/metrics"

// we might want to use getOrCreate instead of newCounter to dynamically handle all http statused with less code
// used http statuses are limited (4) so it is safer and better in terms of performance
var (
	statusOK                  = metrics.NewCounter(`http_requests_total{status="200"}`)
	statusBadRequest          = metrics.NewCounter(`http_requests_total{status="400"}`)
	statusNotFound            = metrics.NewCounter(`http_requests_total{status="404"}`)
	statusInternalServerError = metrics.NewCounter(`http_requests_total{status="500"}`)
)

func StatusOKInc()                  { statusOK.Inc() }
func StatusBadRequestInc()          { statusBadRequest.Inc() }
func StatusNotFoundInc()            { statusNotFound.Inc() }
func StatusInternalServerErrorInc() { statusInternalServerError.Inc() }
