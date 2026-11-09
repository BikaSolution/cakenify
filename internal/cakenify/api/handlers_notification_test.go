package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"cakenify/internal/cakenify"
	mocksvc "cakenify/internal/cakenify/mock"
)

// newAuthedRequest builds a request carrying a session cookie plus a
// matching X-CSRF-Token for that same session, so it passes requireAuth's
// CSRF check on unsafe methods.
func newAuthedRequest(t *testing.T, server *Server, method, target string, form url.Values) *http.Request {
	t.Helper()

	sessionID := uuid.New()
	req := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRF-Token", server.csrfToken(sessionID))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID.String()})

	return req
}

func TestServer_TestNotification(t *testing.T) {
	tests := []struct {
		name           string
		form           url.Values
		setupMock      func(*mocksvc.MockService)
		expectedBody   string
		expectedStatus int
	}{
		{
			name: "success shows notice",
			form: url.Values{"strategy": {"ntfy"}, "topic": {"birthday_nw"}, "token": {""}},
			setupMock: func(svc *mocksvc.MockService) {
				svc.EXPECT().SendTestNotification(gomock.Any(), "ntfy", gomock.Any()).Return(nil)
				svc.EXPECT().ListNotificationSettings(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "test notification sent",
		},
		{
			name: "notifier failure shows error",
			form: url.Values{"strategy": {"ntfy"}, "topic": {"birthday_nw"}, "token": {""}},
			setupMock: func(svc *mocksvc.MockService) {
				svc.EXPECT().SendTestNotification(gomock.Any(), "ntfy", gomock.Any()).Return(cakenify.ErrInternal)
				svc.EXPECT().ListNotificationSettings(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "could not send test notification",
		},
		{
			name: "unknown strategy never reaches the service",
			form: url.Values{"strategy": {"carrier-pigeon"}},
			setupMock: func(svc *mocksvc.MockService) {
				svc.EXPECT().ListNotificationSettings(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "unknown notification strategy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := mocksvc.NewMockService(ctrl)
			svc.EXPECT().SessionUser(gomock.Any(), gomock.Any()).Return(&cakenify.User{Username: "nick"}, nil).AnyTimes()
			tt.setupMock(svc)

			server := NewServer(svc, true)

			req := newAuthedRequest(t, server, http.MethodPost, "/settings/notifications/test", tt.form)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedBody)
		})
	}
}
