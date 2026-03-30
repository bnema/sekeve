package pinprompt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/bnema/sekeve/internal/port"
	"golang.org/x/term"
)

// maxTTYIterations is a defensive upper bound to prevent infinite loops from
// buggy callers that never return a PINFatalError.
const maxTTYIterations = 10

func promptTTY(ctx context.Context, validate port.PINValidateFunc) error {
	for range maxTTYIterations {
		if err := ctx.Err(); err != nil {
			return err
		}

		fmt.Fprint(os.Stderr, "Unlock PIN: ")
		pinBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return port.ErrPINPromptCancelled
			}
			return fmt.Errorf("failed to read PIN: %w", err)
		}
		if len(pinBytes) == 0 {
			return port.ErrPINPromptCancelled
		}
		pin := string(pinBytes)
		clear(pinBytes)

		if vErr := validate(ctx, pin); vErr != nil {
			var fatal *port.PINFatalError
			if errors.As(vErr, &fatal) {
				return fatal.Err
			}
			fmt.Fprintln(os.Stderr, vErr.Error())
			continue
		}

		return nil
	}
	return fmt.Errorf("too many PIN attempts")
}
