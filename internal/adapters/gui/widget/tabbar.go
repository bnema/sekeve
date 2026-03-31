// internal/adapters/gui/widget/tabbar.go
//go:build linux && !nogtk

package widget

import (
	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/sekeve/pkg/gtkutil"
)

// TabBar is a horizontal row of toggle buttons that acts as a tab strip.
type TabBar struct {
	Box         *gtk.Box
	buttons     []*gtk.Button
	active      int
	activeClass string // CSS class for the active tab
	onChange    func(index int)
	callbacks   []interface{}
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
		Box:         box,
		active:      0,
		activeClass: cfg.ActiveClass,
		onChange:    cfg.OnChange,
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

	// Remove active class from old tab, add to new.
	t.buttons[t.active].RemoveCssClass(t.activeClass)
	t.buttons[index].AddCssClass(t.activeClass)

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

// SetButtonVisible shows or hides the button at the given index.
func (t *TabBar) SetButtonVisible(i int, visible bool) {
	if i >= 0 && i < len(t.buttons) {
		t.buttons[i].SetVisible(visible)
	}
}
