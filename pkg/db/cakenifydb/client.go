package cakenifydb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type client struct {
	db        *sql.DB
	validator *validator.Validate
}

// NewClient wraps an already-open *sql.DB (pgx stdlib driver) as a DB.
func NewClient(db *sql.DB) DB {
	return &client{db: db, validator: validator.New()}
}

func (c *client) Close() {
	_ = c.db.Close()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Users

func (c *client) CreateUser(ctx context.Context, params CreateUserParams) (*User, error) {
	if err := c.validator.StructCtx(ctx, params); err != nil {
		return nil, errors.Join(ErrInvalidParams, err)
	}

	_, err := c.db.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, is_admin) VALUES ($1, $2, $3, $4)`,
		params.ID, params.Username, params.PasswordHash, params.IsAdmin,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAlreadyExists
		}
		return nil, errors.Join(ErrInternal, err)
	}

	return c.UserByID(ctx, params.ID)
}

func (c *client) UserByUsername(ctx context.Context, username string) (*User, error) {
	if username == "" {
		return nil, errors.Join(ErrInvalidParams, errors.New("username cannot be empty"))
	}

	var u User
	err := c.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, is_admin, created_at FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, errors.Join(ErrInternal, err)
	}

	return &u, nil
}

func (c *client) UserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	if id == uuid.Nil {
		return nil, errors.Join(ErrInvalidParams, errors.New("user ID cannot be nil"))
	}

	var u User
	err := c.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, is_admin, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, errors.Join(ErrInternal, err)
	}

	return &u, nil
}

func (c *client) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := c.db.QueryContext(ctx,
		`SELECT id, username, password_hash, is_admin, created_at FROM users ORDER BY username`,
	)
	if err != nil {
		return nil, errors.Join(ErrInternal, err)
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt); err != nil {
			return nil, errors.Join(ErrInternal, err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(ErrInternal, err)
	}

	return users, nil
}

func (c *client) CountUsers(ctx context.Context) (int, error) {
	var count int
	if err := c.db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return 0, errors.Join(ErrInternal, err)
	}

	return count, nil
}

// Sessions

func (c *client) CreateSession(ctx context.Context, params CreateSessionParams) (*Session, error) {
	if err := c.validator.StructCtx(ctx, params); err != nil {
		return nil, errors.Join(ErrInvalidParams, err)
	}

	_, err := c.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		params.ID, params.UserID, params.ExpiresAt,
	)
	if err != nil {
		return nil, errors.Join(ErrInternal, err)
	}

	return &Session{ID: params.ID, UserID: params.UserID, ExpiresAt: params.ExpiresAt}, nil
}

func (c *client) SessionByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	if id == uuid.Nil {
		return nil, errors.Join(ErrInvalidParams, errors.New("session ID cannot be nil"))
	}

	var s Session
	err := c.db.QueryRowContext(ctx,
		`SELECT id, user_id, expires_at FROM sessions WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.UserID, &s.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, errors.Join(ErrInternal, err)
	}

	return &s, nil
}

func (c *client) DeleteSession(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.Join(ErrInvalidParams, errors.New("session ID cannot be nil"))
	}

	if _, err := c.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, id); err != nil {
		return errors.Join(ErrInternal, err)
	}

	return nil
}

// Birthdays

