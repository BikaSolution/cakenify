package cronjob

import "context"

// Client runs a task on a fixed interval. Adapted (trimmed down — no
// ForceReset/ResetInterval, cakenify's interval never changes at runtime)
// from bash-checkout-services-go's pkg/cronjob.
type Client interface {
	// Start begins executing the task on the configured interval. Non-blocking.
	Start(ctx context.Context)

	// Stop halts execution and blocks until the current task (if any) finishes.
	Stop()
}
