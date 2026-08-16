package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// ReviewVerdictResponse is the normalized verdict structure shared by strict
// and compatibility output modes.
type ReviewVerdictResponse struct {
	Verdict    string   `json:"verdict"`
	Category   string   `json:"category"`
	Confidence float64  `json:"confidence"`
	Reason     string   `json:"reason"`
	Evidence   []string `json:"evidence"`
}

// ValidateLLMReviewVerdict validates the semantic verdict contract. It
// requires every verdict field but tolerates non-semantic extra fields so the
// compatibility modes can process otherwise valid provider responses.
func ValidateLLMReviewVerdict(data []byte) (*ReviewVerdictResponse, bool, string) {
	return validateLLMReviewVerdict(data, false)
}

// ValidateStrictLLMReviewVerdict validates the strict response contract and
// rejects undeclared top-level fields before a result can be trusted for an
// automatic ban.
func ValidateStrictLLMReviewVerdict(data []byte) (*ReviewVerdictResponse, bool, string) {
	return validateLLMReviewVerdict(data, true)
}

func validateLLMReviewVerdict(data []byte, rejectUnknownFields bool) (*ReviewVerdictResponse, bool, string) {
	if len(data) == 0 {
		return nil, false, "empty response"
	}
	var fields map[string]any
	if err := common.Unmarshal(data, &fields); err != nil {
		return nil, false, "invalid json: " + err.Error()
	}
	if fields == nil {
		return nil, false, "verdict must be a JSON object"
	}
	for _, field := range []string{"verdict", "category", "confidence", "reason", "evidence"} {
		if _, ok := fields[field]; !ok {
			return nil, false, field + " is required"
		}
	}
	if rejectUnknownFields {
		for field := range fields {
			switch field {
			case "verdict", "category", "confidence", "reason", "evidence":
			default:
				return nil, false, "unknown field: " + field
			}
		}
	}

	var resp ReviewVerdictResponse
	if err := common.Unmarshal(data, &resp); err != nil {
		return nil, false, "invalid verdict fields: " + err.Error()
	}
	switch resp.Verdict {
	case "violation", "compliant", "uncertain":
	default:
		return nil, false, "verdict must be violation|compliant|uncertain"
	}
	// Compatibility providers sometimes mirror the uncertain verdict into the
	// category field. Treat that unambiguous no-category alias as "none" only
	// outside strict mode; strict capability checks must still enforce the
	// declared JSON Schema exactly.
	if !rejectUnknownFields && resp.Verdict == "uncertain" && resp.Category == "uncertain" {
		resp.Category = string(model.LLMReviewCategoryNone)
	}
	if !IsValidReviewCategory(resp.Category) {
		return nil, false, "invalid category: " + resp.Category
	}
	if resp.Confidence < 0 || resp.Confidence > 1 {
		return nil, false, "confidence must be in [0,1]"
	}
	if len(resp.Evidence) == 0 {
		return nil, false, "evidence must be a non-empty array"
	}
	return &resp, true, ""
}

// ShouldAutoBan decides whether a verdict triggers an automatic permanent
// disable: violation + confidence >= threshold + auto-ban eligible category +
// evidence + a trusted strict-schema result from the current capability-tested
// config. Root exemption is handled by the caller.
func ShouldAutoBan(resp *ReviewVerdictResponse, schemaPassed bool, cfg *operation_setting.LLMReviewSetting) bool {
	return ShouldAutoBanWithTrust(resp, schemaPassed, cfg, operation_setting.StructuredOutputModeStrictSchema, true)
}

// ShouldAutoBanWithTrust is the final strict-only safety boundary. Compatibility
// modes and any repaired/fenced/prose response are always manual-review-only.
func ShouldAutoBanWithTrust(resp *ReviewVerdictResponse, schemaPassed bool, cfg *operation_setting.LLMReviewSetting, outputMode string, trustedRaw bool) bool {
	if resp == nil || !schemaPassed || !trustedRaw {
		return false
	}
	if cfg == nil ||
		!cfg.SchemaTested ||
		outputMode != operation_setting.StructuredOutputModeStrictSchema ||
		operation_setting.EffectiveStructuredOutputMode(cfg) != operation_setting.StructuredOutputModeStrictSchema {
		return false
	}
	if resp.Verdict != "violation" {
		return false
	}
	threshold := cfg.AutoBanConfidence
	if threshold <= 0 {
		threshold = ReviewDefaultAutoBanConfidence
	}
	if resp.Confidence < threshold {
		return false
	}
	if !IsAutoBanReviewCategory(resp.Category) {
		return false
	}
	if len(resp.Evidence) == 0 {
		return false
	}
	return true
}

// ReviewContentNormalization records whether recovery was needed. Repaired
// content remains usable for manual review but is never trusted for auto-ban.
type ReviewContentNormalization struct {
	Content  string
	Repaired bool
}

