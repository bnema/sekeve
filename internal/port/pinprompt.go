package port

import "context"

// PINValidateFunc checks a PIN entered by the user.
// Return nil if the PIN is correct.
// Return a *PINFatalError to close the prompt immediately (e.g. max retries).
// Return any other error to display its message and let the user retry.
//
// Callers are responsible for enforcing retry limits by returning a
// *PINFatalError after the desired number of failed attempts.
type PINValidateFunc func(ctx context.Context, pin string) error

// PINPromptPort abstracts PIN entry from the user.
//
// PromptForPIN shows a PIN prompt and calls validate for each submission.
// The prompt stays open until validate returns nil (success), the user
// cancels (Escape / window close → ErrPINPromptCancelled), or the context
// is done. A 90-second inactivity timeout auto-closes the GUI prompt.
type PINPromptPort interface {
	PromptForPIN(ctx context.Context, validate PINValidateFunc) error
	IsTTY() bool
}
