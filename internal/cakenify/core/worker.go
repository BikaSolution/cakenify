package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"cakenify/internal/cakenify"
	"cakenify/pkg/db/cakenifydb"
)

const (
	jobDailyReminder = "daily_reminder"
	jobWeeklyDigest  = "weekly_digest"
	reminderHour     = 10
)

// Worker holds the daily/Sunday scheduler logic run by pkg/cronjob. It's
// independent of the Service port — main.go wires it up alongside the HTTP
// server, driven by its own Tick call rather than through api/ handlers.
type Worker struct {
	db        cakenifydb.DB
	notifiers cakenify.NotifierRegistry
	location  *time.Location
	now       func() time.Time
}

func NewWorker(db cakenifydb.DB, notifiers cakenify.NotifierRegistry, location *time.Location) *Worker {
	return &Worker{db: db, notifiers: notifiers, location: location, now: time.Now}
}

// Tick is called on a fixed interval (see pkg/cronjob). It's a no-op
// outside the 10:00 hour and each job is guarded by scheduler_state so a
// restart within the 10:00 window can't double-send.
func (w *Worker) Tick(ctx context.Context) {
	now := w.now().In(w.location)
	if now.Hour() != reminderHour {
		return
	}

	w.runOnce(ctx, jobDailyReminder, now, w.sendDailyReminders)

	if now.Weekday() == time.Sunday {
		w.runOnce(ctx, jobWeeklyDigest, now, w.sendWeeklyDigest)
	}
}

func (w *Worker) runOnce(ctx context.Context, jobName string, now time.Time, run func(ctx context.Context, now time.Time) error) {
	last, err := w.db.SchedulerLastRunDate(ctx, jobName)
	if err != nil {
		slog.Error("worker: failed to read scheduler state", "job", jobName, "error", err)
		return
	}
	if last != nil && sameDate(*last, now) {
		return
	}

	if err := run(ctx, now); err != nil {
		slog.Error("worker: job failed", "job", jobName, "error", err)
		return
	}

	if err := w.db.SetSchedulerLastRunDate(ctx, jobName, now); err != nil {
		slog.Error("worker: failed to record scheduler state", "job", jobName, "error", err)
	}
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// sendDailyReminders notifies each owner with a birthday today, body is
// just the names.
func (w *Worker) sendDailyReminders(ctx context.Context, now time.Time) error {
	birthdays, err := w.db.BirthdaysForMonthDays(ctx, []cakenifydb.MonthDay{{Month: int(now.Month()), Day: now.Day()}})
	if err != nil {
		return err
	}

	byOwner := map[uuid.UUID][]string{}
	for _, b := range birthdays {
		byOwner[b.UserID] = append(byOwner[b.UserID], b.PersonName)
	}

	for ownerID, names := range byOwner {
		w.notifyOwner(ctx, ownerID, cakenify.Message{
			Title: "Birthday today",
			Body:  strings.Join(names, ", "),
		})
	}

	return nil
}

type weekdayBirthdays struct {
	Weekday string
	Names   []string
}

// sendWeeklyDigest notifies each owner with birthdays in the coming Monday
// through Sunday (now is a Sunday, so that's now+1 .. now+7), one line per
// weekday that has a birthday.
func (w *Worker) sendWeeklyDigest(ctx context.Context, now time.Time) error {
	type day struct {
		monthDay cakenifydb.MonthDay
		weekday  string
	}

	days := make([]day, 0, 7)
	for i := 1; i <= 7; i++ {
		d := now.AddDate(0, 0, i)
		days = append(days, day{
			monthDay: cakenifydb.MonthDay{Month: int(d.Month()), Day: d.Day()},
			weekday:  d.Weekday().String(),
		})
	}

	monthDays := make([]cakenifydb.MonthDay, len(days))
	for i, d := range days {
		monthDays[i] = d.monthDay
	}

	birthdays, err := w.db.BirthdaysForMonthDays(ctx, monthDays)
	if err != nil {
		return err
	}

	type ownerWeek struct {
		order []string
		names map[string][]string
	}
	byOwner := map[uuid.UUID]*ownerWeek{}

	for _, d := range days {
		for _, b := range birthdays {
			if b.BirthMonth != d.monthDay.Month || b.BirthDay != d.monthDay.Day {
				continue
			}

			ow, ok := byOwner[b.UserID]
			if !ok {
				ow = &ownerWeek{names: make(map[string][]string)}
				byOwner[b.UserID] = ow
			}
			if _, seen := ow.names[d.weekday]; !seen {
				ow.order = append(ow.order, d.weekday)
			}
			ow.names[d.weekday] = append(ow.names[d.weekday], b.PersonName)
		}
	}

	for ownerID, ow := range byOwner {
		entries := make([]weekdayBirthdays, 0, len(ow.order))
		for _, weekday := range ow.order {
			entries = append(entries, weekdayBirthdays{Weekday: weekday, Names: ow.names[weekday]})
		}

		w.notifyOwner(ctx, ownerID, cakenify.Message{
			Title: "Birthdays this week",
			Body:  buildWeeklyDigestBody(entries),
		})
	}

	return nil
}

func buildWeeklyDigestBody(entries []weekdayBirthdays) string {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%s: %s.", e.Weekday, strings.Join(e.Names, ", ")))
	}

	return strings.Join(lines, "\n")
}

func (w *Worker) notifyOwner(ctx context.Context, ownerID uuid.UUID, msg cakenify.Message) {
	settings, err := w.db.EnabledNotificationSettingsByUser(ctx, ownerID)
	if err != nil {
		slog.Error("worker: failed to load notification settings", "owner", ownerID, "error", err)
		return
	}

	for _, setting := range settings {
		notifier, ok := w.notifiers[setting.Strategy]
		if !ok {
			slog.Warn("worker: unknown notification strategy", "strategy", setting.Strategy, "owner", ownerID)
			continue
		}

		if err := notifier.Send(ctx, setting.Config, msg); err != nil {
			slog.Error("worker: failed to send notification", "owner", ownerID, "strategy", setting.Strategy, "error", err)
		}
	}
}
