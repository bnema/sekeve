package crypto_test

import (
	"bytes"
	"context"
	"os/exec"
	"testing"

	"github.com/bnema/sekeve/internal/adapters/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestGPGKey(t *testing.T) string {
	t.Helper()
	gnupgHome := t.TempDir()
	t.Setenv("GNUPGHOME", gnupgHome)

	input := `%no-protection
Key-Type: RSA
Key-Length: 2048
Subkey-Type: RSA
Subkey-Length: 2048
Name-Real: Test User
Name-Email: test@sekeve.dev
Expire-Date: 0
%commit
`
	cmd := exec.Command("gpg", "--batch", "--gen-key")
	cmd.Stdin = bytes.NewBufferString(input)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "gpg key generation failed: %s", string(out))
	return "test@sekeve.dev"
}

func TestGPGAdapter_EncryptDecrypt(t *testing.T) {
	keyID := setupTestGPGKey(t)
	adapter := crypto.NewGPGAdapter()

	tests := []struct {
		name      string
		plaintext []byte
		wantErr   bool
	}{
		{
			name:      "encrypt and decrypt simple text",
			plaintext: []byte("hello world"),
		},
		{
			name:      "encrypt and decrypt json payload",
			plaintext: []byte(`{"site":"github.com","username":"user","password":"secret123"}`),
		},
		{
			name:      "encrypt and decrypt empty payload",
			plaintext: []byte(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			ciphertext, err := adapter.Encrypt(ctx, tt.plaintext, keyID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEqual(t, tt.plaintext, ciphertext)

			var decrypted []byte
			err = adapter.DecryptAndUse(ctx, ciphertext, func(plaintext []byte) {
				decrypted = make([]byte, len(plaintext))
				copy(decrypted, plaintext)
			})
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestGPGAdapter_DecryptInvalidData(t *testing.T) {
	_ = setupTestGPGKey(t)
	adapter := crypto.NewGPGAdapter()

	err := adapter.DecryptAndUse(context.Background(), []byte("not-encrypted"), func(_ []byte) {})
	assert.Error(t, err)
}
