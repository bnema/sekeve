//go:build linux

package pinprompt

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/bnema/puregotk/v4/gdk"
	"github.com/bnema/puregotk/v4/gio"
	"github.com/bnema/puregotk/v4/glib"
	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/puregotk/v4/layershell"
	"github.com/bnema/sekeve/internal/port"
	"golang.org/x/term"
)

// Compile-time interface check.
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

// PromptForPIN asks the user for a PIN via GUI or TTY fallback.
func (a *PINPromptAdapter) PromptForPIN(ctx context.Context, errorMode bool, message string) (string, error) {
	if a.guiAvailable {
		return a.promptGUI(ctx, errorMode, message)
	}

	if a.isTTY {
		fmt.Fprintln(os.Stderr, "sekeve: GUI unavailable, falling back to terminal input")
		return a.promptTTY(ctx, errorMode, message)
	}

	return "", fmt.Errorf("no PIN input method available (no GUI display and no TTY)")
}

// promptTTY reads a PIN from the terminal. Note: term.ReadPassword is a blocking
// syscall with no cancellation mechanism, so ctx is not used here.
func (a *PINPromptAdapter) promptTTY(_ context.Context, errorMode bool, message string) (string, error) {
	if errorMode && message == "" {
		fmt.Fprintln(os.Stderr, "Incorrect PIN, please try again.")
	} else if message != "" {
		fmt.Fprintln(os.Stderr, message)
	}

	fmt.Fprint(os.Stderr, "Unlock PIN: ")
	pin, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after masked input
	if err != nil {
		return "", fmt.Errorf("failed to read PIN: %w", err)
	}
	return string(pin), nil
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

func (a *PINPromptAdapter) promptGUI(ctx context.Context, errorMode bool, message string) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var pin string

	appID := "dev.bnema.sekeve.pinprompt"
	app := gtk.NewApplication(&appID, gio.GApplicationNonUniqueValue)

	activateCb := func(gio.Application) {
		window := gtk.NewApplicationWindow(app)

		title := "Sekeve"
		window.SetTitle(&title)
		window.SetDecorated(false)

		// Layer-shell setup (Wayland overlay positioning).
		if layershell.Available() && layershell.IsSupported() {
			layershell.InitForWindow(&window.Window)
			layershell.SetLayer(&window.Window, layershell.LayerOverlayValue)
			layershell.SetAnchor(&window.Window, layershell.EdgeBottomValue, true)
			layershell.SetKeyboardMode(&window.Window, layershell.KeyboardModeExclusiveValue)
			layershell.SetExclusiveZone(&window.Window, 0)
			layershell.SetMargin(&window.Window, layershell.EdgeBottomValue, 40)
			ns := "sekeve-pin"
			layershell.SetNamespace(&window.Window, &ns)
		}

		// CSS styling.
		cssProvider := gtk.NewCssProvider()
		cssProvider.LoadFromString(pinCSS)
		display := gdk.DisplayGetDefault()
		gtk.StyleContextAddProviderForDisplay(display, cssProvider, 600)

		// Error styling.
		if errorMode {
			window.AddCssClass("error")
		}

		// Widget tree.
		vbox := gtk.NewBox(gtk.OrientationVerticalValue, 0)
		vbox.SetMarginTop(16)
		vbox.SetMarginBottom(16)
		vbox.SetMarginStart(16)
		vbox.SetMarginEnd(16)

		label := gtk.NewLabel(nil)
		if message != "" {
			label.SetText(message)
			label.SetVisible(true)
		} else if errorMode {
			msg := "Incorrect PIN, please try again."
			label.SetText(msg)
			label.SetVisible(true)
		} else {
			label.SetVisible(false)
		}
		vbox.Append(&label.Widget)

		entry := gtk.NewPasswordEntry()
		entry.SetPropertyPlaceholderText("PIN")
		entry.SetPropertyActivatesDefault(true)

		// Enter key submits.
		activateEntryCb := func(gtk.PasswordEntry) {
			text := entry.GetText()
			if text != "" {
				pin = text
				app.Quit()
			}
		}
		entry.ConnectActivate(&activateEntryCb)
		vbox.Append(&entry.Widget)

		window.SetChild(&vbox.Widget)

		// Escape key cancels.
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

		// Window close without submitting.
		closeRequestCb := func(gtk.Window) bool {
			app.Quit()
			return true
		}
		window.ConnectCloseRequest(&closeRequestCb)

		window.Show()
		entry.GrabFocus()
	}
	app.ConnectActivate(&activateCb)

	// Context cancellation from a goroutine.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			quitFn := glib.SourceOnceFunc(func(uintptr) { app.Quit() })
			glib.IdleAddOnce(&quitFn, 0)
		case <-done:
		}
	}()

	app.Run(0, nil)
	close(done)

	// Determine result.
	if pin != "" {
		return pin, nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return "", port.ErrPINPromptCancelled
}
