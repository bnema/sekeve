package server

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// readPublicKeyInput resolves the public key from the best available source:
//  1. --pubkey-file flag (highest priority)
//  2. SEKEVE_PUBKEY env var (container-friendly, key content directly)
//  3. piped stdin (when stdin is not a terminal)
//  4. interactive TUI paste (when stdin is a terminal)
//
// Returns the raw bytes and a source description for logging.
func readPublicKeyInput(cmd *cobra.Command, pubKeyFile string) ([]byte, string, error) {
	// 1. File flag takes priority.
	if pubKeyFile != "" {
		data, err := os.ReadFile(pubKeyFile)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read public key file: %w", err)
		}
		return data, fmt.Sprintf("file: %s", pubKeyFile), nil
	}

	// 2. Environment variable (container use).
	if envKey := os.Getenv("SEKEVE_PUBKEY"); envKey != "" {
		return []byte(envKey), "env: SEKEVE_PUBKEY", nil
	}

	// 3. Check if input is a terminal using the actual input stream.
	in := cmd.InOrStdin()
	isTTY := false
	if f, ok := in.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	// 4. Piped stdin (non-TTY).
	if !isTTY {
		data, err := io.ReadAll(in)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read public key from stdin: %w", err)
		}
		if len(data) == 0 {
			return nil, "", fmt.Errorf("no public key provided; set SEKEVE_PUBKEY or use --pubkey-file")
		}
		return data, "stdin", nil
	}

	// 5. Interactive TUI paste.
	data, err := runPubkeyTUI()
	if err != nil {
		return nil, "", err
	}
	return data, "interactive paste", nil
}
