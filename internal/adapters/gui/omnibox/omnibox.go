// internal/adapters/gui/omnibox/omnibox.go
//go:build linux && !nogtk

package omnibox

import (
	"context"
	"fmt"
	"os"

	"github.com/bnema/puregotk/v4/gdk"
	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/sekeve/internal/adapters/gui/widget"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/sekeve/pkg/focusring"
	"github.com/bnema/sekeve/pkg/gtkutil"
)

// Omnibox is the main container widget with L1/L2 tabs, a search view,
// an add view, a detail view, and keyboard routing.
type Omnibox struct {
	Root *gtk.Box

	cfg   port.OmniboxConfig
	ctx   context.Context
	quitF func() // called to close the overlay

	modeBar     *widget.TabBar
	categoryBar *widget.TabBar
	search      *SearchView
	addView     *AddView
	detailView  *DetailView
	currentMode int // 0=search, 1=add, 2=detail

	ring      *focusring.Ring
	callbacks []interface{}
}

// New creates the omnibox widget and returns it. quitFn is called when
// the user presses Escape on empty search or copies a value.
func New(ctx context.Context, cfg port.OmniboxConfig, quitFn func()) *Omnibox {
	o := &Omnibox{
		cfg:   cfg,
		ctx:   ctx,
		quitF: quitFn,
	}

	root, _ := gtkutil.SafeNewWidget("omnibox-root", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationVerticalValue, 0)
	})
	root.AddCssClass("sekeve-overlay")
	o.Root = root

	// --- L1 mode tabs (Search / Add) ---
	o.modeBar = widget.NewTabBar(widget.TabBarConfig{
		Labels:      []string{"Search", "Add"},
		ActiveClass: "sekeve-tab-active",
		BaseClass:   "sekeve-tab",
		OnChange: func(index int) {
			o.setMode(index)
		},
	})
	if o.modeBar.Box != nil {
		o.modeBar.Box.AddCssClass("sekeve-header")
		root.Append(&o.modeBar.Box.Widget)
	}

	// --- L2 category tabs (All / Login / Note / Secret) ---
	o.categoryBar = widget.NewTabBar(widget.TabBarConfig{
		Labels:      []string{"All", "Login", "Note", "Secret"},
		ActiveClass: "sekeve-category-active",
		BaseClass:   "sekeve-category",
		OnChange: func(index int) {
			o.onCategoryChange(index)
		},
	})
	if o.categoryBar.Box != nil {
		o.categoryBar.Box.AddCssClass("sekeve-category-bar")
		root.Append(&o.categoryBar.Box.Widget)
	}

	// Set initial category tab if provided.
	if cfg.Category != entity.EntryTypeUnspecified {
		idx := categoryToIndex(cfg.Category)
		o.categoryBar.SetActive(idx)
	}

	// --- Search view ---
	o.search = NewSearchView(ctx, cfg, quitFn)
	if o.search.Root != nil {
		root.Append(&o.search.Root.Widget)
	}

	// --- Add view ---
	o.addView = NewAddView(ctx, cfg, func() {
		o.switchToSearch()
	})
	if o.addView.Root != nil {
		o.addView.Hide()
		root.Append(&o.addView.Root.Widget)
	}

	// --- Detail view ---
	o.detailView = NewDetailView(ctx, cfg, func() {
		o.switchToSearch()
	})
	if o.detailView.Root != nil {
		root.Append(&o.detailView.Root.Widget)
	}

	// --- Footer hints ---
	footer := buildFooter(quitFn, &o.callbacks)
	if footer != nil {
		root.Append(&footer.Widget)
	}

	// --- Focus ring ---
	o.ring = focusring.New()
	o.rebuildFocusRing()

	// If configured to start in Add mode, switch now.
	if cfg.Mode == port.OmniboxModeAdd {
		o.modeBar.SetActive(1)
	}

	return o
}

// State returns the current omnibox state for caching.
func (o *Omnibox) State() (mode int, category int, query string) {
	mode = o.currentMode
	category = o.categoryBar.Active()
	if o.search != nil && o.search.entry != nil {
		query = o.search.entry.GetText()
	}
	return
}

