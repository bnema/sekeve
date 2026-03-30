// internal/adapters/gui/omnibox/search.go
//go:build linux && !nogtk

package omnibox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/sekeve/pkg/fuzzysearch"
	"github.com/bnema/sekeve/pkg/gtkutil"
)

const maxResults = 20

// SearchView handles the search entry, result list, and clipboard copy.
type SearchView struct {
	Root    *gtk.Box
	entry   *gtk.SearchEntry
	listBox *gtk.ListBox
	scroll  *gtk.ScrolledWindow

	cfg      port.OmniboxConfig
	ctx      context.Context
	quitFn   func()
	category entity.EntryType

	// Cached entries and current matches for the active category.
	mu        sync.Mutex
	entries   []*entity.Envelope
	names     []string // parallel to entries, for fuzzy search
	matches   []fuzzysearch.Match
	callbacks []interface{}
}

// NewSearchView creates the search entry and result list.
func NewSearchView(ctx context.Context, cfg port.OmniboxConfig, quitFn func()) *SearchView {
	sv := &SearchView{
		cfg:      cfg,
		ctx:      ctx,
		quitFn:   quitFn,
		category: cfg.Category,
	}

	root, _ := gtkutil.SafeNewWidget("search-root", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationVerticalValue, 0)
	})
	sv.Root = root

	// --- Search entry ---
	sv.entry, _ = gtkutil.SafeNewWidget("search-entry", func() *gtk.SearchEntry {
		return gtk.NewSearchEntry()
	})
	if sv.entry != nil {
		placeholder := "Search entries..."
		sv.entry.SetPlaceholderText(&placeholder)
		sv.entry.SetHexpand(true)

		changedCb := func(_ gtk.SearchEntry) {
			sv.onSearchChanged()
		}
		gtkutil.RetainCallback(&sv.callbacks, changedCb)
		sv.entry.ConnectSearchChanged(&changedCb)

		searchRow, _ := gtkutil.SafeNewWidget("search-row", func() *gtk.Box {
			return gtk.NewBox(gtk.OrientationHorizontalValue, 8)
		})
		if searchRow != nil {
			searchRow.AddCssClass("sekeve-search-row")
			searchRow.Append(&sv.entry.Widget)
			root.Append(&searchRow.Widget)
		} else {
			root.Append(&sv.entry.Widget)
		}
	}

	// --- Scrolled result list ---
	sv.scroll, _ = gtkutil.SafeNewWidget("search-scroll", func() *gtk.ScrolledWindow {
		return gtk.NewScrolledWindow()
	})
	sv.listBox, _ = gtkutil.SafeNewWidget("search-listbox", func() *gtk.ListBox {
		return gtk.NewListBox()
	})

	if sv.listBox != nil {
		sv.listBox.SetSelectionMode(gtk.SelectionBrowseValue)

		activatedCb := func(_ gtk.ListBox, row *gtk.ListBoxRow) {
			if row != nil {
				sv.copyEntryAtIndex(row.GetIndex())
			}
		}
		gtkutil.RetainCallback(&sv.callbacks, activatedCb)
		sv.listBox.ConnectRowActivated(&activatedCb)
	}

	if sv.scroll != nil {
		sv.scroll.SetPolicy(gtk.PolicyNeverValue, gtk.PolicyAutomaticValue)
		sv.scroll.SetMaxContentHeight(320)
		sv.scroll.SetPropagateNaturalHeight(true)
		if sv.listBox != nil {
			sv.scroll.SetChild(&sv.listBox.Widget)
		}
		root.Append(&sv.scroll.Widget)
	}

	// Load initial entries.
	go sv.loadEntries()

	return sv
}

// GrabFocus focuses the search entry.
func (sv *SearchView) GrabFocus() {
	if sv.entry != nil {
		sv.entry.GrabFocus()
	}
}

// HasQuery returns true if the search entry has non-empty text.
func (sv *SearchView) HasQuery() bool {
	if sv.entry == nil {
		return false
	}
	return sv.entry.GetText() != ""
}

// ClearQuery clears the search entry and reloads all entries.
func (sv *SearchView) ClearQuery() {
	if sv.entry != nil {
		sv.entry.SetText("")
	}
}

// SetCategory changes the active category and reloads entries.
func (sv *SearchView) SetCategory(cat entity.EntryType) {
	sv.category = cat
	go sv.loadEntries()
}

// SelectNext moves selection down one row in the list.
func (sv *SearchView) SelectNext() {
	if sv.listBox == nil {
		return
	}
	sel := sv.listBox.GetSelectedRow()
	if sel == nil {
		row := sv.listBox.GetRowAtIndex(0)
		if row != nil {
			sv.listBox.SelectRow(row)
		}
		return
	}
	next := sv.listBox.GetRowAtIndex(sel.GetIndex() + 1)
	if next != nil {
		sv.listBox.SelectRow(next)
	}
}

