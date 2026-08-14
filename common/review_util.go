package common

import (
	"encoding/hex"
	"net"
	"strings"
)

// MaskIP partially masks an IP address for review task lists. Complete IPs
// remain administrator-detail data; list views only receive a partial mask.
// IPv4 keeps the first two octets; IPv6 keeps the first four expanded groups;
// invalid input collapses to a full mask.
func MaskIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		// Non-standard values (hostnames, host:port pairs, ...) are fully
		// masked rather than echoed.
		return "***"
	}
	if v4 := parsed.To4(); v4 != nil {
		parts := strings.Split(v4.String(), ".")
		if len(parts) == 4 {
			return parts[0] + "." + parts[1] + ".***.***"
		}
		return "***"
	}
	// IPv6: expand via To16 so compressed forms never produce empty groups,
	// then keep the first four groups (the 64-bit prefix).
	raw := parsed.To16()
	if raw == nil {
		return "***"
	}
	groups := make([]string, 8)
	for i := 0; i < 8; i++ {
		groups[i] = hex.EncodeToString(raw[i*2 : i*2+2])
	}
	return strings.Join(groups[:4], ":") + ":****"
}

// HashClientIP returns an irreversible per-IP hash for review payloads so the
// raw client IP is never sent to the review LLM or stored in the payload.
// The HMAC-SHA256 key is CryptoSecret, which also prevents correlating review
// records with plaintext IP logs. Canonical IPv4 values are normalized first
// so whitespace variants hash identically.
func HashClientIP(ip string) string {
	if ip == "" {
		return ""
	}
	// Normalize canonical IPv4 (same exact-IPv4 rule as the IP blacklist);
	// IPv6 and other values are hashed as-is.
	if normalized, ok := NormalizeIPv4(ip); ok {
		ip = normalized
	}
	return hex.EncodeToString(HmacSha256Raw([]byte(ip), []byte(CryptoSecret)))
}
