package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/crypto/hkdf"
)

// The review API key is encrypted at rest in the option table and only ever
// decrypted in memory for the reviewer call. Key material is derived from
// common.CryptoSecret with HKDF-SHA256 and domain-separated salts so it can
// never be confused with snapshot/token/cookie keys.

const (
	reviewKeyMagic     = "RVK1\x00"
	reviewKeyVersion   = 1
	reviewKeyNonceSize = 12
	reviewKeySalt      = "new-api/llm-review-api-key"
	reviewKeyInfo      = "new-api/llm-review/key-v1"
)

// ErrReviewSecretNotConfigured means the global crypto secret is missing.
var ErrReviewSecretNotConfigured = errors.New("crypto secret is not configured")

// reviewKeyDerive derives the AES-256-GCM key from CryptoSecret.
func reviewKeyDerive() ([]byte, error) {
	if common.CryptoSecret == "" {
		return nil, ErrReviewSecretNotConfigured
	}
	reader := hkdf.New(sha256.New, []byte(common.CryptoSecret), []byte(reviewKeySalt), []byte(reviewKeyInfo))
	key := make([]byte, 32)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("llm review key derivation failed: %w", err)
	}
	return key, nil
}

// EncryptLLMReviewAPIKey seals the plaintext API key into a versioned
// base64 envelope for option-table persistence.
func EncryptLLMReviewAPIKey(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	key, err := reviewKeyDerive()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, reviewKeyNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plain), nil)

	out := make([]byte, 0, len(reviewKeyMagic)+1+reviewKeyNonceSize+len(sealed))
	out = append(out, reviewKeyMagic...)
	out = append(out, reviewKeyVersion)
	out = append(out, nonce...)
	out = append(out, sealed...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// ErrReviewSecretCorrupt covers every integrity failure (wrong key material,
// tampering, truncation) so callers fail closed.
var ErrReviewSecretCorrupt = errors.New("llm review api key envelope is corrupt")

// DecryptLLMReviewAPIKey opens a versioned envelope. Empty input returns an
// empty key (no key configured).
func DecryptLLMReviewAPIKey(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", ErrReviewSecretCorrupt
	}
	if len(raw) < len(reviewKeyMagic)+1+reviewKeyNonceSize {
		return "", ErrReviewSecretCorrupt
	}
	if string(raw[:len(reviewKeyMagic)]) != reviewKeyMagic {
		return "", ErrReviewSecretCorrupt
	}
	if raw[len(reviewKeyMagic)] != reviewKeyVersion {
		return "", ErrReviewSecretCorrupt
	}
	nonce := raw[len(reviewKeyMagic)+1 : len(reviewKeyMagic)+1+reviewKeyNonceSize]
	sealed := raw[len(reviewKeyMagic)+1+reviewKeyNonceSize:]

	key, err := reviewKeyDerive()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrReviewSecretCorrupt
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrReviewSecretCorrupt
	}
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", ErrReviewSecretCorrupt
	}
	if len(plain) == 0 {
		return "", nil
	}
	return string(plain), nil
}
