package server

import (
	"fmt"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	tuiTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("67"))

	tuiHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))
)

type pubkeyModel struct {
	textarea textarea.Model
	result   string
	err      error
	done     bool
}

func newPubkeyModel() pubkeyModel {
	ta := textarea.New()
	ta.Placeholder = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n...\n-----END PGP PUBLIC KEY BLOCK-----"
	ta.SetWidth(80)
	ta.SetHeight(15)
	ta.ShowLineNumbers = false
	ta.Focus() // Sets internal focus state; not redundant — pointer receiver mutates ta.

	return pubkeyModel{
		textarea: ta,
	}
}

func (m pubkeyModel) Init() tea.Cmd {
	return m.textarea.Focus()
}

func (m pubkeyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.err = fmt.Errorf("cancelled")
			m.done = true
			return m, tea.Quit
		case "ctrl+s":
			val := m.textarea.Value()
			if val == "" {
				// Don't submit empty input
				return m, nil
			}
			m.result = val
			m.done = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m pubkeyModel) View() tea.View {
	s := "\n"
	s += tuiTitleStyle.Render("Paste your armored GPG public key") + "\n\n"
	s += m.textarea.View() + "\n\n"
	s += tuiHelpStyle.Render("ctrl+s submit • ctrl+c cancel") + "\n"
	return tea.NewView(s)
}

// runPubkeyTUI launches the interactive TUI for pasting a public key.
// Returns the pasted text or an error if cancelled.
func runPubkeyTUI() ([]byte, error) {
	m := newPubkeyModel()
	p := tea.NewProgram(m)

	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("TUI error: %w", err)
	}

	final, ok := result.(pubkeyModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type from TUI")
	}
	if final.err != nil {
		return nil, final.err
	}
	if final.result == "" {
		return nil, fmt.Errorf("no key provided")
	}

	return []byte(final.result), nil
}
