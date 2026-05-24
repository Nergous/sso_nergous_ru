package httpserver

import (
	"net/http"
	"strings"
)

const (
	corsAllowMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowHeaders = "Authorization, Content-Type, X-Request-Id, Grpc-Metadata-*"
	corsExposeHeader = "X-Request-Id"
	corsMaxAge       = "600" // 10m — bounds preflight TTL without being chatty
)

// corsMiddleware applies a permissive-but-explicit CORS policy. Origins are
// matched against allowedOrigins; "*" in that list acts as a wildcard but
// disables Access-Control-Allow-Credentials per the CORS spec. A nil/empty
// list means CORS is effectively off: the middleware still handles preflight
// for parity but never emits Allow-Origin.
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	wildcard := false
	exact := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "*" {
			wildcard = true
			continue
		}
		if o != "" {
			exact[o] = struct{}{}
		}
	}

	allow := func(origin string) (allowed bool, isWildcard bool) {
		if origin == "" {
			return false, false
		}
		if _, ok := exact[origin]; ok {
			return true, false
		}
		if wildcard {
			return true, true
		}
		return false, false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed, isWildcard := allow(origin)

			if allowed {
				if isWildcard {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					// Vary tells caches that the response depends on Origin,
					// so a CDN doesn't serve a response intended for origin A
					// to a request from origin B.
					w.Header().Add("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Expose-Headers", corsExposeHeader)
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if allowed {
					w.Header().Set("Access-Control-Allow-Methods", corsAllowMethods)
					if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
						w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
					} else {
						w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders)
					}
					w.Header().Set("Access-Control-Max-Age", corsMaxAge)
					w.WriteHeader(http.StatusNoContent)
					return
				}
				// Disallowed preflight: do not echo any Allow-* headers; the
				// browser will block the actual request. 204 keeps the wire
				// shape standard.
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
