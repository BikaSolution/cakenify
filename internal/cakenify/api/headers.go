package api

import (
	"net/http"
	"strings"
)

// setSecurityHeaders applies a strict baseline to every response. The CSP
// has no 'unsafe-inline'/'unsafe-eval' — templates avoid inline
// style="..." (moved to CSS classes) and htmx's hx-on (which needs
// unsafe-eval) in favor of a small external listener in static/app.js.
func setSecurityHeaders(w http.ResponseWriter, r *http.Request, secureCookies bool) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
	h.Set("Content-Security-Policy", strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self'",
		"img-src 'self'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"object-src 'none'",
	}, "; "))

	if secureCookies {
		// Only meaningful (and only sent) once the app is actually served
		// over https, i.e. through the Cloudflare tunnel.
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	}

	if !strings.HasPrefix(r.URL.Path, "/static/") {
		h.Set("Cache-Control", "no-store")
	}
}
