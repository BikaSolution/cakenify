package api

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"

	"cakenify/internal/cakenify"
	"cakenify/web"
)

// pageData is what every full-page template receives.
type pageData struct {
	Title     string
	User      *cakenify.User
	CSRFToken string
	Error     string
	Notice    string
	Data      any
}

var (
	pageTemplates     map[string]*template.Template
	fragmentTemplates map[string]*template.Template
)

// pages lists every full page (rendered on top of layout.html) alongside
// any fragment files it embeds directly (e.g. dashboard needs the
// birthday list/form fragments to render on first load).
var pages = map[string][]string{
	"login.html":         {},
	"dashboard.html":     {"birthdays_list.html", "birthday_form.html"},
	"notifications.html": {},
	"admin_users.html":   {},
}

var fragments = []string{"birthdays_list.html", "birthday_form.html"}

func init() {
	pageTemplates = make(map[string]*template.Template, len(pages))
	for page, extra := range pages {
		files := append([]string{"templates/layout.html", "templates/" + page}, prefixAll(extra)...)
		pageTemplates[page] = template.Must(template.New("layout.html").ParseFS(web.TemplatesFS, files...))
	}

	fragmentTemplates = make(map[string]*template.Template, len(fragments))
	for _, f := range fragments {
		fragmentTemplates[f] = template.Must(template.New(f).ParseFS(web.TemplatesFS, "templates/"+f))
	}
}

func prefixAll(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "templates/" + n
	}

	return out
}

func renderPage(w http.ResponseWriter, page string, data pageData) {
	renderPageStatus(w, http.StatusOK, page, data)
}

func renderPageStatus(w http.ResponseWriter, status int, page string, data pageData) {
	tmpl, ok := pageTemplates[page]
	if !ok {
		slog.Error("render: unknown page template", "page", page)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("render: failed to execute page template", "page", page, "error", err)
	}
}

// renderFragment renders an htmx partial identified by its defined
// template name (e.g. "birthdays_list"), looked up in the file of the
// same name (birthdays_list.html).
func renderFragment(w http.ResponseWriter, file, definedName string, data any) {
	tmpl, ok := fragmentTemplates[file]
	if !ok {
		slog.Error("render: unknown fragment template", "file", file)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, definedName, data); err != nil {
		slog.Error("render: failed to execute fragment template", "file", file, "error", err)
	}
}

// errorStatus maps a sentinel error to the HTTP status that best fits it —
// the local stand-in for bash-checkout-services-go's experror-embedded
// status, without pulling in that private package.
func errorStatus(err error) int {
	switch {
	case errors.Is(err, cakenify.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, cakenify.ErrInvalidParameters):
		return http.StatusBadRequest
	case errors.Is(err, cakenify.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, cakenify.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, cakenify.ErrUsernameTaken):
		return http.StatusConflict
	case errors.Is(err, cakenify.ErrUnknownStrategy):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	slog.Error("request failed", "operation", operation, "error", err)
	renderInlineError(w, errorStatus(err), "something went wrong")
}

// renderInlineError writes a small styled error snippet instead of a bare
// text/plain body, since responses can land inside an htmx swap target as
// easily as a full page.
func renderInlineError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`<div class="error">` + template.HTMLEscapeString(message) + `</div>`))
}
