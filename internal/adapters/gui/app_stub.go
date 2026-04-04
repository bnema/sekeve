// internal/adapters/gui/app_stub.go
//go:build !linux || nogtk

package gui

import (
	"context"
	"fmt"
	"os"

	"github.com/bnema/sekeve/internal/adapters/gui/pin"
	"github.com/bnema/sekeve/internal/port"
	"golang.org/x/term"
)

var _ port.GUIPort = (*GUIAdapter)(nil)

type GUIAdapter struct {
	isTTY bool
}

func NewGUIAdapter() *GUIAdapter {
	return &GUIAdapter{isTTY: term.IsTerminal(int(os.Stdin.Fd()))}
}

func (a *GUIAdapter) IsTTY() bool { return a.isTTY }

func (a *GUIAdapter) PromptForPIN(ctx context.Context, validate port.PINValidateFunc) error {
	if a.isTTY {
		return pin.PromptTTY(ctx, validate)
	}
	return port.ErrNoPINInputMethod
}

func (a *GUIAdapter) ShowOmnibox(_ context.Context, _ port.OmniboxConfig) error {
	return fmt.Errorf("omnibox requires Linux with GTK4 and Wayland")
}

func (a *GUIAdapter) SetPendingPIN(_ port.PINValidateFunc) {}
