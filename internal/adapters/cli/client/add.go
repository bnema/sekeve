package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
		Use:   "login",
		Short: "Add a login entry",
		Long: `Add a login entry. Name is auto-derived from site and username.

Examples:
  sekeve add login --site https://gmail.com --username brice@gmail.com --password hunter2
  sekeve add login  # interactive prompts`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if site == "" {
				site = prompt("Site", "")
			}
			if username == "" {
				username = prompt("Username", "")
			}
			if password == "" {
				fmt.Print("Password: ")
				passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Println()
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}
				password = string(passBytes)
			}
			if notes == "" && isTerminal() {
				notes = prompt("Notes (optional)", "")
			}

			name := deriveLoginName(site, username)

			login := entity.Login{
				Site:     site,
				Username: username,
				Password: password,
				Notes:    notes,
			}
			payload, err := json.Marshal(login)
			if err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

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

			env := &entity.Envelope{
				Name:    name,
				Type:    entity.EntryTypeLogin,
				Meta:    map[string]string{"username": username, "site": site},
				Payload: payload,
			}

			if err := clientApp.Vault.AddEntry(ctx, env); err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, env)
			}
			return styles.RenderSuccess(os.Stdout, fmt.Sprintf("Login %q added", name))
		},
	}

	cmd.Flags().StringVar(&site, "site", "", "Site URL")
	cmd.Flags().StringVar(&username, "username", "", "Username")
	cmd.Flags().StringVar(&password, "password", "", "Password")
	cmd.Flags().StringVar(&notes, "notes", "", "Notes")
	return cmd
}

func deriveLoginName(site, username string) string {
	domain := extractDomain(site)
	if domain == "" {
		domain = site
	}
	if username != "" {
		return fmt.Sprintf("%s (%s)", domain, username)
	}
	return domain
}

func extractDomain(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := u.Host
	if host == "" {
		u2, err := url.Parse("https://" + raw)
		if err != nil {
			return raw
		}
		host = u2.Host
	}
	if host == "" {
		return raw
	}
	return host
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
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

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

			env := &entity.Envelope{
				Name:    name,
				Type:    entity.EntryTypeSecret,
				Payload: payload,
			}

			if err := clientApp.Vault.AddEntry(ctx, env); err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, env)
			}
			return styles.RenderSuccess(os.Stdout, fmt.Sprintf("Secret %q added", name))
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
					_ = styles.RenderError(os.Stderr, err)
					return err
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
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

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

			env := &entity.Envelope{
				Name:    name,
				Type:    entity.EntryTypeNote,
				Payload: payload,
			}

			if err := clientApp.Vault.AddEntry(ctx, env); err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, env)
			}
			return styles.RenderSuccess(os.Stdout, fmt.Sprintf("Note %q added", name))
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Read content from file")
	return cmd
}
