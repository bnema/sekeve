package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

func NewAddCmd() *cobra.Command {
	add := &cobra.Command{
		Use:   "add",
		Short: "Add a new entry",
	}
	add.AddCommand(newAddLoginCmd())
	add.AddCommand(newAddSecretCmd())
	add.AddCommand(newAddNoteCmd())
	return add
}

func prompt(label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return defaultVal
	}
	return val
}

func newAddLoginCmd() *cobra.Command {
	var site, username, password, notes string

	cmd := &cobra.Command{
		Use:   "login <name>",
		Short: "Add a login entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]

			if site == "" {
				site = prompt("Site", "")
			}
			if username == "" {
				username = prompt("Username", "")
			}
			if password == "" {
				password = prompt("Password", "")
			}
			if notes == "" {
				notes = prompt("Notes (optional)", "")
			}

			login := entity.Login{
				Site:     site,
				Username: username,
				Password: password,
				Notes:    notes,
			}
			payload, err := json.Marshal(login)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}

			clientApp, err := cliconfig.ConnectAndAuth(ctx, cliconfig.ServerAddr, cliconfig.GPGKeyID)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}
			defer clientApp.Close(ctx)

			env := &entity.Envelope{
				Name:    name,
				Type:    entity.EntryTypeLogin,
				Meta:    map[string]string{"username": username, "site": site},
				Payload: payload,
			}

			if err := clientApp.Vault.AddEntry(ctx, env); err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, env)
			}
			styles.RenderSuccess(os.Stdout, fmt.Sprintf("Login %q added", name))
			return nil
		},
	}

	cmd.Flags().StringVar(&site, "site", "", "Site URL")
	cmd.Flags().StringVar(&username, "username", "", "Username")
	cmd.Flags().StringVar(&password, "password", "", "Password")
	cmd.Flags().StringVar(&notes, "notes", "", "Notes")
	return cmd
}

func newAddSecretCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "secret <name> <value>",
		Short: "Add a secret entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			value := args[1]

			secret := entity.Secret{Name: name, Value: value}
			payload, err := json.Marshal(secret)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}

			clientApp, err := cliconfig.ConnectAndAuth(ctx, cliconfig.ServerAddr, cliconfig.GPGKeyID)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}
			defer clientApp.Close(ctx)

			env := &entity.Envelope{
				Name:    name,
				Type:    entity.EntryTypeSecret,
				Payload: payload,
			}

			if err := clientApp.Vault.AddEntry(ctx, env); err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, env)
			}
			styles.RenderSuccess(os.Stdout, fmt.Sprintf("Secret %q added", name))
			return nil
		},
	}
}

func newAddNoteCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "note <name>",
		Short: "Add a note entry (reads from stdin or --file)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]

			var content string
			if filePath != "" {
				data, err := os.ReadFile(filePath)
				if err != nil {
					styles.RenderError(os.Stderr, err)
					return nil
				}
				content = string(data)
			} else {
				fmt.Println("Enter note content (Ctrl+D to finish):")
				scanner := bufio.NewScanner(os.Stdin)
				var lines []string
				for scanner.Scan() {
					lines = append(lines, scanner.Text())
				}
				content = strings.Join(lines, "\n")
			}

			note := entity.Note{Name: name, Content: content}
			payload, err := json.Marshal(note)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}

			clientApp, err := cliconfig.ConnectAndAuth(ctx, cliconfig.ServerAddr, cliconfig.GPGKeyID)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}
			defer clientApp.Close(ctx)

			env := &entity.Envelope{
				Name:    name,
				Type:    entity.EntryTypeNote,
				Payload: payload,
			}

			if err := clientApp.Vault.AddEntry(ctx, env); err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, env)
			}
			styles.RenderSuccess(os.Stdout, fmt.Sprintf("Note %q added", name))
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Read content from file")
	return cmd
}
