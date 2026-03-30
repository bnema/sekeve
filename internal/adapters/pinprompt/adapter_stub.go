//go:build !linux

package pinprompt

import (
	"context"

	"github.com/bnema/sekeve/internal/port"
)

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

func (a *PINPromptStubAdapter) PromptForPIN(_ context.Context, errorMode bool, message string) (string, error) {
	return promptTTY(errorMode, message)
}
