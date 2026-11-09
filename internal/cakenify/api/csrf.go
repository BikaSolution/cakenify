package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"

	"github.com/google/uuid"
)

// csrfToken derives a token from sessionID + the server's per-process
// secret via HMAC — no server-side token storage needed, and it's
// naturally invalidated on logout (session deleted) or process restart.
func (s *Server) csrfToken(sessionID uuid.UUID) string {
	mac := hmac.New(sha256.New, s.csrfSecret)
	mac.Write([]byte(sessionID.String()))

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// validCSRFToken checks the X-CSRF-Token header (set via htmx's hx-headers
// on <body>, for htmx-driven requests) or the csrf_token form field (for
// plain <form> submissions) against the expected token for sessionID.
func (s *Server) validCSRFToken(r *http.Request, sessionID uuid.UUID) bool {
	expected := s.csrfToken(sessionID)

	got := r.Header.Get("X-CSRF-Token")
	if got == "" {
		_ = r.ParseForm()
		got = r.FormValue("csrf_token")
	}

	return got != "" && hmac.Equal([]byte(got), []byte(expected))
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
