package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/zerowrap"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// importWorkers is the number of concurrent goroutines used to encrypt and
// sync entries during import. 10 balances throughput against gRPC server load.
const importWorkers = 10

// VaultImporter abstracts vault operations needed by the import command.
type VaultImporter interface {
	AddEntry(ctx context.Context, envelope *entity.Envelope) error
	ListEntries(ctx context.Context, entryType entity.EntryType) ([]*entity.Envelope, error)
	DeleteEntry(ctx context.Context, id string) error
}

// importResult holds the counters produced by processImport.
type importResult struct {
	Imported    int64
	Duplicates  int
	Failed      int64
	Unsupported int
	Invalid     int
	FirstErr    error
}

// ProgressFunc is called after each entry is processed during import.
// done is the number of entries processed so far, total is the total count.
type ProgressFunc func(done, total int)

// NewImportCmd returns the "import" parent command with provider subcommands.
func NewImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import entries from external sources",
	}
	cmd.AddCommand(newImportBitwardenCmd())
	return cmd
}

func newImportBitwardenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bitwarden",
		Short: "Import entries from Bitwarden",
		Long: `Import logins and secure notes from Bitwarden.

By default, calls the Bitwarden CLI directly: prompts for your master password,
exports the vault to memory (never to disk), imports into sekeve, and re-locks
the Bitwarden vault. The JSON never touches the filesystem.

Use --file to import from an existing JSON export instead.`,
		Args: cobra.NoArgs,
		RunE: runImportBitwarden,
	}
	cmd.Flags().String("file", "", "Import from an existing Bitwarden JSON export file instead of calling bw CLI")
	cmd.Flags().Bool("force", false, "Delete all existing entries before importing")
	return cmd
}

func runImportBitwarden(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	filePath, _ := cmd.Flags().GetString("file")
	force, _ := cmd.Flags().GetBool("force")

	var data []byte
	var err error

	if filePath != "" {
		data, err = os.ReadFile(filePath)
		if err != nil {
			_ = styles.RenderError(os.Stderr, fmt.Errorf("cannot read file: %w", err))
			return err
		}
	} else {
		data, err = exportFromBitwardenCLI()
		if err != nil {
			_ = styles.RenderError(os.Stderr, err)
			return err
		}
	}
	defer clear(data)

	var export BitwardenExport
	if err := json.Unmarshal(data, &export); err != nil {
		_ = styles.RenderError(os.Stderr, fmt.Errorf("invalid Bitwarden JSON"))
		return err
	}

	if export.Encrypted {
		err := fmt.Errorf("encrypted export not supported; re-export without encryption")
		_ = styles.RenderError(os.Stderr, err)
		return err
	}

	if export.Items == nil {
		err := fmt.Errorf("invalid Bitwarden export format")
		_ = styles.RenderError(os.Stderr, err)
		return err
	}

	// Connect and authenticate to sekeve.
	cfg := cliconfig.ConfigFromCmd(cmd)
	clientApp, err := cliconfig.ConnectAndAuth(ctx, cfg)
	if err != nil {
		_ = styles.RenderError(os.Stderr, err)
		return err
	}
	defer func() {
		if cerr := clientApp.Close(ctx); cerr != nil {
			_ = styles.RenderError(os.Stderr, cerr)
		}
	}()

	// Suppress domain-layer logs during batch operations (noisy WrapErr).
	silentCtx := zerowrap.WithCtx(ctx, zerowrap.New(zerowrap.Config{Level: "disabled", Output: io.Discard}))

	// --force: wipe all existing entries before importing.
	if force {
		if err := wipeEntries(ctx, clientApp.Vault); err != nil {
			_ = styles.RenderError(os.Stderr, err)
			return err
		}
	}

	// Run import with spinner if TTY is available.
	var result *importResult
	if term.IsTerminal(int(os.Stderr.Fd())) {
		result, err = runImportWithSpinner(silentCtx, clientApp.Vault, export, force)
	} else {
		result, err = processImport(silentCtx, clientApp.Vault, export, force, nil)
	}
	if err != nil {
		_ = styles.RenderError(os.Stderr, err)
		return err
	}

	summary := fmt.Sprintf("Import complete: %d imported, %d skipped (duplicate), %d failed, %d skipped (unsupported type), %d skipped (invalid)",
		result.Imported, result.Duplicates, result.Failed, result.Unsupported, result.Invalid)
	if result.Failed > 0 && result.FirstErr != nil {
		summary += fmt.Sprintf("\n  first error: %v", result.FirstErr)
	}
	return styles.RenderSuccess(os.Stderr, summary)
}