// ParseRawLLMResponse extracts and deterministically normalizes
// choices[0].message.content. String and an unambiguous array of text parts
// are supported; mixed/unknown parts are rejected.
func ParseRawLLMResponse(body []byte) (string, error) {
	result, err := NormalizeRawLLMResponse(body)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

func NormalizeRawLLMResponse(body []byte) (ReviewContentNormalization, error) {
	var payload struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := common.Unmarshal(body, &payload); err != nil {
		return ReviewContentNormalization{}, err
	}
	if len(payload.Choices) == 0 {
		return ReviewContentNormalization{}, errors.New("no choices in response")
	}
	content := payload.Choices[0].Message.Content
	var text string
	switch value := content.(type) {
	case string:
		text = value
	case []any:
		if len(value) == 0 {
			return ReviewContentNormalization{}, errors.New("empty content parts")
		}
		var builder strings.Builder
		for _, item := range value {
			part, ok := item.(map[string]any)
			if !ok || part["type"] != "text" {
				return ReviewContentNormalization{}, errors.New("unsupported or ambiguous content part")
			}
			partText, ok := part["text"].(string)
			if !ok || strings.TrimSpace(partText) == "" {
				return ReviewContentNormalization{}, errors.New("empty text content part")
			}
			builder.WriteString(partText)
		}
		text = builder.String()
	default:
		return ReviewContentNormalization{}, errors.New("unsupported content format")
	}
	return normalizeReviewJSONContent(text)
}

func normalizeReviewJSONContent(raw string) (ReviewContentNormalization, error) {
	clean := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
	if clean == "" {
		return ReviewContentNormalization{}, errors.New("empty response content")
	}
	if isJSONObjectContent(clean) {
		return ReviewContentNormalization{Content: clean}, nil
	}
	if strings.HasPrefix(clean, "```") {
		lines := strings.Split(clean, "\n")
		if len(lines) >= 3 && strings.HasPrefix(lines[0], "```") && lines[len(lines)-1] == "```" {
			candidate := strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
			if isJSONObjectContent(candidate) {
				return ReviewContentNormalization{Content: candidate, Repaired: true}, nil
			}
		}
	}
	candidate, found, ambiguous := extractSingleJSONObject(clean)
	if ambiguous {
		return ReviewContentNormalization{}, errors.New("ambiguous multiple JSON objects in response")
	}
	if !found {
		return ReviewContentNormalization{}, errors.New("response does not contain one JSON object")
	}
	if !isJSONObjectContent(candidate) {
		return ReviewContentNormalization{}, errors.New("embedded content is not a JSON object")
	}
	return ReviewContentNormalization{Content: candidate, Repaired: true}, nil
}

func isJSONObjectContent(content string) bool {
	var object map[string]any
	if err := common.Unmarshal([]byte(content), &object); err != nil {
		return false
	}
	return object != nil
}

// extractSingleJSONObject finds exactly one balanced JSON object while
// respecting quoted strings. It intentionally does not attempt to repair JSON.
func extractSingleJSONObject(text string) (string, bool, bool) {
	start := strings.IndexByte(text, '{')
	if start < 0 {
		return "", false, false
	}
	depth := 0
	inString := false
	escaped := false
	end := -1
	for i := start; i < len(text); i++ {
		ch := text[i]
		if inString {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
				i = len(text)
			}
		}
	}
	if end < 0 || depth != 0 {
		return "", false, false
	}
	candidate := strings.TrimSpace(text[start : end+1])
	remaining := text[:start] + text[end+1:]
	if strings.Contains(remaining, "{") || strings.Contains(remaining, "}") {
		return "", false, true
	}
	return candidate, true, false
}

// buildPayloadSnapshot serializes a trigger event into the sanitized payload
// snapshot submitted to the reviewer. It never includes usernames, emails,
// API keys, auth headers, cookies or raw IPs; the client IP appears only as
// an irreversible hash.
func buildPayloadSnapshot(trigger LLMReviewTrigger, cfg *operation_setting.LLMReviewSetting) string {
	policyText := ""
	if cfg != nil {
		policyText = sanitizeReviewPolicyText(cfg.PolicyText)
	}
	payload := model.LLMReviewPayload{
		PolicyID:       ReviewPolicyID,
		PolicyText:     policyText,
		PromptVersion:  ReviewPromptVersion,
		SchemaVersion:  ReviewSchemaVersion,
		RequestSnippet: trigger.RequestSnippet,
		TriggerType:    model.LLMReviewTriggerType(trigger.TriggerType),
		Stage:          model.LLMReviewStage(trigger.Stage),
		ModelName:      trigger.ModelName,
		Endpoint:       trigger.Endpoint,
		IsStream:       trigger.IsStream,
		CurrentValue:   trigger.CurrentValue,
		LimitValue:     trigger.LimitValue,
		EstimateInput:  trigger.EstimateInput,
		ActualInput:    trigger.ActualInput,
		ActualOutput:   trigger.ActualOutput,
		ClientIPHash:   common.HashClientIP(trigger.ClientIP),
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}
