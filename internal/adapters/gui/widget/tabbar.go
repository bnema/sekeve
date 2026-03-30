// internal/adapters/gui/widget/tabbar.go
//go:build linux

package widget

import (
	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/sekeve/pkg/gtkutil"
)

// TabBar is a horizontal row of toggle buttons that acts as a tab strip.
type TabBar struct {
	Box       *gtk.Box
	buttons   []*gtk.Button
	active    int
	onChange  func(index int)
	callbacks []interface{}
}

// TabBarConfig holds configuration for creating a TabBar.
type TabBarConfig struct {
	Labels      []string
	ActiveClass string // CSS class for active tab (e.g. "sekeve-tab-active")
	BaseClass   string // CSS class for all tabs (e.g. "sekeve-tab")
	OnChange    func(index int)
}

// NewTabBar creates a tab bar with the given labels.
func NewTabBar(cfg TabBarConfig) *TabBar {
	box, _ := gtkutil.SafeNewWidget("tabbar-box", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationHorizontalValue, 2)
	})

	t := &TabBar{
		Box:      box,
		active:   0,
		onChange: cfg.OnChange,
	}

	for i, label := range cfg.Labels {
		idx := i
		lbl := label
		btn, _ := gtkutil.SafeNewWidget("tab-"+lbl, func() *gtk.Button {
			return gtk.NewButtonWithLabel(lbl)
		})
		if btn == nil {
			continue
		}
		btn.AddCssClass(cfg.BaseClass)
		btn.SetCanFocus(false)
		if i == 0 {
			btn.AddCssClass(cfg.ActiveClass)
		}

		clickCb := func(_ gtk.Button) {
			t.SetActive(idx)
		}
		gtkutil.RetainCallback(&t.callbacks, clickCb)
		btn.ConnectClicked(&clickCb)

		t.buttons = append(t.buttons, btn)
		box.Append(&btn.Widget)
	}

	return t
}

// SetActive changes the active tab and notifies the callback.
func (t *TabBar) SetActive(index int) {
	if index < 0 || index >= len(t.buttons) {
		return
	}
	if index == t.active {
		return
	}

	// Remove active class from old tab, add to new.
	t.buttons[t.active].RemoveCssClass("sekeve-tab-active")
	t.buttons[t.active].RemoveCssClass("sekeve-category-active")
	t.buttons[index].AddCssClass("sekeve-tab-active")
	t.buttons[index].AddCssClass("sekeve-category-active")

	t.active = index
	if t.onChange != nil {
		t.onChange(index)
	}
}

// Active returns the currently active tab index.
func (t *TabBar) Active() int { return t.active }

// ButtonAt returns the button at the given index for focus ring integration.
func (t *TabBar) ButtonAt(i int) *gtk.Button {
	if i < 0 || i >= len(t.buttons) {
		return nil
	}
	return t.buttons[i]
}

// Len returns the number of tabs.
func (t *TabBar) Len() int { return len(t.buttons) }
