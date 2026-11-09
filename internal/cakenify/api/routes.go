package api

import (
	"crypto/rand"
	"io/fs"
	"net/http"
	"time"

	"cakenify/internal/cakenify"
	"cakenify/web"
)

// maxRequestBodyBytes caps every request body — our forms are tiny, this
// just closes off unbounded-body memory exhaustion.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

// Server is cakenify's HTTP layer — thin handlers that decode requests,
// call into the Service port, and render html/template pages or htmx
// fragments. secureCookies should be true whenever the app is reachable
// over https (i.e. always, once it's behind the Cloudflare tunnel).
type Server struct {
	service           cakenify.Service
	mux               *http.ServeMux
	secureCookies     bool
	csrfSecret        []byte
	loginLimiter      *rateLimiter
	testNotifyLimiter *rateLimiter
}

func NewServer(service cakenify.Service, secureCookies bool) *Server {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// crypto/rand failing means the platform's RNG is broken —
		// nothing safe to do but refuse to start.
		panic("cakenify: failed to generate CSRF secret: " + err.Error())
	}

	s := &Server{
		service:           service,
		secureCookies:     secureCookies,
		mux:               http.NewServeMux(),
		csrfSecret:        secret,
		loginLimiter:      newRateLimiter(10, time.Minute),
		testNotifyLimiter: newRateLimiter(5, time.Minute),
	}
	s.routes()

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	setSecurityHeaders(w, r, s.secureCookies)
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	staticFS, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		panic(err) // embedded at build time, can't fail on a working build
	}
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /logout", s.requireAuth(s.handleLogout))

	s.mux.HandleFunc("GET /{$}", s.requireAuth(s.handleDashboard))

	s.mux.HandleFunc("GET /birthdays", s.requireAuth(s.handleListBirthdays))
	s.mux.HandleFunc("POST /birthdays", s.requireAuth(s.handleCreateBirthday))
	s.mux.HandleFunc("GET /birthdays/{id}/edit", s.requireAuth(s.handleEditBirthdayForm))
	s.mux.HandleFunc("PUT /birthdays/{id}", s.requireAuth(s.handleUpdateBirthday))
	s.mux.HandleFunc("DELETE /birthdays/{id}", s.requireAuth(s.handleDeleteBirthday))

	s.mux.HandleFunc("GET /settings/notifications", s.requireAuth(s.handleNotificationSettingsPage))
	s.mux.HandleFunc("POST /settings/notifications", s.requireAuth(s.handleUpsertNotificationSetting))
	s.mux.HandleFunc("POST /settings/notifications/test", s.requireAuth(s.handleTestNotification))

	s.mux.HandleFunc("GET /admin/users", s.requireAuth(s.requireAdmin(s.handleAdminUsersPage)))
	s.mux.HandleFunc("POST /admin/users", s.requireAuth(s.requireAdmin(s.handleAdminCreateUser)))
}
