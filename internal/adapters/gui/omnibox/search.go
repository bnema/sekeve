// internal/adapters/gui/omnibox/search.go
//go:build linux && !nogtk

package omnibox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/sekeve/internal/adapters/gui/widget"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/sekeve/pkg/clipboard"
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

	// allEntries holds the full unfiltered list fetched from the server.
	// entries/names are the category-filtered subset used for display.
	mu         sync.Mutex
	allEntries []*entity.Envelope
	allNames   []string
	entries    []*entity.Envelope
	names      []string // parallel to entries, for fuzzy search
	matches    []fuzzysearch.Match

	debounceTimer *time.Timer
	callbacks     []interface{}
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
	sv.entry, _ = gtkutil.SafeNewWidget("search-entry", gtk.NewSearchEntry)
	if sv.entry != nil {
		placeholder := "Search entries..."
		sv.entry.SetPlaceholderText(&placeholder)
		sv.entry.SetHexpand(true)

		changedCb := func(_ gtk.SearchEntry) {
			if sv.debounceTimer != nil {
				sv.debounceTimer.Stop()
			}
			sv.debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
				gtkutil.IdleAddOnce(func() {
					sv.onSearchChanged()
				})
			})
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
	sv.scroll, _ = gtkutil.SafeNewWidget("search-scroll", gtk.NewScrolledWindow)
	sv.listBox, _ = gtkutil.SafeNewWidget("search-listbox", gtk.NewListBox)

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
		sv.scroll.SetMinContentHeight(200)
		sv.scroll.SetMaxContentHeight(400)
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

// SetCategory changes the active category and filters cached entries.
func (sv *SearchView) SetCategory(cat entity.EntryType) {
	sv.category = cat
	sv.filterByCategory()
	sv.onSearchChanged()
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

// loadEntries fetches all entries and filters by the current category.
func (sv *SearchView) loadEntries() {
	if sv.cfg.ListEntries == nil {
		return
	}

	entries, err := sv.cfg.ListEntries(sv.ctx, entity.EntryTypeUnspecified)
	if err != nil {
		return
	}

	sv.mu.Lock()
	sv.allEntries = entries
	sv.allNames = make([]string, len(entries))
	for i, e := range entries {
		sv.allNames[i] = e.Name
	}
	sv.mu.Unlock()

	// Filter and update UI on the GTK thread.
	gtkutil.IdleAddOnce(func() {
		sv.filterByCategory()
		sv.onSearchChanged()
	})
}

// filterByCategory rebuilds entries/names from allEntries for the current category.
func (sv *SearchView) filterByCategory() {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if sv.category == entity.EntryTypeUnspecified {
		sv.entries = sv.allEntries
		sv.names = sv.allNames
		return
	}

	filtered := make([]*entity.Envelope, 0, len(sv.allEntries))
	filteredNames := make([]string, 0, len(sv.allEntries))
	for i, e := range sv.allEntries {
		if e.Type == sv.category {
			filtered = append(filtered, e)
			filteredNames = append(filteredNames, sv.allNames[i])
		}
	}
	sv.entries = filtered
	sv.names = filteredNames
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
	row, _ := gtkutil.SafeNewWidget("result-row", gtk.NewListBoxRow)
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

	// Type icon (SVG).
	svgTpl, color := typeIconSVG(env.Type)
	iconImg := widget.NewIconImage(svgTpl, color)
	if iconImg != nil {
		hbox.Append(&iconImg.Widget)
	}

	// Entry name — strip parenthesized username for logins since meta shows it.
	displayName := env.Name
	if env.Type == entity.EntryTypeLogin {
		if idx := strings.Index(displayName, " ("); idx > 0 {
			displayName = displayName[:idx]
		}
	}
	nameLabel, _ := gtkutil.SafeNewWidget("row-name", func() *gtk.Label {
		return gtk.NewLabel(&displayName)
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
			sendNotify(sv.ctx, sv.cfg, "Sekeve", "Failed to fetch entry", port.UrgencyCritical, "dialog-error")
			return
		}

		var copyOK bool
		if err := sv.cfg.DecryptAndUse(sv.ctx, full.Payload, func(plaintext []byte) {
			value := extractMainValue(full.Type, plaintext)
			if value == "" {
				sendNotify(sv.ctx, sv.cfg, "Sekeve", "Entry has no copyable value", port.UrgencyNormal, "dialog-warning")
				return
			}
			if err := clipboard.Copy(sv.ctx, value); err != nil {
				sendNotify(sv.ctx, sv.cfg, "Sekeve", fmt.Sprintf("Clipboard copy failed: %v", err), port.UrgencyCritical, "dialog-error")
				fmt.Fprintf(os.Stderr, "sekeve: %v\n", err)
				return
			}
			copyOK = true
			sendNotify(sv.ctx, sv.cfg, "Sekeve", fmt.Sprintf("Copied %s to clipboard", full.Name), port.UrgencyLow, "")
		}); err != nil {
			sendNotify(sv.ctx, sv.cfg, "Sekeve", "Failed to decrypt entry", port.UrgencyCritical, "dialog-error")
			return
		}

		if !copyOK {
			return
		}

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

func typeIconSVG(t entity.EntryType) (svgTemplate, color string) {
	switch t {
	case entity.EntryTypeLogin:
		return widget.IconKey, widget.ColorLogin
	case entity.EntryTypeNote:
		return widget.IconFileText, widget.ColorNote
	case entity.EntryTypeSecret:
		return widget.IconLock, widget.ColorSecret
	default:
		return widget.IconSearch, widget.ColorSearch
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
