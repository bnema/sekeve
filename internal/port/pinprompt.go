package port

import "context"

// PINPromptPort abstracts PIN entry from the user.
//
// PromptForPIN asks for a PIN and returns a non-empty string on success.
// errorMode controls error styling (red border in GUI, prefix in TTY).
// When errorMode is true and message is empty, a default error message is shown.
// When message is non-empty it is always displayed regardless of errorMode.
// Returns ErrPINPromptCancelled if the user dismisses the prompt.
type PINPromptPort interface {
	PromptForPIN(ctx context.Context, errorMode bool, message string) (string, error)
	IsTTY() bool
}
