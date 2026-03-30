// internal/adapters/gui/app.go
//go:build linux && !nogtk

package gui

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/bnema/puregotk/v4/gdk"
	"github.com/bnema/puregotk/v4/gio"
	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/sekeve/internal/adapters/gui/omnibox"
	"github.com/bnema/sekeve/internal/adapters/gui/pin"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/sekeve/pkg/gtkutil"
	lsh "github.com/bnema/sekeve/pkg/layershell"
	"golang.org/x/term"
)

var _ port.GUIPort = (*GUIAdapter)(nil)

// GUIAdapter implements port.GUIPort with GTK4 + layer-shell.
type GUIAdapter struct {
	isTTY        bool
	guiAvailable bool

	// State cache for omnibox persistence (5 min TTL).
	mu         sync.Mutex
	lastState  *omniboxState
	lastActive time.Time
}

const stateTTL = 5 * time.Minute

type omniboxState struct {
	Mode     port.OmniboxMode
	Category int // entity.EntryType as int
	Query    string
}

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

// NewGUIAdapter creates a new adapter, detecting TTY and GUI availability.
func NewGUIAdapter() *GUIAdapter {
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	guiAvailable := checkGTKAvailable()
	return &GUIAdapter{isTTY: isTTY, guiAvailable: guiAvailable}
}

func (a *GUIAdapter) IsTTY() bool { return a.isTTY }

func (a *GUIAdapter) PromptForPIN(ctx context.Context, validate port.PINValidateFunc) error {
	if a.guiAvailable {
		return pin.PromptGUI(ctx, validate, emeraldCSS)
	}
	if a.isTTY {
		fmt.Fprintln(os.Stderr, "sekeve: GUI unavailable, falling back to terminal input")
		return pin.PromptTTY(ctx, validate)
	}
	return port.ErrNoPINInputMethod
}

func (a *GUIAdapter) ShowOmnibox(ctx context.Context, cfg port.OmniboxConfig) error {
	if !a.guiAvailable {
		return fmt.Errorf("omnibox requires a graphical display")
	}
	return a.showOmniboxGUI(ctx, cfg)
}

// saveState snapshots omnibox state for restoration.
func (a *GUIAdapter) saveState(s *omniboxState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastState = s
	a.lastActive = time.Now()
}

// restoreState returns cached state if within TTL, otherwise nil.
func (a *GUIAdapter) restoreState() *omniboxState {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastState == nil || time.Since(a.lastActive) > stateTTL {
		a.lastState = nil
		return nil
	}
	return a.lastState
}

func (a *GUIAdapter) showOmniboxGUI(ctx context.Context, cfg port.OmniboxConfig) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Check for cached state.
	cached := a.restoreState()

	appID := "dev.bnema.sekeve.omnibox"
	app := gtk.NewApplication(&appID, gio.GApplicationNonUniqueValue)

	var ob *omnibox.Omnibox // capture for state saving
	var callbacks []interface{}

	activateCb := func(_ gio.Application) {
		window := gtk.NewApplicationWindow(app)

		title := "Sekeve"
		window.SetTitle(&title)
		window.SetDecorated(false)
		window.SetFocusVisible(false)

		if !setupLayerShell(&window.Window, "sekeve-omnibox") {
			fmt.Fprintln(os.Stderr, "sekeve: WARNING: layer-shell not available, omnibox will open as regular window")
		}
		setupCSS()

		quitFn := func() { app.Quit() }

		ob = omnibox.New(ctx, cfg, quitFn)

		// Center the omnibox inside the full-screen layer-shell surface.
		centerBox := gtk.NewBox(gtk.OrientationVerticalValue, 0)
		centerBox.SetHalign(gtk.AlignCenterValue)
		centerBox.SetValign(gtk.AlignCenterValue)
		centerBox.Append(&ob.Root.Widget)
		window.SetChild(&centerBox.Widget)

		ob.AttachKeyController(&window.Window)

		// Restore cached state if available.
		if cached != nil {
			ob.RestoreState(int(cached.Mode), cached.Category, cached.Query)
		}

		closeRequestCb := func(_ gtk.Window) bool {
			app.Quit()
			return true
		}
		gtkutil.RetainCallback(&callbacks, closeRequestCb)
		window.ConnectCloseRequest(&closeRequestCb)

		window.Show()
		ob.GrabFocus()
	}
	gtkutil.RetainCallback(&callbacks, activateCb)
	app.ConnectActivate(&activateCb)

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
		case <-done:
			return
		}
		// Context cancelled — close the app on the GTK thread.
		select {
		case <-done:
			return
		default:
		}
		gtkutil.IdleAddOnce(func() { app.Quit() })
	}()

	app.Run(0, nil)
	close(done)

	// Save state for next invocation.
	if ob != nil {
		mode, category, query := ob.State()
		a.saveState(&omniboxState{
			Mode:     port.OmniboxMode(mode),
			Category: category,
			Query:    query,
		})
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func setupCSS() {
	cssProvider := gtk.NewCssProvider()
	cssProvider.LoadFromString(emeraldCSS)
	if display := gdk.DisplayGetDefault(); display != nil {
		gtk.StyleContextAddProviderForDisplay(display, cssProvider, 800)
	}
}

func setupLayerShell(window *gtk.Window, namespace string) bool {
	return lsh.InitOverlay(window, lsh.OverlayConfig{
		Namespace: namespace,
		Exclusive: true,
	})
}
