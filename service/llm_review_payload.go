package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// ReviewVerdictResponse is the strict JSON schema verdict structure.
type ReviewVerdictResponse struct {
	Verdict    string   `json:"verdict"`
	Category   string   `json:"category"`
	Confidence float64  `json:"confidence"`
	Reason     string   `json:"reason"`
	Evidence   []string `json:"evidence"`
}

// ValidateLLMReviewVerdict validates a parsed verdict against the strict
// schema: verdict/category vocabulary, confidence range, non-empty evidence.
func ValidateLLMReviewVerdict(data []byte) (*ReviewVerdictResponse, bool, string) {
	if len(data) == 0 {
		return nil, false, "empty response"
	}
	var resp ReviewVerdictResponse
	if err := common.Unmarshal(data, &resp); err != nil {
		return nil, false, "invalid json: " + err.Error()
	}
	switch resp.Verdict {
	case "violation", "compliant", "uncertain":
	default:
		return nil, false, "verdict must be violation|compliant|uncertain"
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
// evidence + strict schema passed + capability test passed for the current
// critical config. Root exemption is handled by the caller.
func ShouldAutoBan(resp *ReviewVerdictResponse, schemaPassed bool, cfg *operation_setting.LLMReviewSetting) bool {
	if resp == nil || !schemaPassed {
		return false
	}
	if cfg == nil || !cfg.SchemaTested {
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

// ParseRawLLMResponse extracts choices[0].message.content from an
// OpenAI-compatible response. String and structured-array (text parts)
// content are both supported.
func ParseRawLLMResponse(body []byte) (string, error) {
	var payload struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := common.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", errors.New("no choices in response")
	}
	content := payload.Choices[0].Message.Content
	switch v := content.(type) {
	case string:
		return v, nil
	case []any:
		var text string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if m["type"] == "text" {
					if t, ok := m["text"].(string); ok {
						text += t
					}
				}
			}
		}
		if text != "" {
			return text, nil
		}
	}
	return "", errors.New("unsupported content format")
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
