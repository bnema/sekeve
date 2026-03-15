package client

import (
	"context"
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	adapterconfig "github.com/bnema/sekeve/internal/adapters/config"
	adaptergrpc "github.com/bnema/sekeve/internal/adapters/grpc"
	"github.com/bnema/zerowrap"
	"github.com/spf13/cobra"
)

func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up the sekeve client",
		Long: `Initialize the sekeve client configuration.

Prompts for the server address and GPG key ID, then writes
the configuration and tests the server connection.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := zerowrap.FromCtx(ctx)

			cfg := cliconfig.ConfigFromCmd(cmd)

			viperCfg, ok := cfg.(*adapterconfig.ViperConfig)
			if !ok {
				return fmt.Errorf("unexpected config type")
			}

			if !viperCfg.IsUnconfigured() {
				_ = styles.RenderSuccess(os.Stdout, "Client already configured")
				return nil
			}

			result, err := cliconfig.RunOnboarding()
			if err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			if err := viperCfg.WriteConfig(result.ServerAddr, result.GPGKeyID); err != nil {
				log.Error().Err(err).Msg("failed to save config")
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			_ = styles.RenderSuccess(os.Stdout, fmt.Sprintf("Config written to %s", viperCfg.ConfigPath()))

			// Test connection with spinner.
			if err := runHealthCheck(ctx, result.ServerAddr); err != nil {
				_ = styles.RenderError(os.Stderr, fmt.Errorf("connection test failed: %w", err))
				fmt.Fprintln(os.Stderr, styles.MutedStyle.Render("  Check that the server is running and the address is correct."))
				// Don't return error — config is already written, connection can be retried.
				return nil
			}

			_ = styles.RenderSuccess(os.Stdout, "Server connection verified")
			return nil
		},
	}

	return cmd
}

// healthCheckMsg is sent when the health check goroutine completes.
type healthCheckMsg struct {
	err error
}

type healthModel struct {
	spinner spinner.Model
	message string
	done    bool
	err     error
}

func newHealthModel() healthModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.Line),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("179"))),
	)
	return healthModel{
		spinner: s,
		message: "Testing connection...",
	}
}

func (m healthModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m healthModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case healthCheckMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			m.err = fmt.Errorf("cancelled")
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m healthModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(fmt.Sprintf("  %s %s", m.spinner.View(), m.message))
}

func runHealthCheck(ctx context.Context, serverAddr string) error {
	m := newHealthModel()
	p := tea.NewProgram(m)

	// Run health check in background.
	go func() {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		client, err := adaptergrpc.NewClient(checkCtx, serverAddr)
		if err != nil {
			p.Send(healthCheckMsg{err: err})
			return
		}
		defer client.Close(ctx)

		err = client.CheckHealth(checkCtx)
		p.Send(healthCheckMsg{err: err})
	}()

	result, err := p.Run()
	if err != nil {
		return err
	}

	final, ok := result.(healthModel)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}
	return final.err
}
