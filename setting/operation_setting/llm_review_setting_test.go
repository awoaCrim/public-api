package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewReadinessRequiresPolicyAndCapability(t *testing.T) {
	base := LLMReviewSetting{
		BaseURL:              "https://review.example.com",
		ModelName:            "reviewer",
		PolicyText:           "No account sharing.",
		StructuredOutputMode: StructuredOutputModeStrictSchema,
		SchemaTested:         true,
	}

	ready := GetReviewReadiness(&base)
	require.True(t, ready.Ready)
	require.True(t, ready.StrictTrusted)

	missingPolicy := base
	missingPolicy.PolicyText = ""
	assert.False(t, GetReviewReadiness(&missingPolicy).Ready)

	presentationOnlyPolicy := base
	presentationOnlyPolicy.PolicyText = "<script>alert(1)</script>"
	assert.False(t, GetReviewReadiness(&presentationOnlyPolicy).Ready)

	untested := base
	untested.SchemaTested = false
	assert.False(t, GetReviewReadiness(&untested).Ready)

	compatibility := base
	compatibility.SchemaTested = false
	compatibility.StructuredOutputMode = StructuredOutputModeJSONObject
	compatibility.StructuredOutputTested = true
	compatibilityReadiness := GetReviewReadiness(&compatibility)
	assert.True(t, compatibilityReadiness.Ready)
	assert.False(t, compatibilityReadiness.StrictTrusted)
}

func TestMarkStructuredOutputTestedClearsStaleStrictTrust(t *testing.T) {
	original := reviewSetting
	t.Cleanup(func() { reviewSetting = original })

	reviewSetting.SchemaTested = true
	reviewSetting.SchemaTestedAt = 123
	reviewSetting.SchemaTestedModel = "strict-model"
	reviewSetting.SchemaVersion = "strict-v1"

	MarkStructuredOutputTested(StructuredOutputModePromptJSON, "compat-model", "compat-v1")

	assert.False(t, reviewSetting.SchemaTested)
	assert.Zero(t, reviewSetting.SchemaTestedAt)
	assert.Empty(t, reviewSetting.SchemaTestedModel)
	assert.Empty(t, reviewSetting.SchemaVersion)
	assert.True(t, reviewSetting.StructuredOutputTested)
	assert.Equal(t, StructuredOutputModePromptJSON, reviewSetting.StructuredOutputMode)
	assert.Equal(t, "compat-model", reviewSetting.StructuredOutputTestedModel)
}
