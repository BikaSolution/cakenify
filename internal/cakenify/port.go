package cakenify

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// Service is the port the api layer talks to. All business logic
// (password hashing, ownership checks, digest building) lives behind it.
type Service interface {
	// Auth
	Login(ctx context.Context, username, password string) (*Session, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	SessionUser(ctx context.Context, sessionID uuid.UUID) (*User, error)

	// Admin
	SeedAdmin(ctx context.Context, username, password string) error
	CreateUser(ctx context.Context, params CreateUserParams) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)

	// Birthdays — all scoped to ownerID, callers never see another user's rows.
	CreateBirthday(ctx context.Context, ownerID uuid.UUID, params BirthdayParams) (*Birthday, error)
	ListBirthdays(ctx context.Context, ownerID uuid.UUID) ([]Birthday, error)
	Birthday(ctx context.Context, ownerID, id uuid.UUID) (*Birthday, error)
	UpdateBirthday(ctx context.Context, ownerID, id uuid.UUID, params BirthdayParams) (*Birthday, error)
	DeleteBirthday(ctx context.Context, ownerID, id uuid.UUID) error

	// Notification settings
	AvailableStrategies() []string
	UpsertNotificationSetting(ctx context.Context, userID uuid.UUID, strategy string, config json.RawMessage, enabled bool) (*NotificationSetting, error)
	ListNotificationSettings(ctx context.Context, userID uuid.UUID) ([]NotificationSetting, error)

	// SendTestNotification sends a canned message straight through the
	// given strategy's Notifier using config as-is — it never touches the
	// DB, so it works against unsaved form values too.
	SendTestNotification(ctx context.Context, strategy string, config json.RawMessage) error
}