// SelectPrev moves selection up one row in the list.
func (sv *SearchView) SelectPrev() {
	if sv.listBox == nil {
		return
	}
	sel := sv.listBox.GetSelectedRow()
	if sel == nil {
		return
	}
	idx := sel.GetIndex() - 1
	if idx < 0 {
		return
	}
	prev := sv.listBox.GetRowAtIndex(idx)
	if prev != nil {
		sv.listBox.SelectRow(prev)
	}
}

// SelectedEntry returns the envelope for the currently selected row, or nil.
func (sv *SearchView) SelectedEntry() *entity.Envelope {
	if sv.listBox == nil {
		return nil
	}
	sel := sv.listBox.GetSelectedRow()
	if sel == nil {
		return nil
	}

	listIndex := sel.GetIndex()

	sv.mu.Lock()
	entries := sv.entries
	matches := sv.matches
	sv.mu.Unlock()

	var entryIdx int
	if matches != nil {
		if listIndex < 0 || listIndex >= len(matches) {
			return nil
		}
		entryIdx = matches[listIndex].Index
	} else {
		entryIdx = listIndex
	}
	if entryIdx < 0 || entryIdx >= len(entries) {
		return nil
	}
	return entries[entryIdx]
}

// CopySelected copies the value of the currently selected entry.
func (sv *SearchView) CopySelected() {
	if sv.listBox == nil {
		return
	}
	sel := sv.listBox.GetSelectedRow()
	if sel == nil {
		return
	}
	sv.copyEntryAtIndex(sel.GetIndex())
}

// loadEntries fetches entries for the current category and populates the list.
func (sv *SearchView) loadEntries() {
	if sv.cfg.ListEntries == nil {
		return
	}

	entries, err := sv.cfg.ListEntries(sv.ctx, sv.category)
	if err != nil {
		return
	}

	sv.mu.Lock()
	sv.entries = entries
	sv.names = make([]string, len(entries))
	for i, e := range entries {
		sv.names[i] = e.Name
	}
	sv.mu.Unlock()

	// Update UI on the GTK thread.
	gtkutil.IdleAddOnce(func() {
		sv.onSearchChanged()
	})
}

// onSearchChanged filters entries and updates the list box.
func (sv *SearchView) onSearchChanged() {
	if sv.listBox == nil {
		return
	}

	query := ""
	if sv.entry != nil {
		query = sv.entry.GetText()
	}

	sv.mu.Lock()
	entries := sv.entries
	names := sv.names
	sv.mu.Unlock()

	// Clear existing rows.
	sv.listBox.RemoveAll()

	if query == "" {
		// Show all entries (up to maxResults).
		sv.mu.Lock()
		sv.matches = nil
		sv.mu.Unlock()

		limit := maxResults
		if len(entries) < limit {
			limit = len(entries)
		}
		for i := 0; i < limit; i++ {
			row := sv.buildRow(entries[i])
			if row != nil {
				sv.listBox.Append(&row.Widget)
			}
		}
	} else {
		// Fuzzy search.
		matches := fuzzysearch.Search(query, names, maxResults)

		sv.mu.Lock()
		sv.matches = matches
		sv.mu.Unlock()

		for _, m := range matches {
			if m.Index < len(entries) {
				row := sv.buildRow(entries[m.Index])
				if row != nil {
					sv.listBox.Append(&row.Widget)
				}
			}
		}
	}

	// Select first row if available.
	first := sv.listBox.GetRowAtIndex(0)
	if first != nil {
		sv.listBox.SelectRow(first)
	}
}

// buildRow creates a ListBoxRow for an envelope.
func (sv *SearchView) buildRow(env *entity.Envelope) *gtk.ListBoxRow {
	row, _ := gtkutil.SafeNewWidget("result-row", func() *gtk.ListBoxRow {
		return gtk.NewListBoxRow()
	})
	if row == nil {
		return nil
	}
	row.AddCssClass("sekeve-row")

	hbox, _ := gtkutil.SafeNewWidget("row-hbox", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationHorizontalValue, 8)
	})
	if hbox == nil {
		return row
	}

	// Type icon label.
	iconText := typeIcon(env.Type)
	iconLabel, _ := gtkutil.SafeNewWidget("row-icon", func() *gtk.Label {
		return gtk.NewLabel(&iconText)
	})
	if iconLabel != nil {
		iconLabel.AddCssClass(typeIconClass(env.Type))
		hbox.Append(&iconLabel.Widget)
	}

	// Entry name.
	nameLabel, _ := gtkutil.SafeNewWidget("row-name", func() *gtk.Label {
		return gtk.NewLabel(&env.Name)
	})
	if nameLabel != nil {
		nameLabel.AddCssClass("sekeve-row-name")
		nameLabel.SetHexpand(true)
		nameLabel.SetHalign(gtk.AlignStartValue)
		hbox.Append(&nameLabel.Widget)
	}

	// Meta text (username for login, type for others).
	meta := typeMeta(env)
	metaLabel, _ := gtkutil.SafeNewWidget("row-meta", func() *gtk.Label {
		return gtk.NewLabel(&meta)
	})
	if metaLabel != nil {
		metaLabel.AddCssClass("sekeve-row-meta")
		metaLabel.SetHalign(gtk.AlignEndValue)
		hbox.Append(&metaLabel.Widget)
	}

	row.SetChild(&hbox.Widget)
	return row
}

