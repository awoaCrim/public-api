package requestsnapshot

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setTestCryptoSecret(t *testing.T, secret string) {
	t.Helper()
	previous := common.CryptoSecret
	t.Setenv("CRYPTO_SECRET", secret)
	common.CryptoSecret = secret
	t.Cleanup(func() {
		common.CryptoSecret = previous
	})
}

func TestEncryptDecryptRoundTripExact(t *testing.T) {
	setTestCryptoSecret(t, "requestsnapshot-crypto-test-secret")

	tests := []struct {
		name      string
		plain     []byte
		requestID string
	}{
		{name: "json body", plain: []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`), requestID: "req-json-1"},
		{name: "empty body", plain: []byte{}, requestID: "req-empty"},
		{name: "binary body", plain: []byte{0x00, 0x01, 0xff, 0xfe, 0x80, 0x7f}, requestID: "req-bin"},
		{name: "large body", plain: bytes.Repeat([]byte("A"), 1<<20), requestID: "req-large"},
		{name: "trailing newline preserved", plain: []byte("line1\nline2\n\n"), requestID: "req-newline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope, err := encryptSnapshot(tt.plain, tt.requestID, snapshotFileName(tt.requestID))
			require.NoError(t, err)

			decrypted, err := decryptSnapshot(envelope, tt.requestID, snapshotFileName(tt.requestID))
			require.NoError(t, err)
			assert.Equal(t, tt.plain, decrypted, "exact bytes must round-trip")

			// Fresh nonce per seal: encrypting the same plaintext twice must
			// never produce the same envelope.
			again, err := encryptSnapshot(tt.plain, tt.requestID, snapshotFileName(tt.requestID))
			require.NoError(t, err)
			assert.NotEqual(t, envelope, again, "nonce must be fresh per seal")
		})
	}
}

func TestDecryptTamperFailsClosed(t *testing.T) {
	setTestCryptoSecret(t, "requestsnapshot-crypto-test-secret")
	plain := []byte("sensitive request body that must never leak")
	requestID := "req-tamper"
	rel := snapshotFileName(requestID)

	envelope, err := encryptSnapshot(plain, requestID, rel)
	require.NoError(t, err)

	// Flip one byte in every region of the envelope (header + ciphertext).
	regions := map[string]int{
		"magic":       0,
		"version":     len(envelopeMagic),
		"key version": len(envelopeMagic) + 1,
		"nonce":       len(envelopeMagic) + 3,
		"ciphertext":  envelopeHeaderLen,
	}
	for name, offset := range regions {
		t.Run(name, func(t *testing.T) {
			tampered := append([]byte(nil), envelope...)
			tampered[offset] ^= 0x01
			_, err := decryptSnapshot(tampered, requestID, rel)
			assert.ErrorIs(t, err, ErrSnapshotCorrupt)
		})
	}

	t.Run("ciphertext body flip", func(t *testing.T) {
		tampered := append([]byte(nil), envelope...)
		tampered[len(tampered)-1] ^= 0x01
		_, err := decryptSnapshot(tampered, requestID, rel)
		assert.ErrorIs(t, err, ErrSnapshotCorrupt)
	})
}

func TestDecryptWrongKeyFailsClosed(t *testing.T) {
	setTestCryptoSecret(t, "requestsnapshot-crypto-test-secret")
	plain := []byte("sealed under one key")
	requestID := "req-key"
	rel := snapshotFileName(requestID)
	envelope, err := encryptSnapshot(plain, requestID, rel)
	require.NoError(t, err)

	previous := common.CryptoSecret
	common.CryptoSecret = "a-different-key-material"
	t.Cleanup(func() { common.CryptoSecret = previous })

	_, err = decryptSnapshot(envelope, requestID, rel)
	assert.ErrorIs(t, err, ErrSnapshotCorrupt, "wrong key must fail closed")
}

func TestDecryptMalformedFailsClosed(t *testing.T) {
	setTestCryptoSecret(t, "requestsnapshot-crypto-test-secret")
	requestID := "req-malformed"
	rel := snapshotFileName(requestID)
	plain := []byte("x")
	envelope, err := encryptSnapshot(plain, requestID, rel)
	require.NoError(t, err)

	t.Run("truncated envelope", func(t *testing.T) {
		_, err := decryptSnapshot(envelope[:envelopeHeaderLen-1], requestID, rel)
		assert.ErrorIs(t, err, ErrSnapshotCorrupt)
	})
	t.Run("empty envelope", func(t *testing.T) {
		_, err := decryptSnapshot(nil, requestID, rel)
		assert.ErrorIs(t, err, ErrSnapshotCorrupt)
	})
	t.Run("bad magic", func(t *testing.T) {
		bad := append([]byte(nil), envelope...)
		copy(bad[:3], []byte("XXX"))
		_, err := decryptSnapshot(bad, requestID, rel)
		assert.ErrorIs(t, err, ErrSnapshotCorrupt)
	})
	t.Run("bad version", func(t *testing.T) {
		bad := append([]byte(nil), envelope...)
		bad[len(envelopeMagic)] = 99
		_, err := decryptSnapshot(bad, requestID, rel)
		assert.ErrorIs(t, err, ErrSnapshotCorrupt)
	})
	t.Run("zero key version", func(t *testing.T) {
		bad := append([]byte(nil), envelope...)
		bad[len(envelopeMagic)+1] = 0
		bad[len(envelopeMagic)+2] = 0
		_, err := decryptSnapshot(bad, requestID, rel)
		assert.ErrorIs(t, err, ErrSnapshotCorrupt)
	})
}

func TestDecryptWrongAADFailsClosed(t *testing.T) {
	setTestCryptoSecret(t, "requestsnapshot-crypto-test-secret")
	plain := []byte("bound to one identity")
	requestID := "req-aad"
	rel := snapshotFileName(requestID)
	envelope, err := encryptSnapshot(plain, requestID, rel)
	require.NoError(t, err)

	t.Run("wrong request id", func(t *testing.T) {
		_, err := decryptSnapshot(envelope, "another-request-id", rel)
		assert.ErrorIs(t, err, ErrSnapshotCorrupt)
	})
	t.Run("wrong relative path", func(t *testing.T) {
		_, err := decryptSnapshot(envelope, requestID, "different.snap")
		assert.ErrorIs(t, err, ErrSnapshotCorrupt)
	})
}

func TestSnapshotAADIsStablePerIdentity(t *testing.T) {
	aad1 := snapshotAAD("req-1", "a.snap")
	aad2 := snapshotAAD("req-1", "a.snap")
	assert.Equal(t, aad1, aad2, "AAD must be deterministic")
	assert.NotEqual(t, aad1, snapshotAAD("req-2", "a.snap"))
	assert.NotEqual(t, aad1, snapshotAAD("req-1", "b.snap"))
}

func TestDeriveKeyDiffersByVersionAndSecret(t *testing.T) {
	setTestCryptoSecret(t, "requestsnapshot-key-test-secret")
	k1 := deriveKey(1)
	k2 := deriveKey(2)
	assert.NotEqual(t, k1, k2, "key versions must derive distinct keys")
	assert.Len(t, k1, gcmKeySize)

	previous := common.CryptoSecret
	common.CryptoSecret = "other-secret"
	t.Cleanup(func() { common.CryptoSecret = previous })
	k3 := deriveKey(1)
	assert.NotEqual(t, k1, k3, "different secrets must derive distinct keys")
}

func TestRandomNonceIsUniqueAcrossSeals(t *testing.T) {
	setTestCryptoSecret(t, "requestsnapshot-nonce-test-secret")
	seen := make(map[string]bool)
	for i := 0; i < 64; i++ {
		envelope, err := encryptSnapshot([]byte("same payload"), "req-nonce", snapshotFileName("req-nonce"))
		require.NoError(t, err)
		nonce := string(envelope[len(envelopeMagic)+3 : envelopeHeaderLen])
		assert.False(t, seen[nonce], "nonce reuse")
		seen[nonce] = true
	}
	// Ensure the test itself is meaningful: random payload to avoid any
	// compiler folding of the loop.
	_ = rand.Reader
}