func (c *client) CreateBirthday(ctx context.Context, params CreateBirthdayParams) (*Birthday, error) {
	if err := c.validator.StructCtx(ctx, params); err != nil {
		return nil, errors.Join(ErrInvalidParams, err)
	}

	_, err := c.db.ExecContext(ctx,
		`INSERT INTO birthdays (id, user_id, person_name, birth_month, birth_day, birth_year, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		params.ID, params.UserID, params.PersonName, params.BirthMonth, params.BirthDay, params.BirthYear, params.Notes,
	)
	if err != nil {
		return nil, errors.Join(ErrInternal, err)
	}

	return c.Birthday(ctx, params.ID)
}

func (c *client) BirthdaysByUser(ctx context.Context, userID uuid.UUID) ([]Birthday, error) {
	if userID == uuid.Nil {
		return nil, errors.Join(ErrInvalidParams, errors.New("user ID cannot be nil"))
	}

	rows, err := c.db.QueryContext(ctx,
		`SELECT id, user_id, person_name, birth_month, birth_day, birth_year, notes, created_at, updated_at
		FROM birthdays WHERE user_id = $1 ORDER BY person_name`,
		userID,
	)
	if err != nil {
		return nil, errors.Join(ErrInternal, err)
	}
	defer rows.Close()

	return scanBirthdays(rows)
}

func (c *client) Birthday(ctx context.Context, id uuid.UUID) (*Birthday, error) {
	if id == uuid.Nil {
		return nil, errors.Join(ErrInvalidParams, errors.New("birthday ID cannot be nil"))
	}

	var b Birthday
	err := c.db.QueryRowContext(ctx,
		`SELECT id, user_id, person_name, birth_month, birth_day, birth_year, notes, created_at, updated_at
		FROM birthdays WHERE id = $1`,
		id,
	).Scan(&b.ID, &b.UserID, &b.PersonName, &b.BirthMonth, &b.BirthDay, &b.BirthYear, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, errors.Join(ErrInternal, err)
	}

	return &b, nil
}

func (c *client) UpdateBirthday(ctx context.Context, id uuid.UUID, params UpdateBirthdayParams) (*Birthday, error) {
	if id == uuid.Nil {
		return nil, errors.Join(ErrInvalidParams, errors.New("birthday ID cannot be nil"))
	}
	if err := c.validator.StructCtx(ctx, params); err != nil {
		return nil, errors.Join(ErrInvalidParams, err)
	}

	res, err := c.db.ExecContext(ctx,
		`UPDATE birthdays SET person_name = $1, birth_month = $2, birth_day = $3, birth_year = $4, notes = $5, updated_at = now()
		WHERE id = $6`,
		params.PersonName, params.BirthMonth, params.BirthDay, params.BirthYear, params.Notes, id,
	)
	if err != nil {
		return nil, errors.Join(ErrInternal, err)
	}

	cnt, err := res.RowsAffected()
	if err != nil {
		return nil, errors.Join(ErrInternal, err)
	}
	if cnt == 0 {
		return nil, ErrNotFound
	}

	return c.Birthday(ctx, id)
}

func (c *client) DeleteBirthday(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.Join(ErrInvalidParams, errors.New("birthday ID cannot be nil"))
	}

	res, err := c.db.ExecContext(ctx, `DELETE FROM birthdays WHERE id = $1`, id)
	if err != nil {
		return errors.Join(ErrInternal, err)
	}

	cnt, err := res.RowsAffected()
	if err != nil {
		return errors.Join(ErrInternal, err)
	}
	if cnt == 0 {
		return ErrNotFound
	}

	return nil
}

// BirthdaysForMonthDays returns every birthday matching any of the given
// month/day pairs. Used both for "today" (one pair) and the weekly digest
// (up to seven).
func (c *client) BirthdaysForMonthDays(ctx context.Context, days []MonthDay) ([]Birthday, error) {
	if len(days) == 0 {
		return []Birthday{}, nil
	}

	conditions := make([]string, 0, len(days))
	args := make([]any, 0, len(days)*2)
	for _, d := range days {
		args = append(args, d.Month, d.Day)
		conditions = append(conditions, fmt.Sprintf("(birth_month = $%d AND birth_day = $%d)", len(args)-1, len(args)))
	}

	query := fmt.Sprintf(
		`SELECT id, user_id, person_name, birth_month, birth_day, birth_year, notes, created_at, updated_at
		FROM birthdays WHERE %s`,
		strings.Join(conditions, " OR "),
	)

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Join(ErrInternal, err)
	}
	defer rows.Close()

	return scanBirthdays(rows)
}

func scanBirthdays(rows *sql.Rows) ([]Birthday, error) {
	birthdays := []Birthday{}
	for rows.Next() {
		var b Birthday
		if err := rows.Scan(&b.ID, &b.UserID, &b.PersonName, &b.BirthMonth, &b.BirthDay, &b.BirthYear, &b.Notes, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, errors.Join(ErrInternal, err)
		}
		birthdays = append(birthdays, b)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(ErrInternal, err)
	}

	return birthdays, nil
}

// Notification settings

func (c *client) UpsertNotificationSetting(ctx context.Context, params UpsertNotificationSettingParams) (*NotificationSetting, error) {
	if err := c.validator.StructCtx(ctx, params); err != nil {
		return nil, errors.Join(ErrInvalidParams, err)
	}

	_, err := c.db.ExecContext(ctx,
		`INSERT INTO notification_settings (id, user_id, strategy, config, enabled)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, strategy) DO UPDATE SET config = EXCLUDED.config, enabled = EXCLUDED.enabled`,
		params.ID, params.UserID, params.Strategy, params.Config, params.Enabled,
	)
	if err != nil {
		return nil, errors.Join(ErrInternal, err)
	}

	var s NotificationSetting
	err = c.db.QueryRowContext(ctx,
		`SELECT id, user_id, strategy, config, enabled FROM notification_settings WHERE user_id = $1 AND strategy = $2`,
		params.UserID, params.Strategy,
	).Scan(&s.ID, &s.UserID, &s.Strategy, &s.Config, &s.Enabled)
	if err != nil {
		return nil, errors.Join(ErrInternal, err)
	}

	return &s, nil
}

func (c *client) NotificationSettingsByUser(ctx context.Context, userID uuid.UUID) ([]NotificationSetting, error) {
	return c.notificationSettings(ctx, userID, false)
}

func (c *client) EnabledNotificationSettingsByUser(ctx context.Context, userID uuid.UUID) ([]NotificationSetting, error) {
	return c.notificationSettings(ctx, userID, true)
}

func (c *client) notificationSettings(ctx context.Context, userID uuid.UUID, enabledOnly bool) ([]NotificationSetting, error) {
	if userID == uuid.Nil {
		return nil, errors.Join(ErrInvalidParams, errors.New("user ID cannot be nil"))
	}

	query := `SELECT id, user_id, strategy, config, enabled FROM notification_settings WHERE user_id = $1`
	if enabledOnly {
		query += ` AND enabled = true`
	}

	rows, err := c.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, errors.Join(ErrInternal, err)
	}
	defer rows.Close()

	settings := []NotificationSetting{}
	for rows.Next() {
		var s NotificationSetting
		if err := rows.Scan(&s.ID, &s.UserID, &s.Strategy, &s.Config, &s.Enabled); err != nil {
			return nil, errors.Join(ErrInternal, err)
		}
		settings = append(settings, s)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(ErrInternal, err)
	}

	return settings, nil
}

// Scheduler state

func (c *client) SchedulerLastRunDate(ctx context.Context, jobName string) (*time.Time, error) {
	if jobName == "" {
		return nil, errors.Join(ErrInvalidParams, errors.New("job name cannot be empty"))
	}

	var t time.Time
	err := c.db.QueryRowContext(ctx, `SELECT last_run_date FROM scheduler_state WHERE job_name = $1`, jobName).Scan(&t)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errors.Join(ErrInternal, err)
	}

	return &t, nil
}

func (c *client) SetSchedulerLastRunDate(ctx context.Context, jobName string, date time.Time) error {
	if jobName == "" {
		return errors.Join(ErrInvalidParams, errors.New("job name cannot be empty"))
	}

	_, err := c.db.ExecContext(ctx,
		`INSERT INTO scheduler_state (job_name, last_run_date) VALUES ($1, $2)
		ON CONFLICT (job_name) DO UPDATE SET last_run_date = EXCLUDED.last_run_date`,
		jobName, date,
	)
	if err != nil {
		return errors.Join(ErrInternal, err)
	}

	return nil
}