// runImportWithSpinner wraps processImport with a bubbletea spinner.
func runImportWithSpinner(ctx context.Context, vault VaultImporter, export BitwardenExport, skipDedup bool) (*importResult, error) {
	// Count importable items for the spinner total (quick pre-scan).
	total := 0
	for _, item := range export.Items {
		if isImportableItem(item) {
			total++
		}
	}

	var progress atomic.Int64
	m := newImportModel(total, &progress)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))

	var result *importResult
	var importErr error

	go func() {
		result, importErr = processImport(ctx, vault, export, skipDedup, func(done, _ int) {
			progress.Store(int64(done))
		})
		p.Send(importDoneMsg{})
	}()

	if _, err := p.Run(); err != nil {
		return nil, err
	}
	return result, importErr
}

func wipeEntries(ctx context.Context, vault VaultImporter) error {
	entries, err := vault.ListEntries(ctx, entity.EntryTypeUnspecified)
	if err != nil {
		return fmt.Errorf("failed to list entries for wipe: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	var deleted, skipped int
	for _, e := range entries {
		if err := vault.DeleteEntry(ctx, e.ID); err != nil {
			if isNotFound(err) {
				skipped++
				continue
			}
			return fmt.Errorf("failed to delete entry during wipe: %w", err)
		}
		deleted++
	}
	_, _ = fmt.Fprintf(os.Stderr, "Wiped %d entries (%d not found, skipped)\n", deleted, skipped)
	return nil
}

// exportFromBitwardenCLI calls the bw CLI to export the vault as JSON.
// The master password is read from the TTY (no echo) and never written to disk.
// The vault is re-locked after export.
func exportFromBitwardenCLI() ([]byte, error) {
	if _, err := exec.LookPath("bw"); err != nil {
		return nil, fmt.Errorf("bitwarden CLI (bw) not found in PATH; install it or use --file")
	}

	statusOut, err := exec.Command("bw", "status").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to check Bitwarden status: %w", err)
	}
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(statusOut, &status); err != nil {
		return nil, fmt.Errorf("failed to parse Bitwarden status")
	}
	if status.Status == "unauthenticated" {
		return nil, fmt.Errorf("not logged into Bitwarden; run 'bw login' first")
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("stdin must be a terminal to read master password; use --file instead")
	}
	fmt.Fprint(os.Stderr, "Bitwarden master password: ")
	passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}
	defer clear(passBytes)

	// Unlock vault using --passwordenv to avoid leaking password in process args.
	unlockCmd := exec.Command("bw", "unlock", "--raw", "--passwordenv", "BW_MASTER_PASS")
	unlockCmd.Env = append(os.Environ(), "BW_MASTER_PASS="+string(passBytes))
	sessionOut, err := unlockCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to unlock Bitwarden vault (wrong password?)")
	}
	trimmed := bytes.TrimSpace(sessionOut)
	sessionBytes := make([]byte, len(trimmed))
	copy(sessionBytes, trimmed)
	clear(sessionOut)
	defer clear(sessionBytes)

	// Export vault as JSON to stdout (never to disk).
	exportCmd := exec.Command("bw", "export", "--raw", "--format", "json")
	exportCmd.Env = append(os.Environ(), "BW_SESSION="+string(sessionBytes))
	var exportBuf bytes.Buffer
	exportCmd.Stdout = &exportBuf
	var stderrBuf bytes.Buffer
	exportCmd.Stderr = &stderrBuf
	if err := exportCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to export Bitwarden vault")
	}

	// Re-lock the Bitwarden vault.
	lockCmd := exec.Command("bw", "lock")
	lockCmd.Env = append(os.Environ(), "BW_SESSION="+string(sessionBytes))
	_ = lockCmd.Run()

	return exportBuf.Bytes(), nil
}

