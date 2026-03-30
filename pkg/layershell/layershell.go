//go:build linux

package layershell

import (
	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/puregotk/v4/layershell"
)

// OverlayConfig configures a layer-shell overlay window.
type OverlayConfig struct {
	Namespace string
	Exclusive bool // if true, sets exclusive keyboard mode
}

// InitOverlay configures a GTK window as a layer-shell overlay.
// Returns false if layer-shell is not available (e.g. running on X11).
func InitOverlay(window *gtk.Window, cfg OverlayConfig) bool {
	if !layershell.Available() || !layershell.IsSupported() {
		return false
	}

	layershell.InitForWindow(window)
	layershell.SetLayer(window, layershell.LayerOverlayValue)
	layershell.SetExclusiveZone(window, 0)

	if cfg.Exclusive {
		layershell.SetKeyboardMode(window, layershell.KeyboardModeExclusiveValue)
	}

	if cfg.Namespace != "" {
		layershell.SetNamespace(window, &cfg.Namespace)
	}

	return true
}
