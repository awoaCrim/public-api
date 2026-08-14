package requestsnapshot

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/crypto/hkdf"
)

// Snapshot envelope layout (all multi-byte integers are big-endian):
//
//	offset 0  : magic "RSNAP1\x00" (7 bytes)
//	offset 7  : envelope version (1 byte)
//	offset 8  : key version (2 bytes)
//	offset 10 : AES-GCM nonce (12 bytes)
//	offset 22 : ciphertext (AAD-bound GCM seal)
const (
	envelopeMagic     = "RSNAP1\x00"
	envelopeVersion   = 1
	currentKeyVersion = 1
	nonceSize         = 12
	envelopeHeaderLen = 7 + 1 + 2 + nonceSize
	gcmKeySize        = 32
)

// snapshotSalt and snapshotInfoPrefix provide domain separation so key material
// derived for request snapshots can never be confused with other uses of
// CryptoSecret (HMACs, session cookies, tokens, ...).
var snapshotSalt = []byte("new-api/request-snapshot")

// deriveKey derives the AES-256-GCM key for a given key version using
// HKDF-SHA256 keyed by common.CryptoSecret. Key versions are forward-scoped in
// the HKDF info so a future rotation produces different keys while old
// envelopes stay readable.
func deriveKey(keyVersion int) []byte {
	info := []byte(fmt.Sprintf("new-api/request-snapshot/key-v%d", keyVersion))
	reader := hkdf.New(sha256.New, []byte(common.CryptoSecret), snapshotSalt, info)
	key := make([]byte, gcmKeySize)
	if _, err := io.ReadFull(reader, key); err != nil {
		// HKDF with SHA-256 can always fill 32 bytes; this is unreachable.
		panic(fmt.Sprintf("requestsnapshot: HKDF output short: %v", err))
	}
	return key
}

// snapshotAAD binds the ciphertext to the request and file identity so an
// envelope copied under a different request id or file name never decrypts.
func snapshotAAD(requestID, relativePath string) []byte {
	return []byte("new-api/request-snapshot|" + requestID + "|" + relativePath)
}

// encryptSnapshot seals plain into a versioned envelope. The nonce is freshly
// random per call; the caller is responsible for supplying the exact request
// id and relative path that will be used at decryption time.
func encryptSnapshot(plain []byte, requestID, relativePath string) ([]byte, error) {
	block, err := aes.NewCipher(deriveKey(currentKeyVersion))
	if err != nil {
		return nil, fmt.Errorf("requestsnapshot: aes init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("requestsnapshot: gcm init: %w", err)
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("requestsnapshot: nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, plain, snapshotAAD(requestID, relativePath))

	out := make([]byte, 0, envelopeHeaderLen+len(sealed))
	out = append(out, envelopeMagic...)
	out = append(out, envelopeVersion)
	out = binary.BigEndian.AppendUint16(out, uint16(currentKeyVersion))
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

// decryptSnapshot opens a versioned envelope. Every integrity failure — wrong
// key material, tampered ciphertext, truncated or malformed data — collapses
// into ErrSnapshotCorrupt so callers fail closed and never surface partial
// bytes.
func decryptSnapshot(envelope []byte, requestID, relativePath string) ([]byte, error) {
	if len(envelope) < envelopeHeaderLen {
		return nil, ErrSnapshotCorrupt
	}
	if string(envelope[:len(envelopeMagic)]) != envelopeMagic {
		return nil, ErrSnapshotCorrupt
	}
	if envelope[len(envelopeMagic)] != envelopeVersion {
		return nil, ErrSnapshotCorrupt
	}
	keyVersion := int(binary.BigEndian.Uint16(envelope[len(envelopeMagic)+1 : len(envelopeMagic)+3]))
	if keyVersion <= 0 || keyVersion > 65535 {
		return nil, ErrSnapshotCorrupt
	}
	nonce := envelope[len(envelopeMagic)+3 : envelopeHeaderLen]
	sealed := envelope[envelopeHeaderLen:]

	block, err := aes.NewCipher(deriveKey(keyVersion))
	if err != nil {
		return nil, ErrSnapshotCorrupt
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrSnapshotCorrupt
	}
	plain, err := gcm.Open(nil, nonce, sealed, snapshotAAD(requestID, relativePath))
	if err != nil {
		return nil, ErrSnapshotCorrupt
	}
	if len(plain) == 0 {
		// GCM.Open returns a nil slice for an empty plaintext; normalize so the
		// exact captured bytes (including an empty body) round-trip unchanged.
		return []byte{}, nil
	}
	return plain, nil
}