// copyEntryAtIndex decrypts the entry at the given list index and copies
// the main value to the system clipboard, then closes the overlay.
func (sv *SearchView) copyEntryAtIndex(listIndex int) {
	sv.mu.Lock()
	entries := sv.entries
	matches := sv.matches
	sv.mu.Unlock()

	// Map list index back to the entry index.
	var entryIdx int
	if matches != nil {
		if listIndex < 0 || listIndex >= len(matches) {
			return
		}
		entryIdx = matches[listIndex].Index
	} else {
		entryIdx = listIndex
	}
	if entryIdx < 0 || entryIdx >= len(entries) {
		return
	}

	env := entries[entryIdx]

	if sv.cfg.GetEntry == nil || sv.cfg.DecryptAndUse == nil {
		return
	}

	go func() {
		// Re-fetch for fresh payload.
		full, err := sv.cfg.GetEntry(sv.ctx, env.ID)
		if err != nil {
			return
		}

		sv.cfg.DecryptAndUse(sv.ctx, full.Payload, func(plaintext []byte) {
			value := extractMainValue(full.Type, plaintext)
			if value == "" {
				return
			}
			copyToClipboard(sv.ctx, value)
		})

		// Close overlay on the GTK thread.
		gtkutil.IdleAddOnce(func() {
			if sv.quitFn != nil {
				sv.quitFn()
			}
		})
	}()
}

// extractMainValue unmarshals the plaintext based on entry type and
// returns the primary copyable value.
func extractMainValue(t entity.EntryType, plaintext []byte) string {
	switch t {
	case entity.EntryTypeLogin:
		var login entity.Login
		if err := json.Unmarshal(plaintext, &login); err != nil {
			return ""
		}
		return login.Password
	case entity.EntryTypeSecret:
		var secret entity.Secret
		if err := json.Unmarshal(plaintext, &secret); err != nil {
			return ""
		}
		return secret.Value
	case entity.EntryTypeNote:
		var note entity.Note
		if err := json.Unmarshal(plaintext, &note); err != nil {
			return ""
		}
		return note.Content
	default:
		return string(plaintext)
	}
}

// copyToClipboard writes value to the system clipboard via wl-copy or xclip.
func copyToClipboard(ctx context.Context, value string) {
	cmd, name := clipboardCmd(ctx)
	cmd.Stdin = strings.NewReader(value)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sekeve: %s failed: %v\n", name, err)
	}
}

func clipboardCmd(ctx context.Context) (*exec.Cmd, string) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return exec.CommandContext(ctx, "wl-copy"), "wl-copy"
	}
	return exec.CommandContext(ctx, "xclip", "-selection", "clipboard"), "xclip"
}

func typeIcon(t entity.EntryType) string {
	switch t {
	case entity.EntryTypeLogin:
		return "\U0001F511" // key emoji
	case entity.EntryTypeNote:
		return "\U0001F4C4" // page emoji
	case entity.EntryTypeSecret:
		return "\U0001F512" // lock emoji
	default:
		return "\u2022" // bullet
	}
}

func typeIconClass(t entity.EntryType) string {
	switch t {
	case entity.EntryTypeLogin:
		return "sekeve-icon-login"
	case entity.EntryTypeNote:
		return "sekeve-icon-note"
	case entity.EntryTypeSecret:
		return "sekeve-icon-secret"
	default:
		return "sekeve-icon-search"
	}
}

func typeMeta(env *entity.Envelope) string {
	switch env.Type {
	case entity.EntryTypeLogin:
		if u, ok := env.Meta["username"]; ok && u != "" {
			return u
		}
		return env.Name
	case entity.EntryTypeNote:
		return "note"
	case entity.EntryTypeSecret:
		return "secret"
	default:
		return ""
	}
}
