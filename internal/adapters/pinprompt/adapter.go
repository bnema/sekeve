//go:build linux

package pinprompt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bnema/puregotk/v4/gdk"
	"github.com/bnema/puregotk/v4/gio"
	"github.com/bnema/puregotk/v4/glib"
	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/puregotk/v4/layershell"
	"github.com/bnema/sekeve/internal/port"
	"golang.org/x/term"
)

var _ port.PINPromptPort = (*PINPromptAdapter)(nil)

var (
	gtkAvailableOnce sync.Once
	gtkAvailable     bool
)

func checkGTKAvailable() bool {
	gtkAvailableOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				gtkAvailable = false
			}
		}()
		gtkAvailable = gtk.InitCheck()
	})
	return gtkAvailable
}

// PINPromptAdapter implements port.PINPromptPort with GTK4 GUI and TTY fallback.
type PINPromptAdapter struct {
	isTTY        bool
	guiAvailable bool
}

// NewPINPromptAdapter creates a new adapter, detecting TTY and GUI availability.
func NewPINPromptAdapter() *PINPromptAdapter {
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	guiAvailable := checkGTKAvailable()
	return &PINPromptAdapter{isTTY: isTTY, guiAvailable: guiAvailable}
}

// IsTTY reports whether the adapter will use terminal input.
func (a *PINPromptAdapter) IsTTY() bool { return a.isTTY }

const maxMessageLen = 200

// PromptForPIN asks the user for a PIN via GUI or TTY fallback.
// The validate callback is called for each PIN submission; the prompt stays
// open until validate returns nil, the user cancels, or the context expires.
func (a *PINPromptAdapter) PromptForPIN(ctx context.Context, validate port.PINValidateFunc) error {
	if a.guiAvailable {
		return a.promptGUI(ctx, validate)
	}

	if a.isTTY {
		fmt.Fprintln(os.Stderr, "sekeve: GUI unavailable, falling back to terminal input")
		return promptTTY(ctx, validate)
	}

	return port.ErrNoPINInputMethod
}

const pinCSS = `
window {
    background-color: #1B1B1F;
}
entry, passwordentry {
    background-color: #25252A;
    color: #FAFAFA;
    border: 1px solid #333;
    border-radius: 4px;
    padding: 8px 12px;
    font-size: 20px;
    min-width: 320px;
}
window.error entry, window.error passwordentry {
    border-color: #E5484D;
    background-color: #2A2020;
}
label {
    color: #999;
    margin-bottom: 8px;
}
window.error label {
    color: #E5484D;
}
`

const promptTimeout = 90 * time.Second

func (a *PINPromptAdapter) promptGUI(ctx context.Context, validate port.PINValidateFunc) error {
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

		if layershell.Available() && layershell.IsSupported() {
			layershell.InitForWindow(&window.Window)
			layershell.SetLayer(&window.Window, layershell.LayerOverlayValue)
			layershell.SetKeyboardMode(&window.Window, layershell.KeyboardModeExclusiveValue)
			layershell.SetExclusiveZone(&window.Window, 0)
			ns := "sekeve-pin"
			layershell.SetNamespace(&window.Window, &ns)
		}

		cssProvider := gtk.NewCssProvider()
		cssProvider.LoadFromString(pinCSS)
		if display := gdk.DisplayGetDefault(); display != nil {
			gtk.StyleContextAddProviderForDisplay(display, cssProvider, 600)
		}

		vbox := gtk.NewBox(gtk.OrientationVerticalValue, 0)
		vbox.SetMarginTop(16)
		vbox.SetMarginBottom(16)
		vbox.SetMarginStart(16)
		vbox.SetMarginEnd(16)

		label := gtk.NewLabel(nil)
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

				// Check for fatal error (e.g. max retries exceeded).
				var fatal *port.PINFatalError
				if errors.As(err, &fatal) {
					fn := glib.SourceOnceFunc(func(uintptr) {
						validationErr = fatal.Err
						app.Quit()
					})
					glib.IdleAddOnce(&fn, 0)
					return
				}

				// Retriable error — show message, clear input, refocus.
				msg := err.Error()
				if utf8.RuneCountInString(msg) > maxMessageLen {
					msg = string([]rune(msg)[:maxMessageLen])
				}
				fn := glib.SourceOnceFunc(func(uintptr) {
					window.AddCssClass("error")
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

	// Context cancellation and inactivity timeout from a goroutine.
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
			return // app.Run already returned
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
