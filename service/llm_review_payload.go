package service

import (
	"errors"
	"math"
	"strconv"
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
	} else {
		normalized := false
		if confidence, ok := fields["confidence"].(string); ok {
			parsedConfidence, err := parseCompatibilityConfidence(confidence)
			if err != nil {
				return nil, false, "invalid verdict fields: " + err.Error()
			}
			fields["confidence"] = parsedConfidence
			normalized = true
		}
		if evidence, ok := fields["evidence"].(string); ok {
			if strings.TrimSpace(evidence) == "" {
				return nil, false, "invalid verdict fields: evidence must be a non-empty string"
			}
			// A single evidence sentence is unambiguous in compatibility mode;
			// preserve its original text as one audit evidence item.
			fields["evidence"] = []string{evidence}
			normalized = true
		}
		if normalized {
			// Re-encode through the project JSON helpers so the normal typed decode
			// remains the single source of truth for the rest of the verdict.
			normalizedData, err := common.Marshal(fields)
			if err != nil {
				return nil, false, "invalid verdict fields: " + err.Error()
			}
			data = normalizedData
		}
	}
	if fields["confidence"] == nil {
		return nil, false, "invalid verdict fields: confidence must be a number"
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

// parseCompatibilityConfidence accepts only a JSON-compatible decimal number
// written as a string. Providers using compatibility output occasionally
// quote this field, but values with whitespace, units, percentages, NaN, or
// infinity are intentionally rejected.
func parseCompatibilityConfidence(value string) (float64, error) {
	if !isDecimalNumber(value) {
		return 0, errors.New("confidence must be a pure numeric string")
	}
	confidence, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(confidence) || math.IsInf(confidence, 0) {
		return 0, errors.New("confidence must be a finite numeric string")
	}
	if confidence < 0 || confidence > 1 {
		return 0, errors.New("confidence must be in [0,1]")
	}
	return confidence, nil
}

// isDecimalNumber matches the JSON number grammar, avoiding strconv formats
// such as hexadecimal values and leading signs that are not unambiguous JSON
// numeric representations.
func isDecimalNumber(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	i := 0
	if value[i] == '-' {
		i++
		if i == len(value) {
			return false
		}
	}
	if value[i] == '0' {
		i++
		if i < len(value) && value[i] >= '0' && value[i] <= '9' {
			return false
		}
	} else if value[i] >= '1' && value[i] <= '9' {
		for i < len(value) && value[i] >= '0' && value[i] <= '9' {
			i++
		}
	} else {
		return false
	}
	if i < len(value) && value[i] == '.' {
		i++
		fractionStart := i
		for i < len(value) && value[i] >= '0' && value[i] <= '9' {
			i++
		}
		if i == fractionStart {
			return false
		}
	}
	if i < len(value) && (value[i] == 'e' || value[i] == 'E') {
		i++
		if i < len(value) && (value[i] == '+' || value[i] == '-') {
			i++
		}
		exponentStart := i
		for i < len(value) && value[i] >= '0' && value[i] <= '9' {
			i++
		}
		if i == exponentStart {
			return false
		}
	}
	return i == len(value)
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
	if normalized, isSSE, err := normalizeOpenAISSE(body); isSSE {
		if err != nil {
			return ReviewContentNormalization{}, err
		}
		return normalized, nil
	}

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
	text, err := llmContentToText(payload.Choices[0].Message.Content)
	if err != nil {
		return ReviewContentNormalization{}, err
	}
	return normalizeReviewJSONContent(text)
}

// normalizeOpenAISSE handles the explicit data-event form emitted by OpenAI
// compatible streaming endpoints. It deliberately does not try to interpret
// arbitrary text as an answer: a response is considered SSE only when it has
// SSE field syntax, and every data event must contain JSON.
func normalizeOpenAISSE(body []byte) (ReviewContentNormalization, bool, error) {
	lines := strings.Split(strings.TrimPrefix(string(body), "\ufeff"), "\n")
	seenSSEField := false
	var content strings.Builder

	for lineNumber, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, ":") {
			seenSSEField = true
			continue
		}
		if strings.HasPrefix(line, "data:") {
			seenSSEField = true
			data := strings.TrimPrefix(line, "data:")
			if strings.HasPrefix(data, " ") {
				data = data[1:]
			}
			data = strings.TrimSpace(data)
			if data == "" || data == "[DONE]" {
				continue
			}

			var event struct {
				Choices []struct {
					Delta struct {
						Content any `json:"content"`
					} `json:"delta"`
					Message struct {
						Content any `json:"content"`
					} `json:"message"`
				} `json:"choices"`
				Usage map[string]any `json:"usage"`
			}
			if err := common.Unmarshal([]byte(data), &event); err != nil {
				return ReviewContentNormalization{}, true, errors.New("SSE data event " + strconv.Itoa(lineNumber+1) + " is invalid JSON: " + err.Error())
			}
			if len(event.Choices) == 0 {
				if len(event.Usage) > 0 {
					continue
				}
				return ReviewContentNormalization{}, true, errors.New("SSE data event has no choices")
			}
			choice := event.Choices[0]
			chunk := choice.Delta.Content
			if chunk == nil {
				chunk = choice.Message.Content
			}
			if chunk == nil {
				continue
			}
			text, err := llmContentToText(chunk)
			if err != nil {
				return ReviewContentNormalization{}, true, errors.New("SSE data event has invalid content: " + err.Error())
			}
			content.WriteString(text)
			continue
		}
		if strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") || strings.HasPrefix(line, "retry:") {
			seenSSEField = true
			continue
		}
		if seenSSEField {
			return ReviewContentNormalization{}, true, errors.New("invalid SSE field")
		}
	}

	if !seenSSEField {
		return ReviewContentNormalization{}, false, nil
	}
	if strings.TrimSpace(content.String()) == "" {
		return ReviewContentNormalization{}, true, errors.New("SSE response contains no content")
	}
	normalized, err := normalizeReviewJSONContent(content.String())
	return normalized, true, err
}

func llmContentToText(content any) (string, error) {
	switch value := content.(type) {
	case string:
		return value, nil
	case []any:
		if len(value) == 0 {
			return "", errors.New("empty content parts")
		}
		var builder strings.Builder
		for _, item := range value {
			part, ok := item.(map[string]any)
			if !ok || part["type"] != "text" {
				return "", errors.New("unsupported or ambiguous content part")
			}
			partText, ok := part["text"].(string)
			if !ok || strings.TrimSpace(partText) == "" {
				return "", errors.New("empty text content part")
			}
			builder.WriteString(partText)
		}
		return builder.String(), nil
	default:
		return "", errors.New("unsupported content format")
	}
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
		RequestBody:    trigger.RequestBody,
		RequestHeaders: trigger.RequestHeaders,
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
