package common

import (
	"regexp"
	"strings"
)

// Review record credential redaction.
//
// MaskSensitiveInfo only masks narrow api_key/URL/IP shapes. Review attempt
// records, raw upstream responses and error text need a stronger contract:
// authorization headers, cookies, access/refresh tokens, passwords, secrets,
// sk-* keys, JWTs and base64 data URIs must be reliably removed before the
// text is persisted or forwarded anywhere.

// reviewSensitiveKeyPattern matches common sensitive JSON key names.
var reviewSensitiveKeyPattern = regexp.MustCompile(`(?i)^(authorization|auth|api[-_]?key|apikey|access[-_]?token|token|password|passwd|pwd|secret|client[-_]?secret|cookie|cookies|set-cookie|refresh[-_]?token|private[-_]?key|session[-_]?(id|token)|signature|sign|jwt)$`)

// reviewCredentialValuePattern matches credential shapes inside free text.
var reviewCredentialValuePattern = regexp.MustCompile(`(?i)(Bearer\s+[A-Za-z0-9\-._~+/]+=*|Authorization\s*[:=]\s*(?:Bearer\s+)?\S+|Cookie\s*[:=]\s*\S+|Set-Cookie\s*[:=]\s*\S+|api[_-]?key\s*[:=]\s*["']?\S+|access[_-]?token\s*[:=]\s*\S+|refresh[_-]?token\s*[:=]\s*\S+|password\s*[:=]\s*\S+|secret\s*[:=]\s*\S+|sk-[A-Za-z0-9\-_]{6,}|data:[^;\s,]+;base64,[A-Za-z0-9+/=]+|eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{4,})`)

// base64DataURIPattern catches any data URI that survived the value pattern.
var base64DataURIPattern = regexp.MustCompile(`(?i)data:[^;\s,]+;base64,[A-Za-z0-9+/=]+`)

// isReviewSensitiveKey reports whether a JSON key name is credential-like.
func isReviewSensitiveKey(key string) bool {
	return reviewSensitiveKeyPattern.MatchString(key)
}

// MaskReviewCredentialText masks credential shapes in free text: bearer
// tokens, authorization/cookie headers, api keys, passwords, secrets, sk-*
// keys, data URIs and JWTs. It never modifies MaskSensitiveInfo itself.
func MaskReviewCredentialText(s string) string {
	if s == "" {
		return ""
	}
	s = reviewCredentialValuePattern.ReplaceAllStringFunc(s, func(m string) string {
		lower := strings.ToLower(m)
		switch {
		case strings.HasPrefix(lower, "bearer "):
			return "Bearer ***"
		case strings.HasPrefix(lower, "sk-"):
			return "sk-***"
		case strings.HasPrefix(lower, "data:"):
			return "data:image/***;base64,***"
		case strings.HasPrefix(m, "eyJ"):
			return "***.jwt.***"
		default:
			// key=value / key:value shapes. A colon form keeps a single
			// space for readability ("Authorization: ***").
			if idx := strings.IndexAny(m, "=:"); idx > 0 {
				if m[idx] == ':' {
					return m[:idx+1] + " ***"
				}
				return m[:idx+1] + "***"
			}
			return "***"
		}
	})
	// Fallback: any data URI that still remains is masked in full.
	s = base64DataURIPattern.ReplaceAllString(s, "data:***;base64,***")
	return s
}

// RedactReviewJSON recursively redacts arbitrary JSON to a small whitelist:
// only model/input/prompt/max_tokens/temperature/top_p/stream survive at the
// top level; messages/content subtrees are recursed; sensitive keys are
// dropped entirely; every unknown key is dropped fail-closed; string values
// are credential-masked. Unparseable input falls back to a masked, truncated
// prefix. It returns an empty string when nothing survives.
func RedactReviewJSON(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var root any
	if err := Unmarshal(data, &root); err != nil {
		// Non-JSON: mask credentials and keep at most 500 runes.
		return truncateRunes(MaskReviewCredentialText(MaskSensitiveInfo(string(data))), 500)
	}
	redacted := redactReviewValue(root)
	out, err := Marshal(redacted)
	if err != nil {
		return ""
	}
	return string(out)
}

// redactReviewValue recurses over a decoded JSON value applying the review
// whitelist.
func redactReviewValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any)
		for key, val := range t {
			lower := strings.ToLower(key)
			if isReviewSensitiveKey(lower) {
				// Credential-like keys are dropped entirely, including any
				// nested file/audio/image payloads they may carry.
				continue
			}
			switch lower {
			case "model", "input", "prompt", "max_tokens", "temperature", "top_p", "stream":
				// Whitelisted fields survive; string values are masked.
				if s, ok := val.(string); ok {
					out[key] = MaskReviewCredentialText(s)
				} else {
					out[key] = redactReviewValue(val)
				}
			case "messages", "content":
				out[key] = redactReviewValue(val)
			default:
				// Unknown fields are dropped fail-closed.
			}
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, redactReviewValue(item))
		}
		return out
	case string:
		return MaskReviewCredentialText(t)
	default:
		return t
	}
}
