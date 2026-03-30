package pinprompt

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/bnema/sekeve/internal/port"
	"golang.org/x/term"
)

const defaultPINError = "Incorrect PIN, please try again."

func promptTTY(errorMode bool, message string) (string, error) {
	if errorMode && message == "" {
		fmt.Fprintln(os.Stderr, defaultPINError)
	} else if message != "" {
		fmt.Fprintln(os.Stderr, message)
	}

	fmt.Fprint(os.Stderr, "Unlock PIN: ")
	pinBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", port.ErrPINPromptCancelled
		}
		return "", fmt.Errorf("failed to read PIN: %w", err)
	}
	if len(pinBytes) == 0 {
		return "", port.ErrPINPromptCancelled
	}
	pinStr := string(pinBytes)
	for i := range pinBytes {
		pinBytes[i] = 0
	}
	return pinStr, nil
}
