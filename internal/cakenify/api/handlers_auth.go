package api

import (
	"net/http"

	"github.com/google/uuid"
)

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, err := s.currentUser(r); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	renderPage(w, "login.html", pageData{Title: "Log in"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPage(w, "login.html", pageData{Title: "Log in", Error: "invalid form submission"})
		return
	}

	username := r.FormValue("username")

	// Check (and record against) both buckets regardless of short-circuiting,
	// so a tripped IP limit doesn't leave the per-username bucket uncounted.
	ipAllowed := s.loginLimiter.Allow(clientIP(r))
	userAllowed := s.loginLimiter.Allow("user:" + username)
	if !ipAllowed || !userAllowed {
		renderPageStatus(w, http.StatusTooManyRequests, "login.html", pageData{
			Title: "Log in",
			Error: "too many attempts — please wait a minute and try again",
		})
		return
	}

	session, err := s.service.Login(r.Context(), username, r.FormValue("password"))
	if err != nil {
		renderPage(w, "login.html", pageData{Title: "Log in", Error: "invalid username or password"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID.String(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies,
		Expires:  session.ExpiresAt,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sessionID := sessionIDFromContext(r.Context()); sessionID != uuid.Nil {
		_ = s.service.Logout(r.Context(), sessionID)
	}

	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
