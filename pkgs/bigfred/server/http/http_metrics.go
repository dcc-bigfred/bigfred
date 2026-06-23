package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/keskad/loco/pkgs/bigfred/server/metrics"
)

// MetricsMiddleware records HTTP request latency and volume with low-cardinality
// route patterns from chi.
func MetricsMiddleware(m *metrics.Metrics) func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}
			m.RecordHTTPRequest(route, r.Method, ww.Status(), time.Since(start))
		})
	}
}
