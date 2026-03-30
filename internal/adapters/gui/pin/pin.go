// internal/adapters/gui/pin/pin.go
//go:build linux

package pin

import (
	"context"
	"errors"
	"runtime"
	"time"
	"unicode/utf8"

	"github.com/bnema/puregotk/v4/gdk"
	"github.com/bnema/puregotk/v4/gio"
	"github.com/bnema/puregotk/v4/glib"
	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/sekeve/internal/port"
	lsh "github.com/bnema/sekeve/pkg/layershell"
)

const (
	maxMessageLen = 200
	promptTimeout = 90 * time.Second
)

// PromptGUI shows a GTK4 layer-shell PIN prompt.
func PromptGUI(ctx context.Context, validate port.PINValidateFunc, css string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var success bool
	var validationErr error

	appID := "dev.bnema.sekeve.pinprompt"
	app := gtk.NewApplication(&appID, gio.GApplicationNonUniqueValue)

	resetCh := make(chan struct{}, 1)

	activateCb := func(gio.Application) {
		window := gtk.NewApplicationWindow(app)

		title := "Sekeve"
		window.SetTitle(&title)
		window.SetDecorated(false)
		window.SetFocusVisible(false)

		lsh.InitOverlay(&window.Window, lsh.OverlayConfig{
			Namespace: "sekeve-pin",
			Exclusive: true,
		})

		cssProvider := gtk.NewCssProvider()
		cssProvider.LoadFromString(css)
		if display := gdk.DisplayGetDefault(); display != nil {
			gtk.StyleContextAddProviderForDisplay(display, cssProvider, 800)
		}

		vbox := gtk.NewBox(gtk.OrientationVerticalValue, 0)
		vbox.AddCssClass("sekeve-pin")
		vbox.SetMarginTop(16)
		vbox.SetMarginBottom(16)
		vbox.SetMarginStart(16)
		vbox.SetMarginEnd(16)

		label := gtk.NewLabel(nil)
		label.AddCssClass("sekeve-label")
		label.SetVisible(false)

		entry := gtk.NewPasswordEntry()
		entry.SetPropertyPlaceholderText("PIN")
		entry.SetPropertyActivatesDefault(true)

		activateEntryCb := func(gtk.PasswordEntry) {
			text := entry.GetText()
			if text == "" {
				return
			}

			select {
			case resetCh <- struct{}{}:
			default:
			}

			entry.SetSensitive(false)

			go func() {
				err := validate(ctx, text)

				if err == nil {
					fn := glib.SourceOnceFunc(func(uintptr) {
						success = true
						app.Quit()
					})
					glib.IdleAddOnce(&fn, 0)
					return
				}

				var fatal *port.PINFatalError
				if errors.As(err, &fatal) {
					fn := glib.SourceOnceFunc(func(uintptr) {
						validationErr = fatal.Err
						app.Quit()
					})
					glib.IdleAddOnce(&fn, 0)
					return
				}

				msg := err.Error()
				if utf8.RuneCountInString(msg) > maxMessageLen {
					msg = string([]rune(msg)[:maxMessageLen])
				}
				fn := glib.SourceOnceFunc(func(uintptr) {
					vbox.AddCssClass("sekeve-pin-error")
					label.SetText(msg)
					label.SetVisible(true)
					entry.SetText("")
					entry.SetSensitive(true)
					entry.GrabFocus()
				})
				glib.IdleAddOnce(&fn, 0)
			}()
		}
		entry.ConnectActivate(&activateEntryCb)

		vbox.Append(&label.Widget)
		vbox.Append(&entry.Widget)

		window.SetChild(&vbox.Widget)

		keyCtrl := gtk.NewEventControllerKey()
		keyPressedCb := func(_ gtk.EventControllerKey, keyval uint, _ uint, _ gdk.ModifierType) bool {
			if keyval == uint(gdk.KEY_Escape) {
				app.Quit()
				return true
			}
			return false
		}
		keyCtrl.ConnectKeyPressed(&keyPressedCb)
		window.AddController(&keyCtrl.EventController)

		closeRequestCb := func(gtk.Window) bool {
			app.Quit()
			return true
		}
		window.ConnectCloseRequest(&closeRequestCb)

		window.Show()
		entry.GrabFocus()
	}
	app.ConnectActivate(&activateCb)

	done := make(chan struct{})
	go func() {
		timer := time.NewTimer(promptTimeout)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
			case <-timer.C:
			case <-done:
				return
			case <-resetCh:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(promptTimeout)
				continue
			}
			break
		}
		select {
		case <-done:
			return
		default:
		}
		quitFn := glib.SourceOnceFunc(func(uintptr) { app.Quit() })
		glib.IdleAddOnce(&quitFn, 0)
	}()

	app.Run(0, nil)
	close(done)

	if success {
		return nil
	}
	if validationErr != nil {
		return validationErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return port.ErrPINPromptCancelled
}
