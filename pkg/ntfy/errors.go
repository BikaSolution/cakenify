package ntfy

import "errors"

var (
	ErrInvalidParams = errors.New("ntfy: invalid parameters")
	ErrPublishFailed = errors.New("ntfy: publish failed")
)