// RestoreState applies a previously cached state.
func (o *Omnibox) RestoreState(mode int, category int, query string) {
	if category > 0 {
		o.categoryBar.SetActive(category)
	}
	if query != "" && o.search != nil && o.search.entry != nil {
		o.search.entry.SetText(query)
	}
	if mode == 1 {
		o.modeBar.SetActive(1)
	}
}

// AttachKeyController creates a key controller and attaches it to the
// given window. The controller routes global keys (Escape, Tab, Ctrl+1-4,
// Up/Down/Enter to the search view).
func (o *Omnibox) AttachKeyController(window *gtk.Window) {
	keyCtrl := gtk.NewEventControllerKey()
	// Use capture phase so we intercept keys before child widgets
	// (e.g. SearchEntry consuming Escape via stop-search).
	keyCtrl.SetPropagationPhase(gtk.PhaseCaptureValue)

	keyPressedCb := func(_ gtk.EventControllerKey, keyval uint, _ uint, state gdk.ModifierType) bool {
		ctrl := state&gdk.ControlMaskValue != 0

		fmt.Fprintf(os.Stderr, "sekeve: key pressed: keyval=%d (0x%x) ctrl=%v\n", keyval, keyval, ctrl)

		switch int(keyval) {
		case gdk.KEY_Escape:
			fmt.Fprintln(os.Stderr, "sekeve: Escape pressed")
			return o.handleEscape()

		case gdk.KEY_Tab:
			fmt.Fprintf(os.Stderr, "sekeve: Tab pressed, ring has %d widgets\n", len(o.ring.Widgets()))
			w := o.ring.Next()
			fmt.Fprintf(os.Stderr, "sekeve: Tab → Next() returned %v\n", w)
			return true

		case gdk.KEY_ISO_Left_Tab: // Shift+Tab
			fmt.Fprintln(os.Stderr, "sekeve: Shift+Tab pressed")
			w := o.ring.Prev()
			fmt.Fprintf(os.Stderr, "sekeve: Shift+Tab → Prev() returned %v\n", w)
			return true

		case gdk.KEY_1:
			if ctrl {
				o.categoryBar.SetActive(0)
				return true
			}
		case gdk.KEY_2:
			if ctrl {
				o.categoryBar.SetActive(1)
				return true
			}
		case gdk.KEY_3:
			if ctrl {
				o.categoryBar.SetActive(2)
				return true
			}
		case gdk.KEY_4:
			if ctrl {
				o.categoryBar.SetActive(3)
				return true
			}

		case gdk.KEY_Up:
			if o.currentMode == 0 {
				o.search.SelectPrev()
				return true
			}
		case gdk.KEY_Down:
			if o.currentMode == 0 {
				o.search.SelectNext()
				return true
			}
		case gdk.KEY_Return:
			// If a tab button has focus, let GTK handle Enter to click it.
			if o.isTabButtonFocused() {
				return false
			}
			if o.currentMode == 0 {
				if ctrl {
					o.openDetail()
					return true
				}
				o.search.CopySelected()
				return true
			}
		}

		return false
	}

	gtkutil.RetainCallback(&o.callbacks, keyPressedCb)
	keyCtrl.ConnectKeyPressed(&keyPressedCb)
	window.AddController(&keyCtrl.EventController)
}

// GrabFocus sets initial focus to the search entry.
func (o *Omnibox) GrabFocus() {
	if o.search != nil {
		o.search.GrabFocus()
	}
}

// handleEscape in detail mode or add mode switches back to search;
// in search mode clears query text if non-empty, otherwise closes the overlay.
func (o *Omnibox) handleEscape() bool {
	if o.currentMode != 0 {
		o.switchToSearch()
		return true
	}
	if o.search != nil && o.search.HasQuery() {
		o.search.ClearQuery()
		return true
	}
	if o.quitF != nil {
		o.quitF()
	}
	return true
}

// onCategoryChange reloads the search results for the new category,
// and rebuilds the add form if currently in add mode.
func (o *Omnibox) onCategoryChange(index int) {
	cat := indexToCategory(index)
	o.search.SetCategory(cat)
	if o.currentMode == 1 && o.addView != nil {
		o.addView.SetCategory(cat)
		o.rebuildFocusRing()
	}
}

