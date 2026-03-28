package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
	argonKeyLen  = 32
	pinSaltLen   = 16
)

// HashPIN returns an argon2id hash and random salt for the given PIN.
func HashPIN(pin string) (hash, salt []byte, err error) {
	if pin == "" {
		return nil, nil, fmt.Errorf("PIN must not be empty")
	}
	salt = make([]byte, pinSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	hash = argon2.IDKey([]byte(pin), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return hash, salt, nil
}

// VerifyPIN checks a PIN against a stored hash and salt.
func VerifyPIN(pin string, hash, salt []byte) bool {
	computed := argon2.IDKey([]byte(pin), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return subtle.ConstantTimeCompare(computed, hash) == 1
}
