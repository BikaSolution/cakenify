package cakenifydb_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cakenify/pkg/db/cakenifydb"
)

// newTestDB connects to a real Postgres instance for integration testing.
// Skips the whole test if CAKENIFY_TEST_DATABASE_URL isn't set — see
// docker-compose.test.yml for spinning one up locally.
func newTestDB(t *testing.T) cakenifydb.DB {
	t.Helper()

	dsn := os.Getenv("CAKENIFY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CAKENIFY_TEST_DATABASE_URL not set, skipping integration test")
	}

	sqlDB, err := sql.Open("pgx", dsn)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, sqlDB.PingContext(ctx))
	require.NoError(t, cakenifydb.RunMigration(ctx, sqlDB))

	_, err = sqlDB.ExecContext(ctx, `TRUNCATE TABLE scheduler_state, notification_settings, birthdays, sessions, users CASCADE`)
	require.NoError(t, err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	return cakenifydb.NewClient(sqlDB)
}

func TestUsers(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	userID := uuid.New()
	created, err := db.CreateUser(ctx, cakenifydb.CreateUserParams{
		ID:           userID,
		Username:     "nick",
		PasswordHash: "hash",
		IsAdmin:      true,
	})
	require.NoError(t, err)
	assert.Equal(t, "nick", created.Username)
	assert.True(t, created.IsAdmin)

	_, err = db.CreateUser(ctx, cakenifydb.CreateUserParams{ID: uuid.New(), Username: "nick", PasswordHash: "hash"})
	assert.ErrorIs(t, err, cakenifydb.ErrAlreadyExists)

	byUsername, err := db.UserByUsername(ctx, "nick")
	require.NoError(t, err)
	assert.Equal(t, userID, byUsername.ID)

	_, err = db.UserByUsername(ctx, "does-not-exist")
	assert.ErrorIs(t, err, cakenifydb.ErrNotFound)

	count, err := db.CountUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	users, err := db.ListUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 1)
}

func TestSessions(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, cakenifydb.CreateUserParams{ID: uuid.New(), Username: "jurene", PasswordHash: "hash"})
	require.NoError(t, err)

	sessionID := uuid.New()
	expiresAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	session, err := db.CreateSession(ctx, cakenifydb.CreateSessionParams{ID: sessionID, UserID: user.ID, ExpiresAt: expiresAt})
	require.NoError(t, err)
	assert.Equal(t, user.ID, session.UserID)

	fetched, err := db.SessionByID(ctx, sessionID)
	require.NoError(t, err)
	assert.WithinDuration(t, expiresAt, fetched.ExpiresAt, time.Second)

	require.NoError(t, db.DeleteSession(ctx, sessionID))

	_, err = db.SessionByID(ctx, sessionID)
	assert.ErrorIs(t, err, cakenifydb.ErrNotFound)
}

func TestBirthdays(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, cakenifydb.CreateUserParams{ID: uuid.New(), Username: "owner", PasswordHash: "hash"})
	require.NoError(t, err)

	year := 1990
	birthdayID := uuid.New()
	created, err := db.CreateBirthday(ctx, cakenifydb.CreateBirthdayParams{
		ID:         birthdayID,
		UserID:     user.ID,
		PersonName: "David Agar",
		BirthMonth: 3,
		BirthDay:   14,
		BirthYear:  &year,
	})
	require.NoError(t, err)
	assert.Equal(t, "David Agar", created.PersonName)

	updated, err := db.UpdateBirthday(ctx, birthdayID, cakenifydb.UpdateBirthdayParams{
		PersonName: "David A. Agar",
		BirthMonth: 3,
		BirthDay:   15,
		Notes:      "moved a day",
	})
	require.NoError(t, err)
	assert.Equal(t, "David A. Agar", updated.PersonName)
	assert.Equal(t, 15, updated.BirthDay)
	assert.Nil(t, updated.BirthYear)

	byUser, err := db.BirthdaysByUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, byUser, 1)

	matches, err := db.BirthdaysForMonthDays(ctx, []cakenifydb.MonthDay{{Month: 3, Day: 15}, {Month: 12, Day: 25}})
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, birthdayID, matches[0].ID)

	require.NoError(t, db.DeleteBirthday(ctx, birthdayID))

	_, err = db.Birthday(ctx, birthdayID)
	assert.ErrorIs(t, err, cakenifydb.ErrNotFound)
}

func TestNotificationSettings(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, cakenifydb.CreateUserParams{ID: uuid.New(), Username: "settings-user", PasswordHash: "hash"})
	require.NoError(t, err)

	cfg, err := json.Marshal(map[string]string{"topic": "birthday_nw", "token": "tk_abc"})
	require.NoError(t, err)

	_, err = db.UpsertNotificationSetting(ctx, cakenifydb.UpsertNotificationSettingParams{
		ID: uuid.New(), UserID: user.ID, Strategy: "ntfy", Config: cfg, Enabled: true,
	})
	require.NoError(t, err)

	// Upsert again with a different config — should update in place, not duplicate.
	cfg2, err := json.Marshal(map[string]string{"topic": "birthday_nw", "token": "tk_def"})
	require.NoError(t, err)
	updated, err := db.UpsertNotificationSetting(ctx, cakenifydb.UpsertNotificationSettingParams{
		ID: uuid.New(), UserID: user.ID, Strategy: "ntfy", Config: cfg2, Enabled: false,
	})
	require.NoError(t, err)
	assert.False(t, updated.Enabled)

	all, err := db.NotificationSettingsByUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, all, 1)

	enabled, err := db.EnabledNotificationSettingsByUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, enabled)
}

func TestSchedulerState(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	none, err := db.SchedulerLastRunDate(ctx, "daily_reminder")
	require.NoError(t, err)
	assert.Nil(t, none)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	require.NoError(t, db.SetSchedulerLastRunDate(ctx, "daily_reminder", today))

	got, err := db.SchedulerLastRunDate(ctx, "daily_reminder")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, today.Equal(*got))
}