// setMode switches between search (0) and add (1) modes.
func (o *Omnibox) setMode(index int) {
	o.currentMode = index

	// Always hide detail view when switching via tab bar.
	if o.detailView != nil {
		o.detailView.Hide()
	}
	// Restore tab bars visibility.
	if o.modeBar.Box != nil {
		o.modeBar.Box.SetVisible(true)
	}
	if o.categoryBar.Box != nil {
		o.categoryBar.Box.SetVisible(true)
	}

	if index == 0 {
		// Search mode.
		if o.addView != nil {
			o.addView.Hide()
		}
		if o.search != nil && o.search.Root != nil {
			o.search.Root.SetVisible(true)
		}
		o.rebuildFocusRing()
		o.search.GrabFocus()
	} else {
		// Add mode.
		if o.search != nil && o.search.Root != nil {
			o.search.Root.SetVisible(false)
		}
		if o.addView != nil {
			cat := indexToCategory(o.categoryBar.Active())
			o.addView.SetCategory(cat)
			o.addView.Show()
		}
		o.rebuildFocusRing()
	}
}

// switchToSearch resets the mode bar to search and switches views.
// Called by the AddView/DetailView after save or cancel.
func (o *Omnibox) switchToSearch() {
	o.modeBar.SetActive(0) // update tab bar CSS
	o.setMode(0)           // always apply mode change (handles visibility)
	// Reload search entries to pick up any newly added or updated entry.
	if o.search != nil {
		go o.search.loadEntries()
	}
}

// openDetail gets the selected entry, decrypts it, and shows the detail view.
func (o *Omnibox) openDetail() {
	env := o.search.SelectedEntry()
	if env == nil {
		return
	}

	if o.cfg.GetEntry == nil || o.cfg.DecryptAndUse == nil {
		return
	}

	go func() {
		full, err := o.cfg.GetEntry(o.ctx, env.ID)
		if err != nil {
			return
		}

		o.cfg.DecryptAndUse(o.ctx, full.Payload, func(plaintext []byte) {
			// Copy plaintext so it survives the callback scope.
			pt := make([]byte, len(plaintext))
			copy(pt, plaintext)

			gtkutil.IdleAddOnce(func() {
				o.switchToDetailView(full, pt)
			})
		})
	}()
}

// switchToDetailView hides search/add and shows the detail view.
func (o *Omnibox) switchToDetailView(env *entity.Envelope, plaintext []byte) {
	o.currentMode = 2

	// Hide search and tab bars.
	if o.search != nil && o.search.Root != nil {
		o.search.Root.SetVisible(false)
	}
	if o.addView != nil {
		o.addView.Hide()
	}
	if o.modeBar.Box != nil {
		o.modeBar.Box.SetVisible(false)
	}
	if o.categoryBar.Box != nil {
		o.categoryBar.Box.SetVisible(false)
	}

	// Show detail view.
	if o.detailView != nil {
		o.detailView.Show(env, plaintext)
	}

	o.rebuildFocusRing()
}

// rebuildFocusRing updates the focus ring with current focusable widgets.
func (o *Omnibox) rebuildFocusRing() {
	var widgets []focusring.Focusable
	switch o.currentMode {
	case 1: // add mode
		if o.addView != nil {
			widgets = append(widgets, o.addView.Focusables()...)
		}
	case 2: // detail mode
		if o.detailView != nil {
			widgets = append(widgets, o.detailView.Focusables()...)
		}
	default: // search mode
		if o.search != nil && o.search.entry != nil {
			widgets = append(widgets, &focusableWidget{&o.search.entry.Widget})
		}
		if o.search != nil && o.search.listBox != nil {
			widgets = append(widgets, &focusableWidget{&o.search.listBox.Widget})
		}
		// Add mode tabs (Search, Add) to focus ring so Tab cycles through them.
		for i := 0; i < o.modeBar.Len(); i++ {
			if btn := o.modeBar.ButtonAt(i); btn != nil {
				widgets = append(widgets, &focusableWidget{&btn.Widget})
			}
		}
	}
	fmt.Fprintf(os.Stderr, "sekeve: rebuildFocusRing mode=%d widgets=%d\n", o.currentMode, len(widgets))
	o.ring.SetWidgets(widgets...)
}

