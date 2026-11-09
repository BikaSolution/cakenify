package ntfy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cakenify/pkg/ntfy"
)

func TestClient_Publish(t *testing.T) {
	tests := []struct {
		name          string
		topic         string
		token         string
		serverStatus  int
		expectedError bool
	}{
		{name: "success", topic: "birthday_nw", token: "tk_abc", serverStatus: http.StatusOK},
		{name: "no token still publishes", topic: "birthday_nw", token: "", serverStatus: http.StatusOK},
		{name: "server error", topic: "birthday_nw", token: "tk_abc", serverStatus: http.StatusInternalServerError, expectedError: true},
		{name: "empty topic rejected", topic: "", token: "tk_abc", expectedError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotTitle, gotAuth, gotBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotTitle = r.Header.Get("Title")
				gotAuth = r.Header.Get("Authorization")
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			c := ntfy.NewClient(server.URL)
			err := c.Publish(context.Background(), tt.topic, tt.token, "Birthday today", "David Agar")

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "/"+tt.topic, gotPath)
			assert.Equal(t, "Birthday today", gotTitle)
			assert.Equal(t, "David Agar", gotBody)
			if tt.token != "" {
				assert.Equal(t, "Bearer "+tt.token, gotAuth)
			} else {
				assert.Empty(t, gotAuth)
			}
		})
	}
}
