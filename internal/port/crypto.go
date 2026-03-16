package port

import "context"

type CryptoPort interface {
	Encrypt(ctx context.Context, plaintext []byte, keyID string) ([]byte, error)
	DecryptAndUse(ctx context.Context, ciphertext []byte, fn func(plaintext []byte)) error
}
