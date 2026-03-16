package port

import "errors"

var (
	ErrNotFound      = errors.New("entry not found")
	ErrAlreadyExists = errors.New("entry already exists")
)
