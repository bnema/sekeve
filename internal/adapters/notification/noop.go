package notification

import (
	"context"

	"github.com/bnema/sekeve/internal/port"
)

// Noop silently discards notifications. Used as fallback when no D-Bus session is available.
type Noop struct{}

func NewNoop() port.NotificationPort { return &Noop{} }

func (n *Noop) Notify(context.Context, string, string, port.Urgency, string) error { return nil }
func (n *Noop) Close() error                                                       { return nil }
