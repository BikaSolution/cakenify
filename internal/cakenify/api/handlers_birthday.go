package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"cakenify/internal/cakenify"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	birthdays, err := s.service.ListBirthdays(r.Context(), user.ID)
	if err != nil {
		s.renderError(w, r, "list_birthdays", err)
		return
	}

	now := time.Now()
	sortBirthdaysByUpcoming(birthdays, now)

	renderPage(w, "dashboard.html", pageData{
		Title:     "Dashboard",
		User:      user,
		CSRFToken: csrfTokenFromContext(r.Context()),
		Data: dashboardData{
			Form: newBirthdayFormView(),
			List: birthdaysListView{Birthdays: toBirthdayViews(birthdays, now)},
		},
	})
}

func (s *Server) handleListBirthdays(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	birthdays, err := s.service.ListBirthdays(r.Context(), user.ID)
	if err != nil {
		s.renderError(w, r, "list_birthdays", err)
		return
	}

	now := time.Now()
	sortBirthdaysByUpcoming(birthdays, now)

	renderFragment(w, "birthdays_list.html", "birthdays_list", birthdaysListView{Birthdays: toBirthdayViews(birthdays, now)})
}

func (s *Server) handleCreateBirthday(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	params, err := parseBirthdayForm(r)
	if err != nil {
		renderInlineError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := s.service.CreateBirthday(r.Context(), user.ID, params); err != nil {
		s.renderError(w, r, "create_birthday", err)
		return
	}

	s.handleListBirthdays(w, r)
}

func (s *Server) handleEditBirthdayForm(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		renderInlineError(w, http.StatusBadRequest, "invalid birthday id")
		return
	}

	b, err := s.service.Birthday(r.Context(), user.ID, id)
	if err != nil {
		s.renderError(w, r, "get_birthday", err)
		return
	}

	renderFragment(w, "birthday_form.html", "birthday_form", editBirthdayFormView(*b, time.Now()))
}

func (s *Server) handleUpdateBirthday(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		renderInlineError(w, http.StatusBadRequest, "invalid birthday id")
		return
	}

	params, err := parseBirthdayForm(r)
	if err != nil {
		renderInlineError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := s.service.UpdateBirthday(r.Context(), user.ID, id, params); err != nil {
		s.renderError(w, r, "update_birthday", err)
		return
	}

	s.handleListBirthdays(w, r)
}

func (s *Server) handleDeleteBirthday(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		renderInlineError(w, http.StatusBadRequest, "invalid birthday id")
		return
	}

	if err := s.service.DeleteBirthday(r.Context(), user.ID, id); err != nil {
		s.renderError(w, r, "delete_birthday", err)
		return
	}

	s.handleListBirthdays(w, r)
}

func parseBirthdayForm(r *http.Request) (cakenify.BirthdayParams, error) {
	if err := r.ParseForm(); err != nil {
		return cakenify.BirthdayParams{}, errors.New("invalid form submission")
	}

	month, err := strconv.Atoi(r.FormValue("birth_month"))
	if err != nil {
		return cakenify.BirthdayParams{}, errors.New("invalid month")
	}

	day, err := strconv.Atoi(r.FormValue("birth_day"))
	if err != nil {
		return cakenify.BirthdayParams{}, errors.New("invalid day")
	}

	var year *int
	if raw := r.FormValue("birth_year"); raw != "" {
		y, err := strconv.Atoi(raw)
		if err != nil {
			return cakenify.BirthdayParams{}, errors.New("invalid year")
		}
		year = &y
	}

	return cakenify.BirthdayParams{
		PersonName: r.FormValue("person_name"),
		BirthMonth: month,
		BirthDay:   day,
		BirthYear:  year,
		Notes:      r.FormValue("notes"),
	}, nil
}