// isTabButtonFocused returns true if any mode tab button currently has focus.
func (o *Omnibox) isTabButtonFocused() bool {
	for i := 0; i < o.modeBar.Len(); i++ {
		if btn := o.modeBar.ButtonAt(i); btn != nil && btn.HasFocus() {
			return true
		}
	}
	return false
}

// focusableWidget adapts a gtk.Widget (whose GrabFocus returns bool)
// to the focusring.Focusable interface (whose GrabFocus returns nothing).
type focusableWidget struct {
	w *gtk.Widget
}

func (f *focusableWidget) GrabFocus() {
	ok := f.w.GrabFocus()
	fmt.Fprintf(os.Stderr, "sekeve: GrabFocus on %p → %v, HasFocus=%v\n", f.w.GoPointer(), ok, f.w.HasFocus())
}
func (f *focusableWidget) HasFocus() bool    { return f.w.HasFocus() }
func (f *focusableWidget) SetVisible(v bool) { f.w.SetVisible(v) }
func (f *focusableWidget) GetVisible() bool  { return f.w.GetVisible() }

func buildFooter(quitFn func(), callbacks *[]interface{}) *gtk.Box {
	footerBox, _ := gtkutil.SafeNewWidget("footer-box", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationHorizontalValue, 14)
	})
	if footerBox == nil {
		return nil
	}
	footerBox.AddCssClass("sekeve-footer")

	hints := []struct{ key, action string }{
		{"Enter", "copy"},
		{"^Enter", "details"},
		{"Tab", "next"},
		{"Esc", "close"},
	}
	for _, h := range hints {
		hintBox, _ := gtkutil.SafeNewWidget("hint", func() *gtk.Box {
			return gtk.NewBox(gtk.OrientationHorizontalValue, 2)
		})
		if hintBox == nil {
			continue
		}
		kbdLabel, _ := gtkutil.SafeNewWidget("kbd", func() *gtk.Label {
			k := h.key
			return gtk.NewLabel(&k)
		})
		if kbdLabel != nil {
			kbdLabel.AddCssClass("sekeve-kbd")
			hintBox.Append(&kbdLabel.Widget)
		}
		actionLabel, _ := gtkutil.SafeNewWidget("action", func() *gtk.Label {
			a := h.action
			return gtk.NewLabel(&a)
		})
		if actionLabel != nil {
			hintBox.Append(&actionLabel.Widget)
		}
		footerBox.Append(&hintBox.Widget)
	}

	// Spacer to push Quit button to the right.
	spacer, _ := gtkutil.SafeNewWidget("spacer", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationHorizontalValue, 0)
	})
	if spacer != nil {
		spacer.SetHexpand(true)
		footerBox.Append(&spacer.Widget)
	}

	// Clickable Quit button.
	quitBtn, _ := gtkutil.SafeNewWidget("quit-btn", func() *gtk.Button {
		return gtk.NewButtonWithLabel("✕ Quit")
	})
	if quitBtn != nil {
		quitBtn.AddCssClass("sekeve-quit-btn")
		clickCb := func(_ gtk.Button) {
			if quitFn != nil {
				quitFn()
			}
		}
		gtkutil.RetainCallback(callbacks, clickCb)
		quitBtn.ConnectClicked(&clickCb)
		footerBox.Append(&quitBtn.Widget)
	}

	return footerBox
}

// clearBoxChildren removes all children from a GTK Box.
func clearBoxChildren(box *gtk.Box) {
	for {
		child := box.GetFirstChild()
		if child == nil {
			break
		}
		box.Remove(child)
	}
}

// categoryToIndex maps an entity.EntryType to a tab index.
func categoryToIndex(t entity.EntryType) int {
	switch t {
	case entity.EntryTypeLogin:
		return 1
	case entity.EntryTypeNote:
		return 2
	case entity.EntryTypeSecret:
		return 3
	default:
		return 0
	}
}

// indexToCategory maps a tab index to an entity.EntryType.
func indexToCategory(index int) entity.EntryType {
	switch index {
	case 1:
		return entity.EntryTypeLogin
	case 2:
		return entity.EntryTypeNote
	case 3:
		return entity.EntryTypeSecret
	default:
		return entity.EntryTypeUnspecified
	}
}
