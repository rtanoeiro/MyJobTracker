package middlewares

import (
	"net/http"
)

// RequireHTMX guards state-changing requests against cross-site request forgery.
// Every mutation in the app is issued by HTMX, which always sends the
// "HX-Request: true" header. Browsers refuse to set custom headers on
// cross-origin requests without a CORS preflight, so requiring it blocks
// naive CSRF while leaving GET/HEAD/OPTIONS (and any HTMX call) untouched.
func RequireHTMX(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("HX-Request") != "true" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
