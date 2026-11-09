package api

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"cakenify/internal/cakenify"
)

// birthdayView adds display-only fields (month name, turning-age) on top
// of the domain type, computed here so templates stay dumb.
type birthdayView struct {
	ID         uuid.UUID
	PersonName string
	MonthName  string
	Day        int
	Year       *int
	Notes      string
	TurningAge *int
}

func toBirthdayView(b cakenify.Birthday, now time.Time) birthdayView {
	v := birthdayView{
		ID:         b.ID,
		PersonName: b.PersonName,
		MonthName:  time.Month(b.BirthMonth).String(),
		Day:        b.BirthDay,
		Year:       b.BirthYear,
		Notes:      b.Notes,
	}

	if b.BirthYear != nil {
		nextYear := now.Year()
		if hasPassedThisYear(b.BirthMonth, b.BirthDay, now) {
			nextYear++
		}
		age := nextYear - *b.BirthYear
		v.TurningAge = &age
	}

	return v
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func hasPassedThisYear(month, day int, now time.Time) bool {
	today := truncateDay(now)
	occurrence := time.Date(today.Year(), time.Month(month), day, 0, 0, 0, 0, today.Location())
	return occurrence.Before(today)
}

func daysUntilNext(month, day int, now time.Time) int {
	today := truncateDay(now)
	occurrence := time.Date(today.Year(), time.Month(month), day, 0, 0, 0, 0, today.Location())
	if occurrence.Before(today) {
		occurrence = time.Date(today.Year()+1, time.Month(month), day, 0, 0, 0, 0, today.Location())
	}

	return int(occurrence.Sub(today).Hours() / 24)
}

// sortBirthdaysByUpcoming orders birthdays by soonest next occurrence first.
func sortBirthdaysByUpcoming(birthdays []cakenify.Birthday, now time.Time) {
	sort.Slice(birthdays, func(i, j int) bool {
		return daysUntilNext(birthdays[i].BirthMonth, birthdays[i].BirthDay, now) <
			daysUntilNext(birthdays[j].BirthMonth, birthdays[j].BirthDay, now)
	})
}

func toBirthdayViews(birthdays []cakenify.Birthday, now time.Time) []birthdayView {
	views := make([]birthdayView, 0, len(birthdays))
	for _, b := range birthdays {
		views = append(views, toBirthdayView(b, now))
	}

	return views
}

type monthOption struct {
	Value    int
	Name     string
	Selected bool
}

func monthOptions(selected int) []monthOption {
	options := make([]monthOption, 0, 12)
	for m := 1; m <= 12; m++ {
		options = append(options, monthOption{Value: m, Name: time.Month(m).String(), Selected: m == selected})
	}

	return options
}

// Page/fragment data shapes — these are what the templates' .Data resolves to.

type birthdaysListView struct {
	Birthdays []birthdayView
}

type birthdayFormView struct {
	Birthday *birthdayView
	Months   []monthOption
}

func newBirthdayFormView() birthdayFormView {
	return birthdayFormView{Months: monthOptions(0)}
}

func editBirthdayFormView(b cakenify.Birthday, now time.Time) birthdayFormView {
	view := toBirthdayView(b, now)
	return birthdayFormView{Birthday: &view, Months: monthOptions(b.BirthMonth)}
}

type dashboardData struct {
	Form birthdayFormView
	List birthdaysListView
}

type notificationsData struct {
	NtfyTopic    string
	NtfyHasToken bool // whether a token is already saved — its value is never sent back to the browser
	NtfyEnabled  bool
}

type adminUsersData struct {
	Users []cakenify.User
}
