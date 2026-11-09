package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"cakenify/internal/cakenify"
	mocksvc "cakenify/internal/cakenify/mock"
)

func TestServer_RequireAuth_RedirectsToLogin(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mocksvc.NewMockService(ctrl)
	svc.EXPECT().SessionUser(gomock.Any(), gomock.Any()).Return(nil, cakenify.ErrUnauthorized).AnyTimes()

	server := NewServer(svc, true)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

func TestServer_Login(t *testing.T) {
	tests := []struct {
		name             string
		password         string
		setupMock        func(*mocksvc.MockService)
		expectedStatus   int
		expectedLocation string
		expectedBody     string
	}{
		{
			name:     "success sets cookie and redirects home",
			password: "correct-horse-battery-staple",
			setupMock: func(svc *mocksvc.MockService) {
				svc.EXPECT().Login(gomock.Any(), "nick", "correct-horse-battery-staple").
					Return(&cakenify.Session{ID: uuid.New(), UserID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour)}, nil)
			},
			expectedStatus:   http.StatusSeeOther,
			expectedLocation: "/",
		},
		{
			name:     "invalid credentials re-renders login with error",
			password: "wrong",
			setupMock: func(svc *mocksvc.MockService) {
				svc.EXPECT().Login(gomock.Any(), "nick", "wrong").Return(nil, cakenify.ErrInvalidCredentials)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "invalid username or password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := mocksvc.NewMockService(ctrl)
			tt.setupMock(svc)

			server := NewServer(svc, true)

			form := url.Values{"username": {"nick"}, "password": {tt.password}}
			req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedLocation != "" {
				assert.Equal(t, tt.expectedLocation, w.Header().Get("Location"))
				assert.NotEmpty(t, w.Result().Cookies())
			}
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}
		})
	}
}
