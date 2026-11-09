package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"

	"cakenify/internal/cakenify"
	"cakenify/internal/cakenify/core"
	"cakenify/pkg/db/cakenifydb"
	mockdb "cakenify/pkg/db/cakenifydb/mock"
)

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(hash)
}

func TestService_Login(t *testing.T) {
	userID := uuid.New()
	storedHash := hashPassword(t, "correct-horse-battery-staple")

	tests := []struct {
		name          string
		password      string
		setupMock     func(*mockdb.MockDB)
		expectedError error
	}{
		{
			name:     "success",
			password: "correct-horse-battery-staple",
			setupMock: func(db *mockdb.MockDB) {
				db.EXPECT().UserByUsername(gomock.Any(), "nick").Return(&cakenifydb.User{ID: userID, Username: "nick", PasswordHash: storedHash}, nil)
				db.EXPECT().CreateSession(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, params cakenifydb.CreateSessionParams) (*cakenifydb.Session, error) {
						assert.Equal(t, userID, params.UserID)
						return &cakenifydb.Session{ID: params.ID, UserID: params.UserID, ExpiresAt: params.ExpiresAt}, nil
					})
			},
		},
		{
			name:     "wrong password",
			password: "wrong",
			setupMock: func(db *mockdb.MockDB) {
				db.EXPECT().UserByUsername(gomock.Any(), "nick").Return(&cakenifydb.User{ID: userID, Username: "nick", PasswordHash: storedHash}, nil)
			},
			expectedError: cakenify.ErrInvalidCredentials,
		},
		{
			name:     "unknown user",
			password: "whatever",
			setupMock: func(db *mockdb.MockDB) {
				db.EXPECT().UserByUsername(gomock.Any(), "nick").Return(nil, cakenifydb.ErrNotFound)
			},
			expectedError: cakenify.ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := mockdb.NewMockDB(ctrl)
			tt.setupMock(db)

			svc := core.New(db, cakenify.NewNotifierRegistry(), time.Hour)
			session, err := svc.Login(context.Background(), "nick", tt.password)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, userID, session.UserID)
		})
	}
}

func TestService_CreateUser(t *testing.T) {
	tests := []struct {
		name          string
		params        cakenify.CreateUserParams
		setupMock     func(*mockdb.MockDB)
		expectedError error
	}{
		{
			name:   "success",
			params: cakenify.CreateUserParams{Username: "jurene", Password: "hunter22222", IsAdmin: false},
			setupMock: func(db *mockdb.MockDB) {
				db.EXPECT().CreateUser(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, params cakenifydb.CreateUserParams) (*cakenifydb.User, error) {
						assert.Equal(t, "jurene", params.Username)
						assert.NotEmpty(t, params.PasswordHash)
						return &cakenifydb.User{ID: params.ID, Username: params.Username, IsAdmin: params.IsAdmin}, nil
					})
			},
		},
		{
			name:   "username taken",
			params: cakenify.CreateUserParams{Username: "nick", Password: "hunter22222"},
			setupMock: func(db *mockdb.MockDB) {
				db.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(nil, cakenifydb.ErrAlreadyExists)
			},
			expectedError: cakenify.ErrUsernameTaken,
		},
		{
			name:          "password too short",
			params:        cakenify.CreateUserParams{Username: "nick", Password: "short"},
			setupMock:     func(*mockdb.MockDB) {},
			expectedError: cakenify.ErrInvalidParameters,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := mockdb.NewMockDB(ctrl)
			tt.setupMock(db)

			svc := core.New(db, cakenify.NewNotifierRegistry(), time.Hour)
			user, err := svc.CreateUser(context.Background(), tt.params)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.params.Username, user.Username)
		})
	}
}

func TestService_Birthday_OwnershipEnforced(t *testing.T) {
	owner := uuid.New()
	stranger := uuid.New()
	birthdayID := uuid.New()

	ctrl := gomock.NewController(t)
	db := mockdb.NewMockDB(ctrl)
	db.EXPECT().Birthday(gomock.Any(), birthdayID).Return(&cakenifydb.Birthday{ID: birthdayID, UserID: owner, PersonName: "David Agar"}, nil).AnyTimes()

	svc := core.New(db, cakenify.NewNotifierRegistry(), time.Hour)

	_, err := svc.Birthday(context.Background(), owner, birthdayID)
	require.NoError(t, err)

	_, err = svc.Birthday(context.Background(), stranger, birthdayID)
	assert.ErrorIs(t, err, cakenify.ErrNotFound)
}

func TestService_UpsertNotificationSetting_UnknownStrategy(t *testing.T) {
	ctrl := gomock.NewController(t)
	db := mockdb.NewMockDB(ctrl)

	svc := core.New(db, cakenify.NewNotifierRegistry(), time.Hour)

	_, err := svc.UpsertNotificationSetting(context.Background(), uuid.New(), "pushover", []byte(`{}`), true)
	assert.ErrorIs(t, err, cakenify.ErrUnknownStrategy)
}
