//go:build !linux

package layershell

import "github.com/bnema/puregotk/v4/gtk"

// OverlayConfig configures a layer-shell overlay window.
type OverlayConfig struct {
	Namespace string
	Exclusive bool
}

// InitOverlay is a no-op on non-Linux platforms.
func InitOverlay(_ *gtk.Window, _ OverlayConfig) bool {
	return false
}
