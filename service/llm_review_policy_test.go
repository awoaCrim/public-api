package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeReviewPolicyTextStripsHTMLAndMarkdown(t *testing.T) {
	raw := "<p>Account sharing is <b>prohibited</b>.</p><script>alert(1)</script>" +
		"\n\n![img](https://evil.example/x.png)\n\nSee [rules](https://example.com/rules) for details."
	text := sanitizeReviewPolicyText(raw)
	assert.NotContains(t, text, "<p>")
	assert.NotContains(t, text, "script")
	assert.NotContains(t, text, "evil.example")
	assert.NotContains(t, text, "](https://example.com/rules)")
	assert.Contains(t, text, "Account sharing is prohibited.")
	assert.Contains(t, text, "See rules for details.")
}

func TestSanitizeReviewPolicyTextCapsLength(t *testing.T) {
	long := strings.Repeat("条款", 8000)
	text := sanitizeReviewPolicyText(long)
	assert.LessOrEqual(t, len([]rune(text)), reviewMaxPolicyTextRunes+40)
	assert.Contains(t, text, "[policy text truncated]")
}

func TestSanitizeReviewPolicyTextEmpty(t *testing.T) {
	assert.Empty(t, sanitizeReviewPolicyText(""))
	assert.Empty(t, sanitizeReviewPolicyText("  \n "))
}

func TestPayloadWithReviewPolicyBackfillsMissingPolicy(t *testing.T) {
	cfg := llmReviewSettingForTest(t)
	cfg.PolicyText = "No commercial use."

	out, policy := payloadWithReviewPolicy(`{"request_snippet":"x"}`)
	assert.Equal(t, "No commercial use.", policy)
	assert.Contains(t, out, `"policy_text":"No commercial use."`)

	// Payloads already carrying a policy snapshot keep it (sanitized); only
	// payloadWithCurrentReviewPolicy refreshes to the current policy.
	out, policy = payloadWithReviewPolicy(`{"request_snippet":"x","policy_text":"<p>Stored policy.</p>"}`)
	assert.Equal(t, "Stored policy.", policy)
	assert.Contains(t, out, "Stored policy.")
	assert.NotContains(t, out, "<p>")
}

func TestPayloadWithCurrentReviewPolicyRefreshesVersions(t *testing.T) {
	cfg := llmReviewSettingForTest(t)
	cfg.PolicyText = "Current policy."

	out, policy := payloadWithCurrentReviewPolicy(`{"request_snippet":"x","request_body":"{\"model\":\"gpt-4o\"}","request_headers":{"X-Trace":["ordinary"]},"policy_text":"stale"}`)
	assert.Equal(t, "Current policy.", policy)
	require.NotEmpty(t, out)
	for _, field := range []string{"policy_id", "policy_text", "prompt_version", "schema_version", "request_body", "request_headers"} {
		assert.Contains(t, out, field)
	}
}
