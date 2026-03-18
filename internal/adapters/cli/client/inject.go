package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/app"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

// NewInjectCmd creates the "inject" command.
func NewInjectCmd() *cobra.Command {
	var envFiles []string

	cmd := &cobra.Command{
		Use:   "inject -- <command> [args...]",
		Short: "Run a command with secrets injected from .env files",
		Long: `Parse .env files, resolve sekeve: references by decrypting from the vault,
and inject all variables into the subprocess environment.

Values prefixed with "sekeve:" are decrypted from the vault. All other values
are passed through unchanged. The current environment is inherited.

Examples:
  sekeve inject -- docker compose up
  sekeve inject -- node server.js
  sekeve inject --env .env.production -- terraform apply

.env format:
  DATABASE_URL=sekeve:prod-db-url          # decrypt secret named "prod-db-url"
  GITHUB_TOKEN=sekeve:github               # decrypt login matching "github" → password
  GITHUB_USER=sekeve:github#username       # same entry → username field
  PLAIN_VAR=just-a-value                   # passed through unchanged`,
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Default to .env if no --env flags given
			if len(envFiles) == 0 {
				envFiles = []string{".env"}
			}

			// Parse all env files
			var allLines []envLine
			for _, path := range envFiles {
				f, err := os.Open(path)
				if err != nil {
					if os.IsNotExist(err) && path == ".env" {
						continue // default .env is optional
					}
					_ = styles.RenderError(os.Stderr, fmt.Errorf("open %s: %w", path, err))
					return err
				}
				lines, err := parseDotenv(f)
				f.Close()
				if err != nil {
					_ = styles.RenderError(os.Stderr, fmt.Errorf("parse %s: %w", path, err))
					return err
				}
				allLines = append(allLines, lines...)
			}

			// Check if any lines need vault resolution
			var refs []int // indices of lines with sekeve: refs
			for i, line := range allLines {
				if parseSekeveRef(line.Value) != nil {
					refs = append(refs, i)
				}
			}

			// Only connect to vault if there are references to resolve
			if len(refs) > 0 {
				cfg := cliconfig.ConfigFromCmd(cmd)
				clientApp, err := cliconfig.ConnectAndAuth(ctx, cfg)
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}
				defer func() {
					if err := clientApp.Close(ctx); err != nil {
						_ = styles.RenderError(os.Stderr, err)
					}
				}()

				if err := resolveRefs(ctx, clientApp, allLines, refs); err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}
			}

			// Build env: inherit current env + overlay parsed values
			env := os.Environ()
			for _, line := range allLines {
				env = append(env, line.Key+"="+line.Value)
			}

			// Exec subprocess
			binary, err := exec.LookPath(args[0])
			if err != nil {
				_ = styles.RenderError(os.Stderr, fmt.Errorf("command not found: %s", args[0]))
				return err
			}

			child := exec.CommandContext(ctx, binary, args[1:]...)
			child.Env = env
			child.Stdin = os.Stdin
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			return child.Run()
		},
	}

	cmd.Flags().StringArrayVarP(&envFiles, "env", "e", nil, "env file(s) to load (default: .env)")
	return cmd
}

// resolveRefs decrypts sekeve: references in-place, replacing the raw
// reference strings in allLines with their decrypted values.
// Uses filterEntries directly (not resolveEntry) to avoid launching
// an interactive picker — ambiguous matches always error.
func resolveRefs(ctx context.Context, clientApp *app.ClientApp, allLines []envLine, refIndices []int) error {
	// Fetch all entries once for smart search
	all, err := clientApp.Vault.ListEntries(ctx, entity.EntryTypeUnspecified)
	if err != nil {
		return fmt.Errorf("list entries: %w", err)
	}

	for _, i := range refIndices {
		ref := parseSekeveRef(allLines[i].Value)

		// Use filterEntries directly to avoid interactive picker
		matched := filterEntries(all, resolveOpts{Query: ref.Query})
		if len(matched) == 0 {
			return fmt.Errorf("resolve %s=%s: no entries found matching %q", allLines[i].Key, allLines[i].Value, ref.Query)
		}
		if len(matched) > 1 {
			return fmt.Errorf("resolve %s=%s: %w", allLines[i].Key, allLines[i].Value, &AmbiguousMatchError{Matches: matched})
		}

		// Fetch full entry (with payload) by ID
		env, err := clientApp.Vault.GetEntry(ctx, matched[0].ID)
		if err != nil {
			return fmt.Errorf("get entry %s: %w", matched[0].ID, err)
		}

		// Decrypt and extract field
		var resolved string
		var extractErr error
		decryptErr := clientApp.Vault.DecryptAndUse(ctx, env.Payload, func(plaintext []byte) {
			resolved, extractErr = extractField(plaintext, env.Type.String(), ref.Field)
		})
		if decryptErr != nil {
			return fmt.Errorf("decrypt %s: %w", allLines[i].Key, decryptErr)
		}
		if extractErr != nil {
			return fmt.Errorf("extract %s: %w", allLines[i].Key, extractErr)
		}

		allLines[i].Value = resolved
	}
	return nil
}

const sekevePrefix = "sekeve:"

type sekeveRef struct {
	Query string
	Field string // empty = primary value
}

// parseSekeveRef checks if a value is a sekeve: reference and parses it.
// Returns nil if the value is not a reference.
func parseSekeveRef(value string) *sekeveRef {
	if !strings.HasPrefix(value, sekevePrefix) {
		return nil
	}
	rest := value[len(sekevePrefix):]
	if rest == "" {
		return nil
	}

	ref := &sekeveRef{}
	if idx := strings.IndexByte(rest, '#'); idx >= 0 {
		ref.Query = rest[:idx]
		ref.Field = rest[idx+1:]
	} else {
		ref.Query = rest
	}
	return ref
}

// extractField extracts a field value from decrypted JSON payload.
// If field is empty, returns the primary value for the entry type.
func extractField(plaintext []byte, entryType string, field string) (string, error) {
	switch entryType {
	case "login":
		var login entity.Login
		if err := json.Unmarshal(plaintext, &login); err != nil {
			return "", fmt.Errorf("unmarshal login: %w", err)
		}
		if field == "" {
			return login.Password, nil
		}
		switch field {
		case "password":
			return login.Password, nil
		case "username":
			return login.Username, nil
		case "site":
			return login.Site, nil
		case "notes":
			return login.Notes, nil
		default:
			return "", fmt.Errorf("login has no field %q (valid: password, username, site, notes)", field)
		}

	case "secret":
		var secret entity.Secret
		if err := json.Unmarshal(plaintext, &secret); err != nil {
			return "", fmt.Errorf("unmarshal secret: %w", err)
		}
		if field == "" || field == "value" {
			return secret.Value, nil
		}
		if field == "name" {
			return secret.Name, nil
		}
		return "", fmt.Errorf("secret has no field %q (valid: value, name)", field)

	case "note":
		var note entity.Note
		if err := json.Unmarshal(plaintext, &note); err != nil {
			return "", fmt.Errorf("unmarshal note: %w", err)
		}
		if field == "" || field == "content" {
			return note.Content, nil
		}
		if field == "name" {
			return note.Name, nil
		}
		return "", fmt.Errorf("note has no field %q (valid: content, name)", field)

	default:
		return "", fmt.Errorf("unknown entry type %q", entryType)
	}
}
