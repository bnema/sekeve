package port

import "context"

// Urgency represents the notification urgency level per the freedesktop spec.
type Urgency uint8

const (
	UrgencyLow      Urgency = 0
	UrgencyNormal   Urgency = 1
	UrgencyCritical Urgency = 2
)

// NotificationPort sends desktop notifications via the freedesktop D-Bus spec.
type NotificationPort interface {
	// Notify sends a desktop notification. Icon should be a freedesktop icon name
	// (e.g. "dialog-password", "dialog-error") or empty for the default.
	Notify(ctx context.Context, summary, body string, urgency Urgency, icon string) error

	// Close releases the D-Bus connection.
	Close() error
}
