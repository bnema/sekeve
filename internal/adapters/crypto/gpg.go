package crypto

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime/secret"
	"strings"

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

// ValidateArmoredPublicKey validates that the provided bytes are a valid
// armored GPG public key. It uses a temporary GNUPGHOME to avoid polluting
// any keyring. Returns the normalized (trimmed + trailing newline) key bytes.
func (a *GPGAdapter) ValidateArmoredPublicKey(ctx context.Context, armored []byte) ([]byte, error) {
	log := zerowrap.FromCtx(ctx)

	trimmed := bytes.TrimSpace(armored)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("public key is empty")
	}

	// Quick pre-check for PGP armor markers.
	if !bytes.Contains(trimmed, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----")) ||
		!bytes.Contains(trimmed, []byte("-----END PGP PUBLIC KEY BLOCK-----")) {
		return nil, fmt.Errorf("input does not look like an armored GPG public key")
	}

	// Validate using GPG in an isolated temp homedir.
	tmpDir, err := os.MkdirTemp("", "sekeve-gpg-validate-*")
	if err != nil {
		return nil, log.WrapErr(err, "failed to create temp dir for key validation")
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cmd := exec.CommandContext(ctx, "gpg",
		"--batch", "--quiet",
		"--homedir", tmpDir,
		"--no-autostart",
		"--with-colons",
		"--import-options", "show-only",
		"--import",
	)
	cmd.Stdin = bytes.NewReader(trimmed)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, log.WrapErrf(err, "gpg key validation failed: %s", stderr.String())
	}

	// Verify at least one public key record exists in the output.
	if !bytes.Contains(stdout.Bytes(), []byte("pub:")) {
		return nil, fmt.Errorf("no public key found in the provided input")
	}

	// Normalize: trim whitespace, ensure trailing newline.
	trimmed = append(trimmed, '\n')
	return trimmed, nil
}

// FingerprintFromArmored extracts the primary key fingerprint from an armored
// GPG public key. Uses a temporary GNUPGHOME to avoid polluting any keyring.
// Returns the uppercase 40-char hex fingerprint.
func (a *GPGAdapter) FingerprintFromArmored(ctx context.Context, armored []byte) (string, error) {
	tmpDir, err := os.MkdirTemp("", "sekeve-gpg-fp-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cmd := exec.CommandContext(ctx, "gpg",
		"--batch", "--quiet",
		"--homedir", tmpDir,
		"--no-autostart",
		"--with-colons",
		"--import-options", "show-only",
		"--import",
	)
	cmd.Stdin = bytes.NewReader(armored)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gpg fingerprint extraction failed: %s", stderr.String())
	}

	// Parse colon-delimited output for "fpr" record.
	for line := range strings.SplitSeq(stdout.String(), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 10 && fields[0] == "fpr" {
			fp := fields[9]
			if len(fp) != 40 {
				return "", fmt.Errorf("unexpected fingerprint length: %d", len(fp))
			}
			for _, c := range fp {
				if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
					return "", fmt.Errorf("fingerprint contains non-hex character: %c", c)
				}
			}
			return strings.ToUpper(fp), nil
		}
	}

	return "", fmt.Errorf("no fingerprint found in key")
}

// VerifyKeyIDMatchesFingerprint checks that a GPG key ID resolves to the
// expected fingerprint. Uses an isolated GNUPGHOME with the provided public key.
func (a *GPGAdapter) VerifyKeyIDMatchesFingerprint(ctx context.Context, keyID, expectedFP string, pubKey []byte) error {
	tmpDir, err := os.MkdirTemp("", "sekeve-gpg-verify-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Import the public key into the isolated homedir.
	importCmd := exec.CommandContext(ctx, "gpg",
		"--batch", "--quiet",
		"--homedir", tmpDir,
		"--no-autostart",
		"--import",
	)
	importCmd.Stdin = bytes.NewReader(pubKey)

	var importStderr bytes.Buffer
	importCmd.Stderr = &importStderr
	if err := importCmd.Run(); err != nil {
		return fmt.Errorf("gpg import failed: %s", importStderr.String())
	}

	// List keys matching the requested key ID.
	listCmd := exec.CommandContext(ctx, "gpg",
		"--batch",
		"--homedir", tmpDir,
		"--no-autostart",
		"--with-colons",
		"--list-keys", keyID,
	)

	var listOut, listErr bytes.Buffer
	listCmd.Stdout = &listOut
	listCmd.Stderr = &listErr
	if err := listCmd.Run(); err != nil {
		return fmt.Errorf("unknown GPG key ID: %s", listErr.String())
	}

	// Check if any fingerprint record matches the expected one.
	for line := range strings.SplitSeq(listOut.String(), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 10 && fields[0] == "fpr" && strings.EqualFold(fields[9], expectedFP) {
			return nil
		}
	}

	return fmt.Errorf("GPG key ID does not match registered key")
}
