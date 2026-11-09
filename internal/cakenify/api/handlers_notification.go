package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"cakenify/internal/cakenify"
)

func (s *Server) handleNotificationSettingsPage(w http.ResponseWriter, r *http.Request) {
	s.renderNotificationsPage(w, r, userFromContext(r.Context()), "", "")
}

func (s *Server) renderNotificationsPage(w http.ResponseWriter, r *http.Request, user *cakenify.User, errMsg, notice string) {
	settings, err := s.service.ListNotificationSettings(r.Context(), user.ID)
	if err != nil {
		s.renderError(w, r, "list_notification_settings", err)
		return
	}

	var data notificationsData
	for _, setting := range settings {
		if setting.Strategy != "ntfy" {
			continue
		}

		var cfg cakenify.NtfyConfig
		_ = json.Unmarshal(setting.Config, &cfg)
		data.NtfyTopic = cfg.Topic
		data.NtfyHasToken = cfg.Token != ""
		data.NtfyEnabled = setting.Enabled
	}

	renderPage(w, "notifications.html", pageData{
		Title: "Notifications", User: user, CSRFToken: csrfTokenFromContext(r.Context()),
		Error: errMsg, Notice: notice, Data: data,
	})
}

func (s *Server) handleUpsertNotificationSetting(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	strategy, cfg, err := parseNotificationForm(r)
	if err != nil {
		s.renderNotificationsPage(w, r, user, err.Error(), "")
		return
	}

	// A blank token field means "leave it as-is", not "clear it" — the
	// real value is never sent back to the browser, so it can't be
	// resubmitted, and users shouldn't have to re-enter it just to flip
	// the enabled checkbox or change the topic.
	cfg, err = s.preserveTokenIfBlank(r.Context(), user.ID, strategy, cfg)
	if err != nil {
		s.renderNotificationsPage(w, r, user, "could not save notification settings", "")
		return
	}

	enabled := r.FormValue("enabled") == "on"
	if _, err := s.service.UpsertNotificationSetting(r.Context(), user.ID, strategy, cfg, enabled); err != nil {
		s.renderNotificationsPage(w, r, user, "could not save notification settings", "")
		return
	}

	http.Redirect(w, r, "/settings/notifications", http.StatusSeeOther)
}

// preserveTokenIfBlank only matters for "ntfy" today, but is written to
// stay a no-op for any future strategy that doesn't have a secret field.
func (s *Server) preserveTokenIfBlank(ctx context.Context, userID uuid.UUID, strategy string, cfg json.RawMessage) (json.RawMessage, error) {
	if strategy != "ntfy" {
		return cfg, nil
	}

	var submitted cakenify.NtfyConfig
	if err := json.Unmarshal(cfg, &submitted); err != nil {
		return nil, err
	}
	if submitted.Token != "" {
		return cfg, nil
	}

	existing, err := s.service.ListNotificationSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, setting := range existing {
		if setting.Strategy != "ntfy" {
			continue
		}

		var prev cakenify.NtfyConfig
		if err := json.Unmarshal(setting.Config, &prev); err != nil {
			continue
		}

		submitted.Token = prev.Token
		return json.Marshal(submitted)
	}

	return cfg, nil
}

func (s *Server) handleTestNotification(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	if !s.testNotifyLimiter.Allow(user.ID.String()) {
		s.renderNotificationsPage(w, r, user, "too many test notifications sent — please wait a bit", "")
		return
	}

	strategy, cfg, err := parseNotificationForm(r)
	if err != nil {
		s.renderNotificationsPage(w, r, user, err.Error(), "")
		return
	}

	// Test-send uses exactly what's in the form, unlike save — no
	// preserve-on-blank here, since the point is to verify what's about
	// to be saved (or was just typed) actually works.
	if err := s.service.SendTestNotification(r.Context(), strategy, cfg); err != nil {
		s.renderNotificationsPage(w, r, user, "could not send test notification — double-check the topic/token", "")
		return
	}

	s.renderNotificationsPage(w, r, user, "", "test notification sent — check your device")
}

// parseNotificationForm reads the strategy-specific config straight off
// the submitted form, independent of whatever's saved in the DB — used by
// both save and test-send so a test can be run before saving.
func parseNotificationForm(r *http.Request) (strategy string, cfg json.RawMessage, err error) {
	if err := r.ParseForm(); err != nil {
		return "", nil, errors.New("invalid form submission")
	}

	strategy = r.FormValue("strategy")
	switch strategy {
	case "ntfy":
		raw, err := json.Marshal(cakenify.NtfyConfig{Topic: r.FormValue("topic"), Token: r.FormValue("token")})
		if err != nil {
			return "", nil, err
		}
		return strategy, raw, nil
	default:
		return "", nil, errors.New("unknown notification strategy")
	}
}
