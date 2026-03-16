package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

// importWorkers is the number of concurrent goroutines used to encrypt and
// sync entries during import. 10 balances throughput against gRPC server load.
const importWorkers = 10

// VaultImporter abstracts vault operations needed by the import command.
type VaultImporter interface {
	AddEntry(ctx context.Context, envelope *entity.Envelope) error
	ListEntries(ctx context.Context, entryType entity.EntryType) ([]*entity.Envelope, error)
}

// importResult holds the counters produced by processImport.
type importResult struct {
	Imported    int64
	Duplicates  int
	Failed      int64
	Unsupported int
	Invalid     int
}

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
	return &cobra.Command{
		Use:   "bitwarden <path-to-json>",
		Short: "Import entries from a Bitwarden JSON export",
		Long:  "Import logins and secure notes from an unencrypted Bitwarden JSON export file.",
		Args:  cobra.ExactArgs(1),
		RunE:  runImportBitwarden,
	}
}

func runImportBitwarden(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	filePath := args[0]

	// Read and parse file.
	data, err := os.ReadFile(filePath)
	if err != nil {
		_ = styles.RenderError(os.Stderr, fmt.Errorf("cannot read file: %w", err))
		return err
	}

	var export BitwardenExport
	if err := json.Unmarshal(data, &export); err != nil {
		_ = styles.RenderError(os.Stderr, fmt.Errorf("invalid JSON: %w", err))
		return err
	}

	// Validate: reject encrypted exports.
	if export.Encrypted {
		err := fmt.Errorf("this is an encrypted export; re-export from Bitwarden without encryption")
		_ = styles.RenderError(os.Stderr, err)
		return err
	}

	// Validate: items must be present (nil means the field was absent or null).
	if export.Items == nil {
		err := fmt.Errorf("invalid Bitwarden export format")
		_ = styles.RenderError(os.Stderr, err)
		return err
	}

	// Connect and authenticate.
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

	// Delegate to testable orchestration.
	result, err := processImport(ctx, clientApp.Vault, export, os.Stderr)
	if err != nil {
		_ = styles.RenderError(os.Stderr, err)
		return err
	}

	summary := fmt.Sprintf("Import complete: %d imported, %d skipped (duplicate), %d failed, %d skipped (unsupported type), %d skipped (invalid)",
		result.Imported, result.Duplicates, result.Failed, result.Unsupported, result.Invalid)
	return styles.RenderSuccess(os.Stderr, summary)
}

// processImport handles duplicate detection, item classification, mapping, and
// concurrent import. It writes progress and warnings to w. Returns counters.
func processImport(ctx context.Context, vault VaultImporter, export BitwardenExport, w io.Writer) (*importResult, error) {
	// Preflight: fetch existing entries for duplicate detection.
	existing, err := vault.ListEntries(ctx, entity.EntryTypeUnspecified)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing entries: %w", err)
	}
	seen := make(map[string]bool, len(existing))
	for _, e := range existing {
		seen[deduplicationKey(e)] = true
	}

	// Classify items and build envelopes.
	var (
		toImport []*entity.Envelope
		result   importResult
	)

	for _, item := range export.Items {
		// Validate name.
		if strings.TrimSpace(item.Name) == "" {
			_, _ = fmt.Fprintf(w, "warning: skipped item with empty name (type %d)\n", item.Type)
			result.Invalid++
			continue
		}

		// Check supported type.
		switch item.Type {
		case bwTypeLogin, bwTypeSecureNote:
			// supported
		default:
			_, _ = fmt.Fprintf(w, "warning: skipped unsupported item %q (type %d)\n", item.Name, item.Type)
			result.Unsupported++
			continue
		}

		// Map to envelope first (to get the final disambiguated name).
		var env *entity.Envelope
		var mapErr error
		switch item.Type {
		case bwTypeLogin:
			env, mapErr = MapLoginToEnvelope(item)
		case bwTypeSecureNote:
			env, mapErr = MapNoteToEnvelope(item)
		}
		if mapErr != nil {
			_, _ = fmt.Fprintf(w, "warning: skipped %q: mapping error: %v\n", item.Name, mapErr)
			result.Invalid++
			continue
		}

		key := deduplicationKey(env)
		if seen[key] {
			_, _ = fmt.Fprintf(w, "warning: skipped duplicate %q\n", env.Name)
			result.Duplicates++
			continue
		}
		seen[key] = true

		toImport = append(toImport, env)
	}

	if len(toImport) == 0 {
		_, _ = fmt.Fprintln(w, "nothing to import")
		return &result, nil
	}

	// Import with worker pool.
	total := len(toImport)
	var imported atomic.Int64
	var failed atomic.Int64
	var mu sync.Mutex

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
						mu.Lock()
						_, _ = fmt.Fprintf(w, "error: failed to import %q: %v\n", env.Name, addErr)
						mu.Unlock()
						failed.Add(1)
					} else {
						n := imported.Add(1)
						mu.Lock()
						_, _ = fmt.Fprintf(w, "Importing: %d/%d\n", n, total)
						mu.Unlock()
					}
				}
			}
		}()
	}
	wg.Wait()

	result.Imported = imported.Load()
	result.Failed = failed.Load()
	return &result, nil
}

func deduplicationKey(e *entity.Envelope) string {
	if e.Type == entity.EntryTypeLogin && e.Meta != nil {
		return fmt.Sprintf("login:%s:%s", e.Meta["site"], e.Meta["username"])
	}
	return fmt.Sprintf("%d:%s", e.Type, e.Name)
}
