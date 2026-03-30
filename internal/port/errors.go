package port

import "errors"

// DefaultPINError is the user-facing message for an incorrect PIN attempt.
const DefaultPINError = "Incorrect PIN, please try again."

var (
	ErrNotFound           = errors.New("entry not found")
	ErrAlreadyExists      = errors.New("entry already exists")
	ErrPINPromptCancelled = errors.New("PIN prompt cancelled")
	ErrNoPINInputMethod   = errors.New("no PIN input method available (no GUI display and no TTY)")
)

// PINFatalError wraps an error to signal that the PIN prompt should close
// immediately rather than allowing retry.
type PINFatalError struct{ Err error }

func (e *PINFatalError) Error() string { return e.Err.Error() }
func (e *PINFatalError) Unwrap() error { return e.Err }
