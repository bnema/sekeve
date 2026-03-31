package client

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/domain/service"
)

// resolveOpts holds the flags for entry resolution.
type resolveOpts struct {
	ID     string
	Domain string
	Email  string
	Query  string // positional arg (fuzzy search)
}

// resolveEntry searches for entries matching the given opts and returns exactly one.
// If multiple match and stdout is a TTY, shows an interactive picker.
// If multiple match and not a TTY (or --json), returns an error listing matches.
func resolveEntry(entries []*entity.Envelope, opts resolveOpts) (*entity.Envelope, error) {
	// Direct ID lookup -- caller already fetched by ID, this is for the non-ID path.
	if opts.ID != "" {
		for _, e := range entries {
			if e.ID == opts.ID {
				return e, nil
			}
		}
		return nil, fmt.Errorf("no entry found with id %q", opts.ID)
	}

	searchOpts := service.SearchOpts{
		Domain: opts.Domain,
		Email:  opts.Email,
		Query:  opts.Query,
	}
	matched := service.FilterEntries(entries, searchOpts)

	switch len(matched) {
	case 0:
		return nil, fmt.Errorf("no entries found")
	case 1:
		return matched[0], nil
	default:
		if cliconfig.JSONOutput || !isTerminal() {
			return nil, &AmbiguousMatchError{Matches: matched}
		}
		return pickEntry(matched)
	}
}

// pickEntry shows an interactive picker using bubbletea and returns the selected entry.
func pickEntry(entries []*entity.Envelope) (*entity.Envelope, error) {
	items := make([]pickerItem, len(entries))
	for i, e := range entries {
		items[i] = pickerItem{envelope: e, label: formatPickerLabel(e)}
	}

	m := pickerModel{items: items, selected: -1}
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("picker failed: %w", err)
	}

	final, err := pickerModelFromResult(result)
	if err != nil {
		return nil, err
	}
	if final.selected < 0 || final.selected >= len(entries) {
		return nil, fmt.Errorf("selection cancelled")
	}
	return entries[final.selected], nil
}

func pickerModelFromResult(result tea.Model) (pickerModel, error) {
	final, ok := result.(pickerModel)
	if !ok {
		return pickerModel{}, fmt.Errorf("unexpected picker result type: %T", result)
	}
	return final, nil
}

type pickerItem struct {
	envelope *entity.Envelope
	label    string
}

type pickerModel struct {
	items    []pickerItem
	cursor   int
	selected int
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.cursor
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.selected = -1
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m pickerModel) View() tea.View {
	var b strings.Builder
	b.WriteString("Multiple matches found. Select one:\n\n")
	for i, item := range m.items {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		b.WriteString(cursor + item.label + "\n")
	}
	b.WriteString("\n(j/k or up/down to move, enter to select, esc to cancel)\n")
	return tea.NewView(b.String())
}

// formatPickerLabel creates a display label for the picker.
func formatPickerLabel(e *entity.Envelope) string {
	switch e.Type {
	case entity.EntryTypeLogin:
		site := e.Meta["site"]
		username := e.Meta["username"]
		return fmt.Sprintf("[%s] %s (%s) -- %s", e.Type.String(), e.Name, username, site)
	default:
		return fmt.Sprintf("[%s] %s", e.Type.String(), e.Name)
	}
}

// isTerminal checks if stdout is a terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// AmbiguousMatchError is returned when multiple entries match in non-interactive mode.
type AmbiguousMatchError struct {
	Matches []*entity.Envelope
}

func (e *AmbiguousMatchError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "multiple entries match (%d). Use --id to specify:\n", len(e.Matches))
	for _, m := range e.Matches {
		site := m.Meta["site"]
		username := m.Meta["username"]
		if site != "" || username != "" {
			fmt.Fprintf(&b, "  %s  %s (%s · %s)\n", m.ID, m.Name, username, site)
		} else {
			fmt.Fprintf(&b, "  %s  %s\n", m.ID, m.Name)
		}
	}
	return b.String()
}
