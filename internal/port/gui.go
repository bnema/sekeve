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
type OmniboxConfig struct {
	Mode     OmniboxMode
	Category entity.EntryType // EntryTypeUnspecified means "All"
}

// GUIPort abstracts the graphical user interface.
// It embeds PINPromptPort so the GUI adapter handles both PIN and omnibox.
type GUIPort interface {
	PINPromptPort
	ShowOmnibox(ctx context.Context, cfg OmniboxConfig) error
}
