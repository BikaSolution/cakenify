package cakenify

import (
	"context"
	"encoding/json"
	"errors"

	"cakenify/pkg/ntfy"
)

// NtfyConfig is the shape of a user's notification_settings.config when
// strategy == "ntfy". Users enter these themselves via the notification
// settings page — nothing is seeded.
type NtfyConfig struct {
	Topic string `json:"topic" validate:"required"`
	Token string `json:"token"`
}

type ntfyNotifier struct {
	client ntfy.Client
}

// NewNtfyNotifier adapts an ntfy.Client to the Notifier port.
func NewNtfyNotifier(client ntfy.Client) Notifier {
	return &ntfyNotifier{client: client}
}

func (n *ntfyNotifier) Strategy() string {
	return "ntfy"
}

func (n *ntfyNotifier) Send(ctx context.Context, cfg json.RawMessage, msg Message) error {
	var c NtfyConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return errors.Join(ErrInvalidParameters, err)
	}
	if c.Topic == "" {
		return errors.Join(ErrInvalidParameters, errors.New("ntfy: topic is required"))
	}

	return n.client.Publish(ctx, c.Topic, c.Token, msg.Title, msg.Body)
}

var _ Notifier = (*ntfyNotifier)(nil)
