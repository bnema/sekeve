package pinprompt

import (
	"fmt"
	"os"

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
	pin, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("failed to read PIN: %w", err)
	}
	return string(pin), nil
}
