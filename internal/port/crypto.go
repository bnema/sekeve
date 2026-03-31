package port

import "context"

type CryptoPort interface {
	Encrypt(ctx context.Context, plaintext []byte, keyID string) ([]byte, error)
	DecryptAndUse(ctx context.Context, ciphertext []byte, fn func(plaintext []byte)) error
}

// ServerCryptoPort covers server-side crypto operations for authentication.
type ServerCryptoPort interface {
	Encrypt(ctx context.Context, plaintext []byte, keyID string) ([]byte, error)
	FingerprintFromArmored(ctx context.Context, armored []byte) (string, error)
	VerifyKeyIDMatchesFingerprint(ctx context.Context, keyID, expectedFP string, pubKey []byte) error
}
