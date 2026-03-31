// internal/adapters/gui/widget/icons.go
//go:build linux && !nogtk

package widget

import (
	"fmt"
	"strings"

	"github.com/bnema/puregotk/v4/gdkpixbuf"
	"github.com/bnema/puregotk/v4/gtk"
)

// Lucide SVG icon templates (16x16 rendered, 24x24 viewBox, stroke-based).
const (
	IconKey      = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.814-.814a6.5 6.5 0 1 0-4-4z"/><circle cx="16.5" cy="7.5" r=".5" fill="currentColor"/></svg>`
	IconFileText = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 13H8"/><path d="M16 17H8"/><path d="M16 13h-2"/></svg>`
	IconLock     = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/><circle cx="12" cy="16" r="1"/></svg>`
	IconSearch   = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>`
	IconCopy     = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>`
	IconBack     = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m15 18-6-6 6-6"/></svg>`

	ColorLogin  = "#34d399"
	ColorNote   = "#6ee7b7"
	ColorSecret = "#a7f3d0"
	ColorSearch = "#3a6a56"
	ColorCopy   = "#4a7a66"
	ColorBack   = "#34d399"
)

// NewIconImage renders an SVG template with the given color and returns a GtkImage.
func NewIconImage(svgTemplate, color string) *gtk.Image {
	svg := strings.ReplaceAll(svgTemplate, "currentColor", color)
	data := []byte(svg)

	loader, err := gdkpixbuf.NewPixbufLoaderWithType("svg")
	if err != nil {
		fmt.Printf("sekeve: icon loader error: %v\n", err)
		return nil
	}

	if _, err := loader.Write(data, uint(len(data))); err != nil {
		fmt.Printf("sekeve: icon write error: %v\n", err)
		return nil
	}

	if _, err := loader.Close(); err != nil {
		fmt.Printf("sekeve: icon close error: %v\n", err)
		return nil
	}

	pixbuf := loader.GetPixbuf()
	if pixbuf == nil {
		return nil
	}

	return gtk.NewImageFromPixbuf(pixbuf)
}
