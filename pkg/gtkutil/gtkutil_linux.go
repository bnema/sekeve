//go:build linux && !nogtk

package gtkutil

import "github.com/bnema/puregotk/v4/glib"

// IdleAddOnce schedules fn to run once on the GTK main thread.
func IdleAddOnce(fn func()) {
	onceFn := glib.SourceOnceFunc(func(uintptr) { fn() })
	glib.IdleAddOnce(&onceFn, 0)
}
