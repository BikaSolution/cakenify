package ntfy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type client struct {
	baseURL string
	http    *http.Client
}

// NewClient returns a Client that publishes against the ntfy server at
// baseURL (e.g. "https://ntfy.bikasolutions.co.za").
func NewClient(baseURL string) Client {
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *client) Publish(ctx context.Context, topic, token, title, body string) error {
	if topic == "" {
		return errors.Join(ErrInvalidParams, errors.New("topic cannot be empty"))
	}

	target, err := url.JoinPath(c.baseURL, topic)
	if err != nil {
		return errors.Join(ErrInvalidParams, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		return errors.Join(ErrPublishFailed, err)
	}
	req.Header.Set("Title", title)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return errors.Join(ErrPublishFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return errors.Join(ErrPublishFailed, fmt.Errorf("unexpected status %d", resp.StatusCode))
	}

	return nil
}

var _ Client = (*client)(nil)
