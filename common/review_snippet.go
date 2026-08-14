package common

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Review snippet extraction for LLM compliance review payloads.
//
// The snippet is the only request content that reaches the review pipeline,
// so it must be bounded, text-only and credential-free:
//   - rune-safe truncation (never splits a multi-byte character);
//   - string and structured content arrays, text parts only (image/audio/
//     file/base64 content is excluded);
//   - the most recent user messages are preferred and the last user message
//     is always retained within the caps;
//   - a bounded system prefix is kept for context;
//   - the final text is credential-masked.

const (
	reviewSnippetMaxPerMsg = 300 // max runes kept per message
	reviewSnippetMaxTotal  = 4000
	reviewSnippetMaxSystem = 200 // system prefix cap
	reviewSnippetMaxMsgs   = 20  // max messages retained
)

// ExtractLLMReviewSnippet builds a sanitized review snippet from a request
// body. Empty or non-chat bodies fail safe: chat bodies fall back to a
// bounded whitelist-redacted summary, everything else returns "".
func ExtractLLMReviewSnippet(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		// Non-chat body: keep only a bounded whitelist-redacted summary.
		trimmed := strings.TrimSpace(string(body))
		if trimmed == "" || trimmed == "{}" {
			return ""
		}
		redacted := RedactReviewJSON([]byte(trimmed))
		if redacted == "" || redacted == "{}" {
			return ""
		}
		return truncateRunes(redacted, 500)
	}

	results := messages.Array()
	type part struct {
		role string
		text string
	}

	// Keep one bounded system prefix for context.
	var systemPart *part
	for i := 0; i < len(results); i++ {
		if results[i].Get("role").String() != "system" {
			continue
		}
		if st := truncateRunes(extractMessageText(results[i]), reviewSnippetMaxSystem); st != "" {
			systemPart = &part{role: "system", text: st}
		}
		break
	}

	// Walk backwards so recent messages win the bounded window.
	var collected []part
	total := 0
	for i := len(results) - 1; i >= 0; i-- {
		if len(collected) >= reviewSnippetMaxMsgs {
			break
		}
		msg := results[i]
		role := msg.Get("role").String()
		if role == "system" {
			continue // handled above
		}
		text := extractMessageText(msg)
		if text == "" {
			continue
		}
		if total+utf8.RuneCountInString(text) > reviewSnippetMaxTotal {
			text = truncateRunes(text, reviewSnippetMaxTotal-total)
			if text == "" {
				break
			}
			total += utf8.RuneCountInString(text)
			collected = append(collected, part{role: role, text: text})
			break
		}
		total += utf8.RuneCountInString(text)
		collected = append(collected, part{role: role, text: text})
	}

	// Restore chronological order; system always leads.
	var parts []string
	if systemPart != nil {
		parts = append(parts, fmt.Sprintf("[%s] %s", systemPart.role, systemPart.text))
	}
	for i := len(collected) - 1; i >= 0; i-- {
		p := collected[i]
		parts = append(parts, fmt.Sprintf("[%s] %s", p.role, p.text))
	}
	if total >= reviewSnippetMaxTotal {
		parts = append(parts, "...[truncated]")
	}
	// Final credential pass over the joined text (covers Bearer/sk-*/base64
	// embedded inside message text).
	return MaskReviewCredentialText(MaskSensitiveInfo(strings.Join(parts, "\n")))
}

// extractMessageText extracts the text of one message: string content is used
// directly, structured content keeps only text parts (image_url/input_audio/
// file/video_url and other non-text blocks are excluded).
func extractMessageText(msg gjson.Result) string {
	content := msg.Get("content")
	if !content.Exists() {
		// reasoning_content compatibility.
		if rc := msg.Get("reasoning_content").String(); rc != "" {
			return rc
		}
		return ""
	}
	if content.Type == gjson.String {
		return content.String()
	}
	if content.IsArray() {
		var b strings.Builder
		content.ForEach(func(_, item gjson.Result) bool {
			if item.Get("type").String() == "text" {
				if t := item.Get("text").String(); t != "" {
					if b.Len() > 0 {
						b.WriteByte('\n')
					}
					b.WriteString(t)
				}
			}
			// Non-text blocks (images/audio/files/base64) are skipped.
			return true
		})
		return b.String()
	}
	return content.String()
}

// ExtractLLMReviewSnippetFromContext builds the sanitized review snippet from
// the request body storage. Storage failures return an empty snippet (fail
// safe, never blocking the trigger).
func ExtractLLMReviewSnippetFromContext(c *gin.Context) string {
	storage, err := GetBodyStorage(c)
	if err != nil {
		return ""
	}
	body, err := storage.Bytes()
	if err != nil {
		return ""
	}
	return ExtractLLMReviewSnippet(body)
}

// truncateRunes truncates to at most max runes without splitting a character.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "..."
}
