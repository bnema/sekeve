package crypto

import (
	"bytes"
	"context"
	"os/exec"
	"runtime/secret"

	"github.com/bnema/zerowrap"
)

type GPGAdapter struct{}

func NewGPGAdapter() *GPGAdapter {
	return &GPGAdapter{}
}

func (a *GPGAdapter) Encrypt(ctx context.Context, plaintext []byte, keyID string) ([]byte, error) {
	log := zerowrap.FromCtx(ctx)
	ctx = zerowrap.CtxWithFields(ctx, map[string]any{
		zerowrap.FieldAdapter: "gpg",
		zerowrap.FieldAction:  "encrypt",
	})

	cmd := exec.CommandContext(ctx, "gpg",
		"--batch", "--yes",
		"--recipient", keyID,
		"--trust-model", "always",
		"--encrypt",
	)
	cmd.Stdin = bytes.NewReader(plaintext)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, log.WrapErrf(err, "gpg encrypt failed: %s", stderr.String())
	}

	return stdout.Bytes(), nil
}

func (a *GPGAdapter) DecryptAndUse(ctx context.Context, ciphertext []byte, fn func(plaintext []byte)) error {
	log := zerowrap.FromCtx(ctx)
	ctx = zerowrap.CtxWithFields(ctx, map[string]any{
		zerowrap.FieldAdapter: "gpg",
		zerowrap.FieldAction:  "decrypt",
	})

	var decryptErr error
	secret.Do(func() {
		cmd := exec.CommandContext(ctx, "gpg",
			"--batch", "--yes",
			"--decrypt",
		)
		cmd.Stdin = bytes.NewReader(ciphertext)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			decryptErr = log.WrapErrf(err, "gpg decrypt failed: %s", stderr.String())
			return
		}

		fn(stdout.Bytes())
	})

	return decryptErr
}

func (a *GPGAdapter) ExportPublicKey(ctx context.Context, keyID string) ([]byte, error) {
	log := zerowrap.FromCtx(ctx)

	cmd := exec.CommandContext(ctx, "gpg",
		"--batch", "--yes",
		"--export", "--armor",
		keyID,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, log.WrapErrf(err, "gpg export failed: %s", stderr.String())
	}

	return stdout.Bytes(), nil
}
