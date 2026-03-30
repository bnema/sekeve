package client

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

func NewEditCmd() *cobra.Command {
	var id, domain, email string

	cmd := &cobra.Command{
		Use:   "edit [query]",
		Short: "Edit an existing entry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

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

			opts := resolveOpts{ID: id, Domain: domain, Email: email}
			if len(args) > 0 {
				opts.Query = args[0]
			}

			var env *entity.Envelope
			if opts.ID != "" {
				env, err = clientApp.Vault.GetEntry(ctx, opts.ID)
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}
			} else {
				if opts.Query == "" && opts.Domain == "" && opts.Email == "" {
					return fmt.Errorf("provide a search query, --domain, --email, or --id")
				}

				all, err := clientApp.Vault.ListEntries(ctx, entity.EntryTypeUnspecified)
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}

				env, err = resolveEntry(all, opts)
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}

				env, err = clientApp.Vault.GetEntry(ctx, env.ID)
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}
			}

			if env == nil {
				return fmt.Errorf("entry not found")
			}
			targetName := env.Name

			if env.Meta == nil {
				env.Meta = map[string]string{}
			}

			var updateErr error
			decryptErr := clientApp.Vault.DecryptAndUse(ctx, env.Payload, func(plaintext []byte) {
				switch env.Type {
				case entity.EntryTypeLogin:
					var login entity.Login
					if err := json.Unmarshal(plaintext, &login); err != nil {
						updateErr = err
						return
					}
					login.Site = prompt("Site", login.Site)
					login.Username = prompt("Username", login.Username)
					login.Password = prompt("Password", login.Password)
					login.Notes = prompt("Notes", login.Notes)

					newPayload, err := json.Marshal(login)
					if err != nil {
						updateErr = err
						return
					}
					env.Payload = newPayload
					env.Meta = map[string]string{"username": login.Username, "site": login.Site}
					env.Name = entity.DeriveLoginName(login.Site, login.Username)

				case entity.EntryTypeSecret:
					var secret entity.Secret
					if err := json.Unmarshal(plaintext, &secret); err != nil {
						updateErr = err
						return
					}
					secret.Value = prompt("Value", secret.Value)

					newPayload, err := json.Marshal(secret)
					if err != nil {
						updateErr = err
						return
					}
					env.Payload = newPayload

				case entity.EntryTypeNote:
					var note entity.Note
					if err := json.Unmarshal(plaintext, &note); err != nil {
						updateErr = err
						return
					}
					content, err := editInEditor(note.Content)
					if err != nil {
						updateErr = err
						return
					}
					note.Content = content

					newPayload, err := json.Marshal(note)
					if err != nil {
						updateErr = err
						return
					}
					env.Payload = newPayload
				}
			})

			if decryptErr != nil {
				_ = styles.RenderError(os.Stderr, decryptErr)
				return decryptErr
			}
			if updateErr != nil {
				_ = styles.RenderError(os.Stderr, updateErr)
				return updateErr
			}

			if err := clientApp.Vault.UpdateEntry(ctx, env); err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, env)
			}
			if env.Name != "" {
				targetName = env.Name
			}
			return styles.RenderSuccess(os.Stdout, fmt.Sprintf("Entry %q updated", targetName))
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Edit entry by exact ID")
	cmd.Flags().StringVar(&domain, "domain", "", "Filter by domain/site")
	cmd.Flags().StringVar(&email, "email", "", "Filter by username/email")
	return cmd
}

func editInEditor(content string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmpFile, err := os.CreateTemp("", "sekeve-edit-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Recreate with restricted permissions to avoid TOCTOU race
	tmpFile, err = os.OpenFile(tmpName, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		if removeErr := os.Remove(tmpName); removeErr != nil {
			return "", fmt.Errorf("failed to set temp file permissions: %w (also failed to remove temp file: %v)", err, removeErr)
		}
		return "", fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			_ = os.Remove(tmpName)
			return "", fmt.Errorf("failed to write temp file: %w (also failed to close: %v)", err, closeErr)
		}
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("failed to close temp file after write: %w", err)
	}

	cmd := exec.Command(editor, tmpName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if removeErr := os.Remove(tmpName); removeErr != nil {
			return "", fmt.Errorf("editor failed: %w (also failed to remove temp file: %v)", err, removeErr)
		}
		return "", fmt.Errorf("editor failed: %w", err)
	}

	data, err := os.ReadFile(tmpName)
	if err != nil {
		if removeErr := os.Remove(tmpName); removeErr != nil {
			return "", fmt.Errorf("failed to read temp file: %w (also failed to remove temp file: %v)", err, removeErr)
		}
		return "", fmt.Errorf("failed to read temp file: %w", err)
	}

	// Securely overwrite with zeros before removing.
	// Use file size (editor may have expanded it) rather than data length.
	size := len(data)
	if fi, err := os.Stat(tmpName); err == nil && fi.Size() > int64(size) {
		size = int(fi.Size())
	}
	zeros := make([]byte, size)
	if err := os.WriteFile(tmpName, zeros, 0600); err != nil {
		if removeErr := os.Remove(tmpName); removeErr != nil {
			return string(data), fmt.Errorf("failed to securely overwrite temp file: %w (also failed to remove temp file: %v)", err, removeErr)
		}
		return string(data), fmt.Errorf("failed to securely overwrite temp file: %w", err)
	}
	if err := os.Remove(tmpName); err != nil {
		return string(data), fmt.Errorf("failed to remove temp file: %w", err)
	}

	return string(data), nil
}
