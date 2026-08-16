package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const reviewMaxPolicyTextRunes = operation_setting.MaxPolicyTextRunes

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

// sanitizeReviewPolicyText keeps the service's local vocabulary while
// sharing the exact readiness/payload normalization with the setting layer.
func sanitizeReviewPolicyText(raw string) string {
	return operation_setting.NormalizePolicyText(raw)
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
