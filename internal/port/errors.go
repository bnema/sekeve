package port

import "errors"

var (
	ErrNotFound           = errors.New("entry not found")
	ErrAlreadyExists      = errors.New("entry already exists")
	ErrPINPromptCancelled = errors.New("PIN prompt cancelled")
	ErrNoPINInputMethod   = errors.New("no PIN input method available (no GUI display and no TTY)")
)
