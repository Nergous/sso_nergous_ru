package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// ReadinessFunc is called by the /readyz handler on every probe. A nil error
// means the process is ready to take traffic; any error becomes a 503 with
// the error text in the response body.
type ReadinessFunc func(ctx context.Context) error

// readinessTimeout caps how long a single probe can take. Probers (k8s,
// docker, etc.) hit /readyz on a tight schedule; a stuck DB ping should not
// stall the response indefinitely.
const readinessTimeout = 2 * time.Second

func healthzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
}

func readyzHandler(log *slog.Logger, readiness ReadinessFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		if readiness == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready\n"))
			return
		}

		if err := readiness(ctx); err != nil {
			log.WarnContext(r.Context(), "readyz: not ready",
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.Any("error", err),
			)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready: " + err.Error() + "\n"))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})
}

// metricsStubHandler is a placeholder until §5.3 wires up Prometheus. It
// answers 200 with a single comment line so a Prometheus scrape does not
// fail outright while the real registry is being plumbed in.
func metricsStubHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# metrics not yet wired (see TODO.md §5.3)\n"))
	})
}
