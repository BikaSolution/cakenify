package cakenifydb

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DB is the persistence port for cakenify. A single implementation backs
// the whole app (there's only one service here, unlike a multi-service
// monorepo where each service gets its own db package).
type DB interface {
	// Users
	CreateUser(ctx context.Context, params CreateUserParams) (*User, error)
	UserByUsername(ctx context.Context, username string) (*User, error)
	UserByID(ctx context.Context, id uuid.UUID) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)
	CountUsers(ctx context.Context) (int, error)

	// Sessions
	CreateSession(ctx context.Context, params CreateSessionParams) (*Session, error)
	SessionByID(ctx context.Context, id uuid.UUID) (*Session, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error

	// Birthdays
	CreateBirthday(ctx context.Context, params CreateBirthdayParams) (*Birthday, error)
	BirthdaysByUser(ctx context.Context, userID uuid.UUID) ([]Birthday, error)
	Birthday(ctx context.Context, id uuid.UUID) (*Birthday, error)
	UpdateBirthday(ctx context.Context, id uuid.UUID, params UpdateBirthdayParams) (*Birthday, error)
	DeleteBirthday(ctx context.Context, id uuid.UUID) error
	BirthdaysForMonthDays(ctx context.Context, days []MonthDay) ([]Birthday, error)

	// Notification settings
	UpsertNotificationSetting(ctx context.Context, params UpsertNotificationSettingParams) (*NotificationSetting, error)
	NotificationSettingsByUser(ctx context.Context, userID uuid.UUID) ([]NotificationSetting, error)
	EnabledNotificationSettingsByUser(ctx context.Context, userID uuid.UUID) ([]NotificationSetting, error)

	// Scheduler state — guards the daily/weekly jobs against double-sending on restart.
	SchedulerLastRunDate(ctx context.Context, jobName string) (*time.Time, error)
	SetSchedulerLastRunDate(ctx context.Context, jobName string, date time.Time) error

	Close()
}
