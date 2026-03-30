package port

import "context"

type PINPromptPort interface {
	PromptForPIN(ctx context.Context, errorMode bool, message string) (string, error)
	IsTTY() bool // Whether the adapter uses terminal input (vs GUI)
}
