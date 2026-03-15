package cliconfig

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	onboardTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("67"))

	onboardLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("109")).
				Width(14)

	onboardHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("243"))

	onboardAccentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("179"))
)

// OnboardingResult holds the values collected from the onboarding TUI.
type OnboardingResult struct {
	ServerAddr string
	GPGKeyID   string
}

type onboardingModel struct {
	inputs  []textinput.Model
	labels  []string
	focused int
	result  *OnboardingResult
	err     error
}

func newOnboardingModel() onboardingModel {
	serverInput := textinput.New()
	serverInput.Placeholder = "localhost:50051"
	serverInput.SetValue("localhost:50051")
	serverInput.SetWidth(40)
	serverInput.Focus()

	gpgInput := textinput.New()
	gpgInput.Placeholder = "email@example.com or key ID"
	gpgInput.SetWidth(40)

	return onboardingModel{
		inputs:  []textinput.Model{serverInput, gpgInput},
		labels:  []string{"Server addr", "GPG key ID"},
		focused: 0,
	}
}

func (m onboardingModel) Init() tea.Cmd {
	return m.inputs[0].Focus()
}

func (m onboardingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.err = fmt.Errorf("onboarding cancelled")
			return m, tea.Quit

		case "tab", "enter":
			// If on last field, submit.
			if m.focused == len(m.inputs)-1 {
				gpgVal := m.inputs[1].Value()
				if gpgVal == "" {
					// Don't submit without GPG key
					return m, nil
				}
				serverVal := m.inputs[0].Value()
				if serverVal == "" {
					serverVal = "localhost:50051"
				}
				m.result = &OnboardingResult{
					ServerAddr: serverVal,
					GPGKeyID:   gpgVal,
				}
				return m, tea.Quit
			}

			// Move to next field.
			m.inputs[m.focused].Blur()
			m.focused++
			cmd := m.inputs[m.focused].Focus()
			return m, cmd

		case "shift+tab":
			if m.focused > 0 {
				m.inputs[m.focused].Blur()
				m.focused--
				cmd := m.inputs[m.focused].Focus()
				return m, cmd
			}
		}
	}

	// Update the focused input.
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m onboardingModel) View() tea.View {
	s := "\n"
	s += onboardTitleStyle.Render("Sekeve - first-time setup") + "\n\n"

	for i, input := range m.inputs {
		label := onboardLabelStyle.Render(m.labels[i])
		cursor := "  "
		if i == m.focused {
			cursor = onboardAccentStyle.Render("> ")
		}
		s += cursor + label + input.View() + "\n"
	}

	s += "\n" + onboardHelpStyle.Render("tab next - enter submit - esc cancel") + "\n"
	return tea.NewView(s)
}

// RunOnboarding launches the interactive TUI for first-time client setup.
// Returns the collected config values or an error if cancelled.
func RunOnboarding() (*OnboardingResult, error) {
	m := newOnboardingModel()
	p := tea.NewProgram(m)

	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("onboarding TUI error: %w", err)
	}

	final, ok := result.(onboardingModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type from onboarding TUI")
	}
	if final.err != nil {
		return nil, final.err
	}
	if final.result == nil {
		return nil, fmt.Errorf("onboarding incomplete")
	}

	return final.result, nil
}
