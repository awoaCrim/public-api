package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractLLMReviewSnippetKeepsRecentUserMessages verifies the review
// snippet keeps a bounded system prefix and the most recent messages with the
// last user message always retained.
func TestExtractLLMReviewSnippetKeepsRecentUserMessages(t *testing.T) {
	var msgs strings.Builder
	msgs.WriteString(`{"messages":[`)
	msgs.WriteString(`{"role":"system","content":"` + strings.Repeat("S", 500) + `"},`)
	for i := 0; i < 30; i++ {
		msgs.WriteString(`{"role":"user","content":"old message ` + string(rune('a'+i%26)) + `"},`)
	}
	msgs.WriteString(`{"role":"assistant","content":"assistant reply"},`)
	msgs.WriteString(`{"role":"user","content":"the latest user message"}`)
	msgs.WriteString(`]}`)

	snippet := ExtractLLMReviewSnippet([]byte(msgs.String()))
	require.NotEmpty(t, snippet)
	assert.Contains(t, snippet, "the latest user message", "the last user message must be retained")
	assert.Contains(t, snippet, "[system]", "a bounded system prefix must be retained")
	assert.Contains(t, snippet, "old message m", "recent history inside the 20-message window must be retained")
	assert.NotContains(t, snippet, "old message i", "history beyond the 20-message window must be dropped")
	for _, line := range strings.Split(snippet, "\n") {
		if strings.HasPrefix(line, "[system]") {
			assert.LessOrEqual(t, strings.Count(line, "S"), 200, "system prefix must be capped at 200 runes")
		}
	}
}

// TestExtractLLMReviewSnippetExcludesMultimodal verifies text-only extraction:
// image/audio/file content and base64 payloads never enter the review snippet.
func TestExtractLLMReviewSnippetExcludesMultimodal(t *testing.T) {
	body := `{"messages":[` +
		`{"role":"user","content":[{"type":"text","text":"keep this text"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}},{"type":"input_audio","input_audio":{"data":"base64audiodata"}},{"type":"file","file":{"file_data":"base64filedata"}}]}` +
		`]}`

	snippet := ExtractLLMReviewSnippet([]byte(body))
	assert.Contains(t, snippet, "keep this text")
	assert.NotContains(t, snippet, "base64")
	assert.NotContains(t, snippet, "image_url")
	assert.NotContains(t, snippet, "input_audio")
	assert.NotContains(t, snippet, "file_data")
}

// TestExtractLLMReviewSnippetMasksEmbeddedCredentials verifies the final
// credential pass masks bearer tokens embedded in message text.
func TestExtractLLMReviewSnippetMasksEmbeddedCredentials(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"use token sk-proj-abcdef and Authorization: Bearer abc"}]}`
	snippet := ExtractLLMReviewSnippet([]byte(body))
	assert.NotContains(t, snippet, "sk-proj-abcdef")
	assert.NotContains(t, snippet, "Bearer abc")
	assert.Contains(t, snippet, "sk-***")
}

// TestExtractLLMReviewSnippetUnicodeSafeTruncation verifies rune-safe
// truncation with an explicit truncation marker and no mangled UTF-8.
func TestExtractLLMReviewSnippetUnicodeSafeTruncation(t *testing.T) {
	content := strings.Repeat("中文😀", 3000)
	body := `{"messages":[{"role":"user","content":"` + content + `"}]}`
	snippet := ExtractLLMReviewSnippet([]byte(body))
	assert.NotEmpty(t, snippet)
	assert.NotContains(t, snippet, "\uFFFD", "truncation must never split a rune")
	assert.Contains(t, snippet, "...[truncated]")
}

// TestExtractLLMReviewSnippetNonChatFallback verifies the non-chat fallback
// returns a bounded whitelist-redacted summary.
func TestExtractLLMReviewSnippetNonChatFallback(t *testing.T) {
	body := `{"model":"gpt-4o","input":"some input"}` + strings.Repeat("x", 2000)
	snippet := ExtractLLMReviewSnippet([]byte(body))
	assert.NotEmpty(t, snippet)
	assert.LessOrEqual(t, len([]rune(snippet)), 550)
}

// TestExtractLLMReviewSnippetEmptyBody verifies empty/malformed inputs fail
// safe with an empty snippet.
func TestExtractLLMReviewSnippetEmptyBody(t *testing.T) {
	assert.Empty(t, ExtractLLMReviewSnippet(nil))
	assert.Empty(t, ExtractLLMReviewSnippet([]byte("")))
	assert.Empty(t, ExtractLLMReviewSnippet([]byte(`{}`)))
}
