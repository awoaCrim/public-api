package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLLMReviewVerdict(t *testing.T) {
	tests := []struct {
		name string
		in   string
		ok   bool
	}{
		{"valid violation", `{"verdict":"violation","category":"limit_bypass","confidence":0.95,"reason":"r","evidence":["e1"]}`, true},
		{"valid compliant", `{"verdict":"compliant","category":"none","confidence":0.9,"reason":"ok","evidence":["fine"]}`, true},
		{"empty", "", false},
		{"invalid json", `{`, false},
		{"bad verdict", `{"verdict":"maybe","category":"none","confidence":0.5,"reason":"r","evidence":["e"]}`, false},
		{"bad category", `{"verdict":"violation","category":"spam","confidence":0.9,"reason":"r","evidence":["e"]}`, false},
		{"confidence too high", `{"verdict":"violation","category":"none","confidence":1.5,"reason":"r","evidence":["e"]}`, false},
		{"confidence negative", `{"verdict":"violation","category":"none","confidence":-0.1,"reason":"r","evidence":["e"]}`, false},
		{"missing reason", `{"verdict":"violation","category":"none","confidence":0.9,"evidence":["e"]}`, false},
		{"missing evidence", `{"verdict":"violation","category":"none","confidence":0.9,"reason":"r","evidence":[]}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict, ok, msg := ValidateLLMReviewVerdict([]byte(tt.in))
			assert.Equal(t, tt.ok, ok, msg)
			if tt.ok {
				require.NotNil(t, verdict)
			}
		})
	}
}

func TestValidateLLMReviewVerdictCompatibilityFields(t *testing.T) {
	data := []byte(`{"verdict":"compliant","category":"none","confidence":0.9,"reason":"ok","evidence":["fine"],"provider_note":"extra"}`)

	_, compatibilityPassed, compatibilityErr := ValidateLLMReviewVerdict(data)
	assert.True(t, compatibilityPassed, compatibilityErr)
	_, strictPassed, strictErr := ValidateStrictLLMReviewVerdict(data)
	assert.False(t, strictPassed)
	assert.Contains(t, strictErr, "unknown field")
}

func TestValidateLLMReviewVerdictCompatibilityConfidenceString(t *testing.T) {
	data := []byte(`{"verdict":"violation","category":"limit_bypass","confidence":"0.92","reason":"r","evidence":["e"]}`)

	verdict, compatibilityPassed, compatibilityErr := ValidateLLMReviewVerdict(data)
	require.True(t, compatibilityPassed, compatibilityErr)
	require.NotNil(t, verdict)
	assert.Equal(t, 0.92, verdict.Confidence)

	_, strictPassed, strictErr := ValidateStrictLLMReviewVerdict(data)
	assert.False(t, strictPassed)
	assert.Contains(t, strictErr, "invalid verdict fields")
}

func TestValidateLLMReviewVerdictCompatibilityConfidenceStringValidation(t *testing.T) {
	cases := []struct {
		name       string
		confidence string
		wantOK     bool
	}{
		{name: "lower boundary", confidence: "0", wantOK: true},
		{name: "upper boundary", confidence: "1", wantOK: true},
		{name: "scientific notation", confidence: "9.2e-1", wantOK: true},
		{name: "percentage", confidence: "92%"},
		{name: "unit", confidence: "0.92 score"},
		{name: "empty", confidence: ""},
		{name: "whitespace", confidence: " 0.92"},
		{name: "nan", confidence: "NaN"},
		{name: "infinity", confidence: "Inf"},
		{name: "above upper boundary", confidence: "1.01"},
		{name: "below lower boundary", confidence: "-0.1"},
		{name: "ambiguous hexadecimal", confidence: "0x1p-2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(`{"verdict":"violation","category":"limit_bypass","confidence":"` + tc.confidence + `","reason":"r","evidence":["e"]}`)
			_, passed, errMsg := ValidateLLMReviewVerdict(data)
			assert.Equal(t, tc.wantOK, passed, errMsg)
		})
	}
}

func TestValidateLLMReviewVerdictCompatibilityNormalizesUncertainCategory(t *testing.T) {
	data := []byte(`{"verdict":"uncertain","category":"uncertain","confidence":0.2,"reason":"insufficient evidence","evidence":["probe"]}`)

	verdict, compatibilityPassed, compatibilityErr := ValidateLLMReviewVerdict(data)
	require.True(t, compatibilityPassed, compatibilityErr)
	require.NotNil(t, verdict)
	assert.Equal(t, "none", verdict.Category)

	_, strictPassed, strictErr := ValidateStrictLLMReviewVerdict(data)
	assert.False(t, strictPassed)
	assert.Contains(t, strictErr, "invalid category")
}

func TestShouldAutoBanGating(t *testing.T) {
	cfg := llmReviewSettingForTest(t)
	cfg.SchemaTested = true
	cfg.AutoBanConfidence = 0.9

	violation := &ReviewVerdictResponse{
		Verdict:    "violation",
		Category:   "account_sharing",
		Confidence: 0.95,
		Reason:     "shared account",
		Evidence:   []string{"multiple tokens from one account"},
	}

	assert.True(t, ShouldAutoBan(violation, true, cfg))
	assert.False(t, ShouldAutoBan(nil, true, cfg), "nil verdict must not ban")
	assert.False(t, ShouldAutoBan(violation, false, cfg), "schema failure must not ban")

	low := *violation
	low.Confidence = 0.8
	assert.False(t, ShouldAutoBan(&low, true, cfg), "below-threshold confidence must not ban")

	contentCategory := *violation
	contentCategory.Category = "code_generation"
	assert.False(t, ShouldAutoBan(&contentCategory, true, cfg), "content-semantic categories require human review")

	untested := llmReviewSettingForTest(t)
	untested.SchemaTested = false
	assert.False(t, ShouldAutoBan(violation, true, untested), "capability-test failure must not ban")

	compatibility := *cfg
	compatibility.SchemaTested = false
	compatibility.StructuredOutputMode = operation_setting.StructuredOutputModeJSONObject
	compatibility.StructuredOutputTested = true
	assert.False(t, ShouldAutoBanWithTrust(violation, true, &compatibility, operation_setting.StructuredOutputModeJSONObject, true), "compatibility results are manual-review only")
	currentCompatibility := *cfg
	currentCompatibility.StructuredOutputMode = operation_setting.StructuredOutputModeJSONObject
	assert.False(t, ShouldAutoBanWithTrust(violation, true, &currentCompatibility, operation_setting.StructuredOutputModeStrictSchema, true), "a stale strict task mode must not bypass the current compatibility mode")
	assert.False(t, ShouldAutoBanWithTrust(violation, true, cfg, operation_setting.StructuredOutputModeStrictSchema, false), "repaired output is never trusted for auto-ban")
}

func TestParseRawLLMResponse(t *testing.T) {
	stringContent := `{"choices":[{"message":{"content":"{\"verdict\":\"compliant\"}"}}]}`
	content, err := ParseRawLLMResponse([]byte(stringContent))
	require.NoError(t, err)
	assert.Equal(t, `{"verdict":"compliant"}`, content)

	arrayContent := `{"choices":[{"message":{"content":[{"type":"text","text":"{\"verdict\":\"com"},{"type":"text","text":"pliant\"}"}]}}]}`
	content, err = ParseRawLLMResponse([]byte(arrayContent))
	require.NoError(t, err)
	assert.Equal(t, `{"verdict":"compliant"}`, content)

	fencedContent, marshalErr := common.Marshal("```json\n{\"verdict\":\"compliant\"}\n```")
	require.NoError(t, marshalErr)
	fenced := `{"choices":[{"message":{"content":` + string(fencedContent) + `}}]}`
	content, err = ParseRawLLMResponse([]byte(fenced))
	require.NoError(t, err)
	assert.Equal(t, `{"verdict":"compliant"}`, content)

	prose := `{"choices":[{"message":{"content":"Here is the result: {\"verdict\":\"compliant\"}"}}]}`
	content, err = ParseRawLLMResponse([]byte(prose))
	require.NoError(t, err)
	assert.Equal(t, `{"verdict":"compliant"}`, content)

	_, err = ParseRawLLMResponse([]byte(`{"choices":[{"message":{"content":"first {\"verdict\":\"compliant\"} then {\"verdict\":\"uncertain\"}"}}]}`))
	require.Error(t, err)
	_, err = ParseRawLLMResponse([]byte(`{"choices":[]}`))
	require.Error(t, err)
	_, err = ParseRawLLMResponse([]byte(`{`))
	require.Error(t, err)
}

func TestBuildPayloadSnapshotSanitizedContract(t *testing.T) {
	originalSecret := common.CryptoSecret
	common.CryptoSecret = "review-payload-test-secret"
	t.Cleanup(func() { common.CryptoSecret = originalSecret })

	cfg := llmReviewSettingForTest(t)
	cfg.PolicyText = "Do not share accounts."

	snapshot := buildPayloadSnapshot(LLMReviewTrigger{
		UserId:         7,
		ModelName:      "gpt-4o",
		Endpoint:       "/v1/chat/completions",
		IsStream:       true,
		TriggerType:    LLMReviewTriggerRPM,
		Stage:          LLMReviewStagePreflight,
		CurrentValue:   12,
		LimitValue:     10,
		RequestSnippet: "sanitized snippet",
		ClientIP:       "203.0.113.7",
	}, cfg)
	require.NotEmpty(t, snapshot)

	var payload model.LLMReviewPayload
	require.NoError(t, common.Unmarshal([]byte(snapshot), &payload))
	assert.Equal(t, ReviewPolicyID, payload.PolicyID)
	assert.Equal(t, "Do not share accounts.", payload.PolicyText)
	assert.NotContains(t, snapshot, "203.0.113.7", "the raw client IP must never enter the payload")
	assert.NotEmpty(t, payload.ClientIPHash, "only the irreversible IP hash may be included")
	assert.Equal(t, model.LLMReviewTriggerRPM, payload.TriggerType)
	assert.Equal(t, model.LLMReviewStagePreflight, payload.Stage)
	assert.Equal(t, 12, payload.CurrentValue)
}

// llmReviewSettingForTest returns the live review settings with automatic
// restoration.
func llmReviewSettingForTest(t *testing.T) *operation_setting.LLMReviewSetting {
	t.Helper()
	cfg := operation_setting.GetLLMReviewSetting()
	original := *cfg
	t.Cleanup(func() { *cfg = original })
	return cfg
}
