package api

import (
	"errors"
	"net/http"

	"cakenify/internal/cakenify"
)

func (s *Server) handleAdminUsersPage(w http.ResponseWriter, r *http.Request) {
	s.renderAdminUsersPage(w, r, userFromContext(r.Context()), "")
}

func (s *Server) renderAdminUsersPage(w http.ResponseWriter, r *http.Request, user *cakenify.User, errMsg string) {
	users, err := s.service.ListUsers(r.Context())
	if err != nil {
		s.renderError(w, r, "list_users", err)
		return
	}

	renderPage(w, "admin_users.html", pageData{
		Title: "Users", User: user, CSRFToken: csrfTokenFromContext(r.Context()),
		Error: errMsg, Data: adminUsersData{Users: users},
	})
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		s.renderAdminUsersPage(w, r, user, "invalid form submission")
		return
	}

	params := cakenify.CreateUserParams{
		Username: r.FormValue("username"),
		Password: r.FormValue("password"),
		IsAdmin:  r.FormValue("is_admin") == "on",
	}

	if _, err := s.service.CreateUser(r.Context(), params); err != nil {
		msg := "could not create user"
		switch {
		case errors.Is(err, cakenify.ErrUsernameTaken):
			msg = "username already taken"
		case errors.Is(err, cakenify.ErrInvalidParameters):
			msg = "invalid username or password (username min 3 chars, password min 8 chars)"
		}
		s.renderAdminUsersPage(w, r, user, msg)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}
