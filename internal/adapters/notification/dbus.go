// Package notification implements desktop notifications via D-Bus.
package notification

import (
	"context"
	"fmt"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"

	"github.com/bnema/sekeve/internal/port"
)

const (
	appName          = "sekeve"
	defaultTimeoutMs = 5000 // 5 seconds
)

// DBusNotifier sends desktop notifications over the org.freedesktop.Notifications D-Bus interface.
type DBusNotifier struct {
	conn     *dbus.Conn
	notifier notify.Notifier
}

// NewDBus connects to the session bus and returns a ready notifier.
// Returns an error if the session bus or notifier cannot be initialised;
// callers should fall back to NewNoop().
func NewDBus() (port.NotificationPort, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("session bus: %w", err)
	}

	notifier, err := notify.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("notifier: %w", err)
	}

	return &DBusNotifier{conn: conn, notifier: notifier}, nil
}

func (d *DBusNotifier) Notify(_ context.Context, summary, body string, urgency port.Urgency, icon string) error {
	if icon == "" {
		icon = "dialog-password"
	}

	n := notify.Notification{
		AppName:       appName,
		AppIcon:       icon,
		Summary:       summary,
		Body:          body,
		ExpireTimeout: defaultTimeoutMs,
		Hints: map[string]dbus.Variant{
			"urgency":       dbus.MakeVariant(byte(urgency)),
			"desktop-entry": dbus.MakeVariant("dev.bnema.sekeve"),
		},
	}

	if _, err := d.notifier.SendNotification(n); err != nil {
		return fmt.Errorf("notification send: %w", err)
	}
	return nil
}

func (d *DBusNotifier) Close() error {
	d.notifier.Close()
	return d.conn.Close()
}
