//go:build !linux

package pinprompt

import (
	"context"
	"fmt"
	"os"

	"github.com/bnema/sekeve/internal/port"
	"golang.org/x/term"
)

// Compile-time interface check.
var _ port.PINPromptPort = (*PINPromptStubAdapter)(nil)

// PINPromptStubAdapter implements port.PINPromptPort using terminal input only.
// It is used on non-Linux platforms where the GTK4 GUI is unavailable.
type PINPromptStubAdapter struct{}

// NewPINPromptAdapter creates a new stub adapter for non-Linux platforms.
func NewPINPromptAdapter() *PINPromptStubAdapter {
	return &PINPromptStubAdapter{}
}

// IsTTY always returns true for the stub adapter.
func (a *PINPromptStubAdapter) IsTTY() bool { return true }

// PromptForPIN reads a PIN from the terminal. Note: term.ReadPassword is a blocking
// syscall with no cancellation mechanism, so ctx is not used here.
func (a *PINPromptStubAdapter) PromptForPIN(_ context.Context, errorMode bool, message string) (string, error) {
	if errorMode && message == "" {
		fmt.Fprintln(os.Stderr, "Incorrect PIN, please try again.")
	} else if message != "" {
		fmt.Fprintln(os.Stderr, message)
	}

	fmt.Fprint(os.Stderr, "Unlock PIN: ")
	pin, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after masked input
	if err != nil {
		return "", fmt.Errorf("failed to read PIN: %w", err)
	}
	return string(pin), nil
}
