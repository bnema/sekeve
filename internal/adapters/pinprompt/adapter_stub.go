//go:build !linux

package pinprompt

import (
	"context"
	"os"

	"github.com/bnema/sekeve/internal/port"
	"golang.org/x/term"
)

var _ port.PINPromptPort = (*PINPromptStubAdapter)(nil)

// PINPromptStubAdapter implements port.PINPromptPort using terminal input only.
// It is used on non-Linux platforms where the GTK4 GUI is unavailable.
type PINPromptStubAdapter struct {
	isTTY bool
}

func NewPINPromptAdapter() *PINPromptStubAdapter {
	return &PINPromptStubAdapter{
		isTTY: term.IsTerminal(int(os.Stdin.Fd())),
	}
}

func (a *PINPromptStubAdapter) IsTTY() bool { return a.isTTY }

func (a *PINPromptStubAdapter) PromptForPIN(ctx context.Context, validate port.PINValidateFunc) error {
	return promptTTY(ctx, validate)
}
