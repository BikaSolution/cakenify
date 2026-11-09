package cakenify

import (
	"context"
	"encoding/json"
	"sort"
)

// Notifier is the "blanket interface" for a notification strategy. Adding a
// new one (email, Pushover, Slack, ...) means writing one adapter that
// implements this and registering it — no changes anywhere else.
type Notifier interface {
	Strategy() string
	Send(ctx context.Context, cfg json.RawMessage, msg Message) error
}

// NotifierRegistry looks up a Notifier by its strategy name.
type NotifierRegistry map[string]Notifier

func NewNotifierRegistry(notifiers ...Notifier) NotifierRegistry {
	reg := make(NotifierRegistry, len(notifiers))
	for _, n := range notifiers {
		reg[n.Strategy()] = n
	}

	return reg
}

func (r NotifierRegistry) Strategies() []string {
	out := make([]string, 0, len(r))
	for s := range r {
		out = append(out, s)
	}
	sort.Strings(out)

	return out
}
