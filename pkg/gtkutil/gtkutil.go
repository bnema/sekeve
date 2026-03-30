package gtkutil

import (
	"fmt"

	"github.com/bnema/puregotk/v4/glib"
)

// RetainCallback appends cb to the slice to prevent Go GC from collecting
// callbacks that GTK still references via C pointers.
func RetainCallback(callbacks *[]interface{}, cb interface{}) {
	*callbacks = append(*callbacks, cb)
}

// IdleAddOnce schedules fn to run once on the GTK main thread.
func IdleAddOnce(fn func()) {
	onceFn := glib.SourceOnceFunc(func(uintptr) { fn() })
	glib.IdleAddOnce(&onceFn, 0)
}

// SafeNewWidget creates a widget and returns an error if the constructor returns nil.
func SafeNewWidget[T any](name string, constructor func() *T) (*T, error) {
	w := constructor()
	if w == nil {
		return nil, fmt.Errorf("failed to create GTK widget: %s", name)
	}
	return w, nil
}
