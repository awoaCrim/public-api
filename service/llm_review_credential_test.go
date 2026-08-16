package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewAPIKeyEncryptDecryptRoundtrip(t *testing.T) {
	original := common.CryptoSecret
	common.CryptoSecret = "llm-review-test-secret"
	t.Cleanup(func() { common.CryptoSecret = original })

	enc, err := EncryptLLMReviewAPIKey("sk-super-secret-key")
	require.NoError(t, err)
	assert.NotEmpty(t, enc)
	assert.NotContains(t, enc, "sk-super-secret-key")

	plain, err := DecryptLLMReviewAPIKey(enc)
	require.NoError(t, err)
	assert.Equal(t, "sk-super-secret-key", plain)
}

func TestReviewAPIKeyEmptyRoundtrip(t *testing.T) {
	enc, err := EncryptLLMReviewAPIKey("")
	require.NoError(t, err)
	assert.Empty(t, enc)
	plain, err := DecryptLLMReviewAPIKey("")
	require.NoError(t, err)
	assert.Empty(t, plain)
}

func TestReviewAPIKeyDecryptsLegacyEnvelope(t *testing.T) {
	original := common.CryptoSecret
	common.CryptoSecret = "llm-review-test-secret"
	t.Cleanup(func() { common.CryptoSecret = original })

	legacy, err := encryptLegacyReviewAPIKeyForTest("sk-legacy-key")
	require.NoError(t, err)

	plain, err := DecryptLLMReviewAPIKey(legacy)
	require.NoError(t, err)
	assert.Equal(t, "sk-legacy-key", plain)
}

func encryptLegacyReviewAPIKeyForTest(plain string) (string, error) {
	key := sha256.Sum256([]byte(common.CryptoSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(append([]byte("v1:"), sealed...)), nil
}

func TestReviewAPIKeyRejectsTamperedEnvelope(t *testing.T) {
	original := common.CryptoSecret
	common.CryptoSecret = "llm-review-test-secret"
	t.Cleanup(func() { common.CryptoSecret = original })

	enc, err := EncryptLLMReviewAPIKey("sk-key-value")
	require.NoError(t, err)

	// Flip one character in the ciphertext tail.
	chars := []byte(enc)
	chars[len(chars)-1] = chars[len(chars)-1] ^ 0xFF
	_, err = DecryptLLMReviewAPIKey(string(chars))
	assert.ErrorIs(t, err, ErrReviewSecretCorrupt)

	// Wrong key material also fails closed.
	common.CryptoSecret = "different-secret"
	_, err = DecryptLLMReviewAPIKey(enc)
	assert.ErrorIs(t, err, ErrReviewSecretCorrupt)

	// Truncated envelope.
	_, err = DecryptLLMReviewAPIKey(enc[:10])
	assert.ErrorIs(t, err, ErrReviewSecretCorrupt)
}

func TestReviewAPIKeyRequiresConfiguredSecret(t *testing.T) {
	original := common.CryptoSecret
	common.CryptoSecret = ""
	t.Cleanup(func() { common.CryptoSecret = original })

	_, err := EncryptLLMReviewAPIKey("sk-key")
	assert.ErrorIs(t, err, ErrReviewSecretNotConfigured)
}

func TestMaskAPIKeyDerivesTailMask(t *testing.T) {
	assert.Empty(t, operation_setting.MaskAPIKey(""))
	assert.Equal(t, "****", operation_setting.MaskAPIKey("short"))
	assert.Equal(t, "sk-****klmn", operation_setting.MaskAPIKey("sk-abcdefghijklmn"), "mask must reveal only a bounded tail")
}
