// internal/port/gui.go
package port

import (
	"context"

	"github.com/bnema/sekeve/internal/domain/entity"
)

// OmniboxMode selects the active mode tab.
type OmniboxMode int

const (
	OmniboxModeSearch OmniboxMode = iota
	OmniboxModeAdd
)

// OmniboxConfig controls the initial state of the omnibox when opened.
// Vault callbacks allow the omnibox to list, read, and decrypt entries
// without importing the domain service directly.
type OmniboxConfig struct {
	Mode     OmniboxMode
	Category entity.EntryType // EntryTypeUnspecified means "All"

	// Vault operation callbacks.
	ListEntries   func(ctx context.Context, t entity.EntryType) ([]*entity.Envelope, error)
	GetEntry      func(ctx context.Context, id string) (*entity.Envelope, error)
	DecryptAndUse func(ctx context.Context, ciphertext []byte, fn func(plaintext []byte)) error
	AddEntry      func(ctx context.Context, env *entity.Envelope) error
	UpdateEntry   func(ctx context.Context, env *entity.Envelope) error

	// Notify sends a desktop notification. Nil means notifications are silently dropped.
	Notify func(ctx context.Context, summary, body string, urgency Urgency, icon string)
}

// GUIPort abstracts the graphical user interface.
// It embeds PINPromptPort so the GUI adapter handles both PIN and omnibox.
type GUIPort interface {
	PINPromptPort
	ShowOmnibox(ctx context.Context, cfg OmniboxConfig) error
	// SetPendingPIN stores a PIN validation function for the next ShowOmnibox
	// call. When set, ShowOmnibox shows the PIN prompt first within the same
	// GTK application, avoiding layer-shell focus issues from running two
	// separate GTK apps sequentially.
	SetPendingPIN(validate PINValidateFunc)
}
