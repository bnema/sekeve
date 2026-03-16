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
	return &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit an existing entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]

			clientApp, err := cliconfig.ConnectAndAuth(ctx, cliconfig.ServerAddr, cliconfig.GPGKeyID)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return err
			}
			defer clientApp.Close(ctx)

			env, err := clientApp.Vault.GetEntry(ctx, name)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return err
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
				styles.RenderError(os.Stderr, decryptErr)
				return decryptErr
			}
			if updateErr != nil {
				styles.RenderError(os.Stderr, updateErr)
				return updateErr
			}

			if err := clientApp.Vault.UpdateEntry(ctx, env); err != nil {
				styles.RenderError(os.Stderr, err)
				return err
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, env)
			}
			styles.RenderSuccess(os.Stdout, fmt.Sprintf("Entry %q updated", name))
			return nil
		},
	}
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

	if err := os.Chmod(tmpName, 0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	cmd := exec.Command(editor, tmpName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("editor failed: %w", err)
	}

	data, err := os.ReadFile(tmpName)
	if err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to read temp file: %w", err)
	}

	// Securely overwrite with zeros before removing.
	zeros := make([]byte, len(data))
	_ = os.WriteFile(tmpName, zeros, 0600)
	os.Remove(tmpName)

	return string(data), nil
}
