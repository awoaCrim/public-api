package service

import (
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"golang.org/x/net/html"
)

const reviewMaxPolicyTextRunes = 12000

var (
	markdownImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\([^\)]*\)`)
	markdownLinkPattern  = regexp.MustCompile(`\[([^\]]+)\]\([^\)]*\)`)
)

// currentReviewPolicyText returns the configured review policy text in plain
// form. The policy lives in llm_review_setting.policy_text so admins control
// exactly what the reviewer is allowed to enforce.
func currentReviewPolicyText() string {
	cfg := operation_setting.GetLLMReviewSetting()
	if cfg == nil {
		return ""
	}
	return sanitizeReviewPolicyText(cfg.PolicyText)
}

// sanitizeReviewPolicyText strips HTML/Markdown presentation wrappers and
// caps the length so rich-text markup, image links or oversized content never
// reach the reviewer context.
func sanitizeReviewPolicyText(raw string) string {
	text := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return ""
	}

	if strings.Contains(text, "<") && strings.Contains(text, ">") {
		if plain, ok := extractHTMLText(text); ok {
			text = plain
		}
	}
	text = markdownImagePattern.ReplaceAllString(text, "$1")
	text = markdownLinkPattern.ReplaceAllString(text, "$1")

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	lastBlank := true
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			if !lastBlank {
				cleaned = append(cleaned, "")
				lastBlank = true
			}
			continue
		}
		cleaned = append(cleaned, line)
		lastBlank = false
	}
	text = strings.TrimSpace(strings.Join(cleaned, "\n"))

	runes := []rune(text)
	if len(runes) > reviewMaxPolicyTextRunes {
		text = strings.TrimSpace(string(runes[:reviewMaxPolicyTextRunes])) + "\n[policy text truncated]"
	}
	return text
}

func extractHTMLText(raw string) (string, bool) {
	doc, err := html.Parse(strings.NewReader("<div>" + raw + "</div>"))
	if err != nil {
		return "", false
	}
	var builder strings.Builder
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, skipped bool) {
		if node.Type == html.ElementNode {
			switch strings.ToLower(node.Data) {
			case "script", "style", "noscript", "svg":
				skipped = true
			case "br", "p", "div", "section", "article", "header", "footer", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "blockquote", "tr":
				builder.WriteByte('\n')
			}
		}
		if node.Type == html.TextNode && !skipped {
			builder.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, skipped)
		}
		if node.Type == html.ElementNode && !skipped {
			switch strings.ToLower(node.Data) {
			case "p", "div", "section", "article", "header", "footer", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "blockquote", "tr":
				builder.WriteByte('\n')
			}
		}
	}
	walk(doc, false)
	return builder.String(), true
}

// payloadWithReviewPolicy ensures the payload contains the current policy
// text. Payloads carrying an older snapshot get the current policy; payloads
// missing the field get it backfilled.
func payloadWithReviewPolicy(payloadText string) (string, string) {
	var payload map[string]any
	if err := common.Unmarshal([]byte(payloadText), &payload); err != nil || payload == nil {
		policyText := currentReviewPolicyText()
		wrapped := map[string]any{
			"policy_text":        policyText,
			"legacy_payload_raw": payloadText,
		}
		if data, marshalErr := common.Marshal(wrapped); marshalErr == nil {
			return string(data), policyText
		}
		return payloadText, policyText
	}

	policyText, _ := payload["policy_text"].(string)
	policyText = sanitizeReviewPolicyText(policyText)
	if policyText == "" {
		policyText = currentReviewPolicyText()
	}
	payload["policy_text"] = policyText
	data, err := common.Marshal(payload)
	if err != nil {
		return payloadText, policyText
	}
	return string(data), policyText
}

// payloadWithCurrentReviewPolicy unconditionally refreshes the payload with
// the current policy before a reviewer call and persists it back, so policy
// edits take effect and the submitted payload stays traceable.
func payloadWithCurrentReviewPolicy(payloadText string) (string, string) {
	policyText := currentReviewPolicyText()
	var payload map[string]any
	if err := common.Unmarshal([]byte(payloadText), &payload); err != nil || payload == nil {
		payload = map[string]any{"legacy_payload_raw": payloadText}
	}
	payload["policy_id"] = ReviewPolicyID
	payload["policy_text"] = policyText
	payload["prompt_version"] = ReviewPromptVersion
	payload["schema_version"] = ReviewSchemaVersion
	data, err := common.Marshal(payload)
	if err != nil {
		return payloadText, policyText
	}
	return string(data), policyText
}

func reviewPayloadPolicyText(payloadText string) string {
	var payload map[string]any
	if err := common.Unmarshal([]byte(payloadText), &payload); err != nil || payload == nil {
		return ""
	}
	policyText, _ := payload["policy_text"].(string)
	return sanitizeReviewPolicyText(policyText)
}
