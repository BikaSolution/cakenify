package cakenify

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	Username  string
	IsAdmin   bool
	CreatedAt time.Time
}

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
}

type Birthday struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	PersonName string
	BirthMonth int
	BirthDay   int
	BirthYear  *int
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// BirthdayParams holds the user-editable fields of a Birthday, shared by
// create and update.
type BirthdayParams struct {
	PersonName string `validate:"required,max=200"`
	BirthMonth int    `validate:"required,min=1,max=12"`
	BirthDay   int    `validate:"required,min=1,max=31"`
	BirthYear  *int   `validate:"omitempty,min=1900,max=2100"`
	Notes      string `validate:"max=1000"`
}

type CreateUserParams struct {
	Username string `validate:"required,min=3,max=100"`
	// max=72: bcrypt hard-rejects longer inputs (golang.org/x/crypto/bcrypt
	// returns ErrPasswordTooLong past 72 bytes) — validating here gives a
	// clean error instead of a confusing failure surfacing from bcrypt.
	Password string `validate:"required,min=8,max=72"`
	IsAdmin  bool
}

type NotificationSetting struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	Strategy string
	Config   json.RawMessage
	Enabled  bool
}

// Message is what a Notifier actually sends.
type Message struct {
	Title string
	Body  string
}
