package ntfy

import "context"

// Client publishes messages to topics on an ntfy server. The base URL and
// HTTP transport are fixed at construction; topic/token/title/body vary
// per call since cakenify publishes to a different topic per user.
type Client interface {
	Publish(ctx context.Context, topic, token, title, body string) error
}
