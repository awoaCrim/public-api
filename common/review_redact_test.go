package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMaskReviewCredentialText guards the credential-masking contract used by
// LLM review attempt records, raw responses and error text: bearer tokens,
// sk-* keys, data URIs, JWTs and key/value credential pairs must never reach
// persisted review records.
func TestMaskReviewCredentialText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain text untouched", "hello world", "hello world"},
		{"authorization header", "Authorization: Bearer sk-abc123", "Authorization: ***"},
		{"bare bearer token", "Bearer abc123DEF", "Bearer ***"},
		{"sk key", "sk-proj-abcdef123456", "sk-***"},
		{"data uri", "data:image/png;base64,iVBORw0KGgo=", "data:image/***;base64,***"},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.abcd.efgh", "***.jwt.***"},
		{"password pair", "password=hunter2", "password=***"},
		{"api key colon pair", "api_key: abc", "api_key: ***"},
		{"cookie header", "Cookie: session=abc", "Cookie: ***"},
		{"refresh token pair", "refresh_token=abc.def", "refresh_token=***"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MaskReviewCredentialText(tt.in))
		})
	}
}

// TestRedactReviewJSONKeepsOnlyWhitelistAndDropsSensitive verifies the
// recursive whitelist redaction: only model/input/prompt/max_tokens/
// temperature/top_p/stream survive at the top level, sensitive keys are
// dropped entirely, unknown keys are dropped (fail-closed), and nested
// messages/content trees are recursed with string values credential-masked.
func TestRedactReviewJSONKeepsOnlyWhitelistAndDropsSensitive(t *testing.T) {
	in := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi Bearer sk-secret"}],` +
		`"api_key":"sk-top-secret","max_tokens":10,"stream":true,"temperature":0.2,"unknown_field":"drop"}`
	out := RedactReviewJSON([]byte(in))

	var got map[string]any
	require.NoError(t, Unmarshal([]byte(out), &got))
	assert.Equal(t, "gpt-4o", got["model"])
	assert.Equal(t, float64(10), got["max_tokens"])
	assert.Equal(t, true, got["stream"])
	assert.Equal(t, 0.2, got["temperature"])
	assert.NotContains(t, got, "api_key")
	assert.NotContains(t, got, "unknown_field")

	messages, ok := got["messages"].([]any)
	require.True(t, ok, "messages must survive as an array")
	require.Len(t, messages, 1)
	msg, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, msg, "role", "role is not whitelisted and must be dropped")
	content, ok := msg["content"].(string)
	require.True(t, ok, "string content must survive")
	assert.Equal(t, "hi Bearer ***", content, "credentials inside message text must be masked")
}

// TestRedactReviewJSONNonJSONFallback verifies the non-JSON fallback keeps a
// bounded, credential-masked prefix instead of echoing raw input.
func TestRedactReviewJSONNonJSONFallback(t *testing.T) {
	long := ""
	for i := 0; i < 800; i++ {
		long += "x"
	}
	out := RedactReviewJSON([]byte("not json, password=hunter2, " + long))
	assert.Contains(t, out, "password=***")
	assert.LessOrEqual(t, len([]rune(out)), 503, "fallback must be truncated to 500 runes plus ellipsis")
	assert.Empty(t, RedactReviewJSON(nil))
	assert.Empty(t, RedactReviewJSON([]byte("")))
}