// processImport handles item classification, optional duplicate detection, mapping,
// and concurrent import. When skipDedup is true, server-side dedup is skipped (used
// with --force after a wipe). onProgress is called after each entry is processed (may be nil).
func processImport(ctx context.Context, vault VaultImporter, export BitwardenExport, skipDedup bool, onProgress ProgressFunc) (*importResult, error) {
	seen := make(map[string]bool)
	if !skipDedup {
		existing, err := vault.ListEntries(ctx, entity.EntryTypeUnspecified)
		if err != nil {
			return nil, fmt.Errorf("failed to list existing entries: %w", err)
		}
		for _, e := range existing {
			seen[deduplicationKey(e)] = true
		}
	}

	var (
		toImport []*entity.Envelope
		result   importResult
	)

	for _, item := range export.Items {
		if !isImportableItem(item) {
			if strings.TrimSpace(item.Name) == "" {
				result.Invalid++
			} else {
				result.Unsupported++
			}
			continue
		}

		var env *entity.Envelope
		var mapErr error
		switch item.Type {
		case bwTypeLogin:
			env, mapErr = MapLoginToEnvelope(item)
		case bwTypeSecureNote:
			env, mapErr = MapNoteToEnvelope(item)
		}
		if mapErr != nil {
			result.Invalid++
			continue
		}

		key := deduplicationKey(env)
		if seen[key] {
			result.Duplicates++
			continue
		}
		seen[key] = true

		toImport = append(toImport, env)
	}

	if len(toImport) == 0 {
		return &result, nil
	}

	total := len(toImport)
	var imported atomic.Int64
	var failed atomic.Int64
	var done atomic.Int64
	var firstErr atomic.Value

	ch := make(chan *entity.Envelope, len(toImport))
	for _, env := range toImport {
		ch <- env
	}
	close(ch)

	workerCount := min(importWorkers, total)
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case env, ok := <-ch:
					if !ok {
						return
					}
					if addErr := vault.AddEntry(ctx, env); addErr != nil {
						failed.Add(1)
						firstErr.CompareAndSwap(nil, addErr)
					} else {
						imported.Add(1)
					}
					n := int(done.Add(1))
					if onProgress != nil {
						onProgress(n, total)
					}
				}
			}
		}()
	}
	wg.Wait()

	result.Imported = imported.Load()
	result.Failed = failed.Load()
	if v := firstErr.Load(); v != nil {
		result.FirstErr = v.(error)
	}
	return &result, nil
}

// Bubbletea model for import spinner.

type importDoneMsg struct{}

type importModel struct {
	spinner  spinner.Model
	total    int
	progress *atomic.Int64
	done     bool
}

func newImportModel(total int, progress *atomic.Int64) importModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.Line),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("179"))),
	)
	return importModel{
		spinner:  s,
		total:    total,
		progress: progress,
	}
}

func (m importModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m importModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case importDoneMsg:
		m.done = true
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m importModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	cur := m.progress.Load()
	return tea.NewView(fmt.Sprintf("  %s Importing %d/%d", m.spinner.View(), cur, m.total))
}

func isNotFound(err error) bool {
	s, ok := status.FromError(err)
	return ok && s.Code() == codes.NotFound
}

func deduplicationKey(e *entity.Envelope) string {
	if e.Type == entity.EntryTypeLogin && e.Meta != nil {
		return fmt.Sprintf("login:%s:%s", e.Meta["site"], e.Meta["username"])
	}
	return fmt.Sprintf("%d:%s", e.Type, e.Name)
}
