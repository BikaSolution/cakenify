package core

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"cakenify/internal/cakenify"
	"cakenify/pkg/db/cakenifydb"
)

var validate = validator.New()

// dummyPasswordHash lets Login always run a bcrypt compare, even for a
// username that doesn't exist. Skipping bcrypt on a DB miss would make an
// unknown-username response measurably faster than a wrong-password one,
// letting an attacker enumerate valid usernames purely by timing.
var dummyPasswordHash = mustBcryptHash("cakenify-timing-safety-dummy-password")

func mustBcryptHash(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	return hash
}

type service struct {
	db         cakenifydb.DB
	notifiers  cakenify.NotifierRegistry
	sessionTTL time.Duration
	now        func() time.Time
}

// New returns a cakenify.Service backed by db and the given notifier registry.
func New(db cakenifydb.DB, notifiers cakenify.NotifierRegistry, sessionTTL time.Duration) cakenify.Service {
	return &service{
		db:         db,
		notifiers:  notifiers,
		sessionTTL: sessionTTL,
		now:        time.Now,
	}
}

// Auth

func (s *service) Login(ctx context.Context, username, password string) (*cakenify.Session, error) {
	user, err := s.db.UserByUsername(ctx, username)
	if err != nil && !errors.Is(err, cakenifydb.ErrNotFound) {
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	hash := dummyPasswordHash
	if user != nil {
		hash = []byte(user.PasswordHash)
	}

	// Always compare, even against the dummy hash for an unknown user, so
	// a nonexistent username and a wrong password take the same amount of
	// time to reject.
	compareErr := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if user == nil || compareErr != nil {
		return nil, cakenify.ErrInvalidCredentials
	}

	session, err := s.db.CreateSession(ctx, cakenifydb.CreateSessionParams{
		ID:        uuid.New(),
		UserID:    user.ID,
		ExpiresAt: s.now().Add(s.sessionTTL),
	})
	if err != nil {
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	return toSession(*session), nil
}

func (s *service) Logout(ctx context.Context, sessionID uuid.UUID) error {
	if err := s.db.DeleteSession(ctx, sessionID); err != nil && !errors.Is(err, cakenifydb.ErrNotFound) {
		return errors.Join(cakenify.ErrInternal, err)
	}

	return nil
}

func (s *service) SessionUser(ctx context.Context, sessionID uuid.UUID) (*cakenify.User, error) {
	session, err := s.db.SessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, cakenifydb.ErrNotFound) {
			return nil, cakenify.ErrUnauthorized
		}
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	if session.ExpiresAt.Before(s.now()) {
		_ = s.db.DeleteSession(ctx, sessionID)
		return nil, cakenify.ErrUnauthorized
	}

	user, err := s.db.UserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, cakenifydb.ErrNotFound) {
			return nil, cakenify.ErrUnauthorized
		}
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	return toUser(*user), nil
}

// Admin

func (s *service) SeedAdmin(ctx context.Context, username, password string) error {
	count, err := s.db.CountUsers(ctx)
	if err != nil {
		return errors.Join(cakenify.ErrInternal, err)
	}
	if count > 0 {
		return nil
	}

	_, err = s.CreateUser(ctx, cakenify.CreateUserParams{Username: username, Password: password, IsAdmin: true})

	return err
}

