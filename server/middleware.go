package server

import (
	"net/http"

	"github.com/flashbots/rpc-endpoint/metrics"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		switch rec.status {
		case http.StatusOK:
			metrics.StatusOKInc()
		case http.StatusBadRequest:
			metrics.StatusBadRequestInc()
		case http.StatusInternalServerError:
			metrics.StatusInternalServerErrorInc()
		case http.StatusNotFound:
			metrics.StatusNotFoundInc()
		}
	})
}
