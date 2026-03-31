// internal/adapters/gui/widget/entry.go
//go:build linux && !nogtk

package widget

import (
	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/sekeve/pkg/gtkutil"
)

// LabeledEntry is a labeled text input field.
type LabeledEntry struct {
	Box   *gtk.Box
	Label *gtk.Label
	Entry *gtk.Entry
}

// NewLabeledEntry creates a label + entry pair in a vertical box.
func NewLabeledEntry(labelText, placeholder string) *LabeledEntry {
	box, _ := gtkutil.SafeNewWidget("field-box", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationVerticalValue, 3)
	})

	label, _ := gtkutil.SafeNewWidget("field-label", func() *gtk.Label {
		return gtk.NewLabel(&labelText)
	})
	if label != nil {
		label.AddCssClass("sekeve-label")
		label.SetHalign(gtk.AlignStartValue)
	}

	entry, _ := gtkutil.SafeNewWidget("field-entry", gtk.NewEntry)
	if entry != nil {
		entry.SetPlaceholderText(&placeholder)
		entry.SetHexpand(true)
	}

	if box != nil {
		if label != nil {
			box.Append(&label.Widget)
		}
		if entry != nil {
			box.Append(&entry.Widget)
		}
	}

	return &LabeledEntry{Box: box, Label: label, Entry: entry}
}

// LabeledPassword is a labeled password input field.
type LabeledPassword struct {
	Box   *gtk.Box
	Label *gtk.Label
	Entry *gtk.PasswordEntry
}

// NewLabeledPassword creates a label + password entry pair.
func NewLabeledPassword(labelText, placeholder string) *LabeledPassword {
	box, _ := gtkutil.SafeNewWidget("field-box", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationVerticalValue, 3)
	})

	label, _ := gtkutil.SafeNewWidget("field-label", func() *gtk.Label {
		return gtk.NewLabel(&labelText)
	})
	if label != nil {
		label.AddCssClass("sekeve-label")
		label.SetHalign(gtk.AlignStartValue)
	}

	entry, _ := gtkutil.SafeNewWidget("field-pw", gtk.NewPasswordEntry)
	if entry != nil {
		entry.SetPropertyPlaceholderText(placeholder)
		entry.SetHexpand(true)
	}

	if box != nil {
		if label != nil {
			box.Append(&label.Widget)
		}
		if entry != nil {
			box.Append(&entry.Widget)
		}
	}

	return &LabeledPassword{Box: box, Label: label, Entry: entry}
}