func (s *service) CreateUser(ctx context.Context, params cakenify.CreateUserParams) (*cakenify.User, error) {
	if err := validate.StructCtx(ctx, params); err != nil {
		return nil, errors.Join(cakenify.ErrInvalidParameters, err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	user, err := s.db.CreateUser(ctx, cakenifydb.CreateUserParams{
		ID:           uuid.New(),
		Username:     params.Username,
		PasswordHash: string(hash),
		IsAdmin:      params.IsAdmin,
	})
	if err != nil {
		if errors.Is(err, cakenifydb.ErrAlreadyExists) {
			return nil, cakenify.ErrUsernameTaken
		}
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	return toUser(*user), nil
}

func (s *service) ListUsers(ctx context.Context) ([]cakenify.User, error) {
	users, err := s.db.ListUsers(ctx)
	if err != nil {
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	out := make([]cakenify.User, 0, len(users))
	for _, u := range users {
		out = append(out, *toUser(u))
	}

	return out, nil
}

// Birthdays

func (s *service) CreateBirthday(ctx context.Context, ownerID uuid.UUID, params cakenify.BirthdayParams) (*cakenify.Birthday, error) {
	if err := validate.StructCtx(ctx, params); err != nil {
		return nil, errors.Join(cakenify.ErrInvalidParameters, err)
	}

	b, err := s.db.CreateBirthday(ctx, cakenifydb.CreateBirthdayParams{
		ID:         uuid.New(),
		UserID:     ownerID,
		PersonName: params.PersonName,
		BirthMonth: params.BirthMonth,
		BirthDay:   params.BirthDay,
		BirthYear:  params.BirthYear,
		Notes:      params.Notes,
	})
	if err != nil {
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	return toBirthday(*b), nil
}

func (s *service) ListBirthdays(ctx context.Context, ownerID uuid.UUID) ([]cakenify.Birthday, error) {
	birthdays, err := s.db.BirthdaysByUser(ctx, ownerID)
	if err != nil {
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	out := make([]cakenify.Birthday, 0, len(birthdays))
	for _, b := range birthdays {
		out = append(out, *toBirthday(b))
	}

	return out, nil
}

func (s *service) Birthday(ctx context.Context, ownerID, id uuid.UUID) (*cakenify.Birthday, error) {
	b, err := s.ownedBirthday(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}

	return toBirthday(*b), nil
}

func (s *service) UpdateBirthday(ctx context.Context, ownerID, id uuid.UUID, params cakenify.BirthdayParams) (*cakenify.Birthday, error) {
	if _, err := s.ownedBirthday(ctx, ownerID, id); err != nil {
		return nil, err
	}
	if err := validate.StructCtx(ctx, params); err != nil {
		return nil, errors.Join(cakenify.ErrInvalidParameters, err)
	}

	b, err := s.db.UpdateBirthday(ctx, id, cakenifydb.UpdateBirthdayParams{
		PersonName: params.PersonName,
		BirthMonth: params.BirthMonth,
		BirthDay:   params.BirthDay,
		BirthYear:  params.BirthYear,
		Notes:      params.Notes,
	})
	if err != nil {
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	return toBirthday(*b), nil
}

func (s *service) DeleteBirthday(ctx context.Context, ownerID, id uuid.UUID) error {
	if _, err := s.ownedBirthday(ctx, ownerID, id); err != nil {
		return err
	}

	if err := s.db.DeleteBirthday(ctx, id); err != nil {
		return errors.Join(cakenify.ErrInternal, err)
	}

	return nil
}

// ownedBirthday fetches a birthday and verifies ownerID owns it, returning
// ErrNotFound (not ErrForbidden) on mismatch so we don't leak existence of
// other users' rows.
func (s *service) ownedBirthday(ctx context.Context, ownerID, id uuid.UUID) (*cakenifydb.Birthday, error) {
	b, err := s.db.Birthday(ctx, id)
	if err != nil {
		if errors.Is(err, cakenifydb.ErrNotFound) {
			return nil, cakenify.ErrNotFound
		}
		return nil, errors.Join(cakenify.ErrInternal, err)
	}
	if b.UserID != ownerID {
		return nil, cakenify.ErrNotFound
	}

	return b, nil
}

// Notification settings

func (s *service) AvailableStrategies() []string {
	return s.notifiers.Strategies()
}

func (s *service) UpsertNotificationSetting(ctx context.Context, userID uuid.UUID, strategy string, config json.RawMessage, enabled bool) (*cakenify.NotificationSetting, error) {
	if _, ok := s.notifiers[strategy]; !ok {
		return nil, cakenify.ErrUnknownStrategy
	}

	setting, err := s.db.UpsertNotificationSetting(ctx, cakenifydb.UpsertNotificationSettingParams{
		ID:       uuid.New(),
		UserID:   userID,
		Strategy: strategy,
		Config:   config,
		Enabled:  enabled,
	})
	if err != nil {
		if errors.Is(err, cakenifydb.ErrInvalidParams) {
			return nil, errors.Join(cakenify.ErrInvalidParameters, err)
		}
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	return toNotificationSetting(*setting), nil
}

func (s *service) ListNotificationSettings(ctx context.Context, userID uuid.UUID) ([]cakenify.NotificationSetting, error) {
	settings, err := s.db.NotificationSettingsByUser(ctx, userID)
	if err != nil {
		return nil, errors.Join(cakenify.ErrInternal, err)
	}

	out := make([]cakenify.NotificationSetting, 0, len(settings))
	for _, st := range settings {
		out = append(out, *toNotificationSetting(st))
	}

	return out, nil
}

func (s *service) SendTestNotification(ctx context.Context, strategy string, config json.RawMessage) error {
	notifier, ok := s.notifiers[strategy]
	if !ok {
		return cakenify.ErrUnknownStrategy
	}

	if err := notifier.Send(ctx, config, cakenify.Message{
		Title: "Cakenify test",
		Body:  "This is a test notification from Cakenify — if you can read this, it's wired up correctly.",
	}); err != nil {
		return errors.Join(cakenify.ErrInternal, err)
	}

	return nil
}

// mapping helpers

func toUser(u cakenifydb.User) *cakenify.User {
	return &cakenify.User{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin, CreatedAt: u.CreatedAt}
}

func toSession(s cakenifydb.Session) *cakenify.Session {
	return &cakenify.Session{ID: s.ID, UserID: s.UserID, ExpiresAt: s.ExpiresAt}
}

func toBirthday(b cakenifydb.Birthday) *cakenify.Birthday {
	return &cakenify.Birthday{
		ID:         b.ID,
		OwnerID:    b.UserID,
		PersonName: b.PersonName,
		BirthMonth: b.BirthMonth,
		BirthDay:   b.BirthDay,
		BirthYear:  b.BirthYear,
		Notes:      b.Notes,
		CreatedAt:  b.CreatedAt,
		UpdatedAt:  b.UpdatedAt,
	}
}

func toNotificationSetting(n cakenifydb.NotificationSetting) *cakenify.NotificationSetting {
	return &cakenify.NotificationSetting{ID: n.ID, UserID: n.UserID, Strategy: n.Strategy, Config: n.Config, Enabled: n.Enabled}
}
