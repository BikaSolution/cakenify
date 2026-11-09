package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"cakenify/internal/cakenify"
)

const sessionCookieName = "cakenify_session"

type contextKey string

const authContextKey contextKey = "auth"

// authContext is everything a request needs once requireAuth has resolved
// the session: who's making the request, which session it is (for
// logout), and that session's CSRF token (for rendering into forms).
type authContext struct {
	User      *cakenify.User
	SessionID uuid.UUID
	CSRFToken string
}

func (s *Server) currentUser(r *http.Request) (*cakenify.User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, cakenify.ErrUnauthorized
	}

	sessionID, err := uuid.Parse(cookie.Value)
	if err != nil {
		return nil, cakenify.ErrUnauthorized
	}

	return s.service.SessionUser(r.Context(), sessionID)
}

// requireAuth redirects to /login unless a valid session cookie is
// present, stashing the resolved user/session/CSRF token in the request
// context for handlers/requireAdmin. For state-changing methods it also
// enforces the CSRF token — SameSite=Lax cookies already block most CSRF,
// this is defense in depth.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		sessionID, err := uuid.Parse(cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		user, err := s.service.SessionUser(r.Context(), sessionID)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if isUnsafeMethod(r.Method) && !s.validCSRFToken(r, sessionID) {
			renderInlineError(w, http.StatusForbidden, "your session looks out of date — please refresh the page and try again")
			return
		}

		ctx := context.WithValue(r.Context(), authContextKey, &authContext{
			User:      user,
			SessionID: sessionID,
			CSRFToken: s.csrfToken(sessionID),
		})
		next(w, r.WithContext(ctx))
	}
}

// requireAdmin must be chained inside requireAuth (s.requireAuth(s.requireAdmin(handler))).
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())
		if user == nil || !user.IsAdmin {
			renderInlineError(w, http.StatusForbidden, "admins only")
			return
		}

		next(w, r)
	}
}

func authFromContext(ctx context.Context) *authContext {
	a, _ := ctx.Value(authContextKey).(*authContext)
	return a
}

func userFromContext(ctx context.Context) *cakenify.User {
	if a := authFromContext(ctx); a != nil {
		return a.User
	}
	return nil
}

func sessionIDFromContext(ctx context.Context) uuid.UUID {
	if a := authFromContext(ctx); a != nil {
		return a.SessionID
	}
	return uuid.Nil
}

func csrfTokenFromContext(ctx context.Context) string {
	if a := authFromContext(ctx); a != nil {
		return a.CSRFToken
	}
	return ""
}
