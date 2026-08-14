package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMaskIPMatchesExactIPv4Semantics guards the partial-mask contract used by
// review task lists: IPv4 keeps the first two octets, IPv6 keeps the first
// four expanded groups, invalid input collapses to a full mask.
func TestMaskIP(t *testing.T) {
	assert.Equal(t, "192.168.***.***", MaskIP("192.168.1.10"))
	assert.Equal(t, "203.0.***.***", MaskIP("203.0.113.7"))
	assert.Equal(t, "", MaskIP(""))
	assert.Equal(t, "***", MaskIP("not-an-ip"))
	assert.Equal(t, "***", MaskIP("192.168.1.10:8080"))

	v6 := MaskIP("2001:db8:0:0:1:2:3:4")
	assert.True(t, strings.HasPrefix(v6, "2001:0db8:"), "IPv6 must keep the first four expanded groups")
	assert.True(t, strings.HasSuffix(v6, ":****"), "IPv6 must mask the trailing groups")
	assert.NotContains(t, v6, "1:2:3:4", "the full IPv6 suffix must never appear")
}

// TestHashClientIPIsDeterministicAndNormalized verifies the irreversible
// per-IP hash: same canonical IPv4 always hashes identically, different IPs
// hash differently, and the hash is not reversible to the raw address.
func TestHashClientIP(t *testing.T) {
	require.NotEmpty(t, HashClientIP("192.168.1.10"))
	assert.Equal(t, HashClientIP("192.168.1.10"), HashClientIP(" 192.168.1.10 "), "canonical IPv4 must hash identically after normalization")
	assert.NotEqual(t, HashClientIP("192.168.1.10"), HashClientIP("192.168.1.11"))
	assert.NotContains(t, HashClientIP("192.168.1.10"), "192.168.1.10")
	assert.Empty(t, HashClientIP(""))
}
