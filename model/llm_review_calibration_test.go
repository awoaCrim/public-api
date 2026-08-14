package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordLLMReviewCalibrationSampleStoresErrorAndThreshold(t *testing.T) {
	truncateTables(t)
	require.NoError(t, RecordLLMReviewCalibrationSample("gpt-4o", 105, 100, 1000, "estimator-v1"))

	var sample LLMReviewCalibration
	require.NoError(t, DB.First(&sample).Error)
	assert.Equal(t, 105, sample.EstimateTokens)
	assert.Equal(t, 100, sample.ActualTokens)
	assert.InDelta(t, 0.05, sample.RelativeError, 1e-9)
	assert.Equal(t, 1000, sample.LimitValue)
	assert.False(t, sample.NearThreshold, "100 < 900 so the sample is not near threshold")

	// Near-threshold: actual >= limit*0.9.
	require.NoError(t, RecordLLMReviewCalibrationSample("gpt-4o", 2000, 950, 1000, "estimator-v1"))
	var near LLMReviewCalibration
	require.NoError(t, DB.Where("actual_tokens = ?", 950).First(&near).Error)
	assert.True(t, near.NearThreshold)

	require.NoError(t, RecordLLMReviewCalibrationSample("boundary-model", 0, common.MaxQuota, 0, "estimator-v1"))
	var boundary LLMReviewCalibration
	require.NoError(t, DB.Where("model_name = ?", "boundary-model").First(&boundary).Error)
	assert.InDelta(t, 1.0, boundary.RelativeError, 1e-9)
}

func TestRecordLLMReviewCalibrationSampleRejectsInvalidInput(t *testing.T) {
	truncateTables(t)
	maxInt := int(^uint(0) >> 1)
	type invalidSample struct {
		name     string
		model    string
		estimate int
		actual   int
		limit    int
	}
	tests := []invalidSample{
		{name: "missing model", estimate: 10, actual: 10},
		{name: "negative estimate", model: "gpt-4o", estimate: -1, actual: 10},
		{name: "zero actual", model: "gpt-4o", estimate: 10},
		{name: "oversized estimate", model: "gpt-4o", estimate: common.MaxQuota + 1, actual: 10},
		{name: "oversized actual", model: "gpt-4o", estimate: 10, actual: common.MaxQuota + 1},
		{name: "negative limit", model: "gpt-4o", estimate: 10, actual: 10, limit: -1},
		{name: "oversized limit", model: "gpt-4o", estimate: 10, actual: 10, limit: common.MaxQuota + 1},
	}
	if maxInt > common.MaxQuota {
		tests = append(tests, invalidSample{name: "max int estimate", model: "gpt-4o", estimate: maxInt, actual: 10})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, RecordLLMReviewCalibrationSample(test.model, test.estimate, test.actual, test.limit, ""))
		})
	}

	var count int64
	require.NoError(t, DB.Model(&LLMReviewCalibration{}).Count(&count).Error)
	assert.Zero(t, count, "invalid samples must be ignored")
}

// TestLLMReviewCalibrationAcceptanceCriteria pins the hard-limit acceptance
// contract: >=1000 samples, >=95% within 5% relative error, zero
// near-threshold false rejects.
func TestLLMReviewCalibrationAcceptanceCriteria(t *testing.T) {
	tests := []struct {
		name            string
		samples         int
		pass            int
		nearFalseReject int
		want            bool
	}{
		{"below minimum samples", 999, 950, 0, false},
		{"below pass rate", 1000, 949, 0, false},
		{"near threshold false reject", 1000, 980, 1, false},
		{"acceptance met", 1000, 980, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &LLMReviewCalibrationStatsResult{
				SampleCount:     tt.samples,
				PassCount:       tt.pass,
				NearFalseReject: tt.nearFalseReject,
			}
			assert.Equal(t, tt.want, llmReviewCalibrationPassed(stats))
		})
	}
}

// TestIsLLMReviewPreflightEligibleFromDatabase drives eligibility from real
// samples: 960 perfect samples plus 40 at 20% error meet acceptance; a single
// near-threshold false reject disqualifies the model again.
func TestIsLLMReviewPreflightEligibleFromDatabase(t *testing.T) {
	truncateTables(t)
	const model = "preflight-model"
	// 960 samples within tolerance (estimate == actual).
	for i := 0; i < 960; i++ {
		require.NoError(t, RecordLLMReviewCalibrationSample(model, 1000, 1000, 0, "estimator-v1"))
	}
	// 40 samples at 20% error (still above the 95% pass threshold).
	for i := 0; i < 40; i++ {
		require.NoError(t, RecordLLMReviewCalibrationSample(model, 1200, 1000, 0, "estimator-v1"))
	}
	assert.True(t, IsLLMReviewPreflightEligible(model, "estimator-v1"), "1000 samples with 96%% pass rate must qualify")

	// One near-threshold false reject: estimate exceeded the limit but the
	// actual input stayed within it.
	require.NoError(t, RecordLLMReviewCalibrationSample(model, 2100, 950, 1000, "estimator-v1"))
	assert.False(t, IsLLMReviewPreflightEligible(model, "estimator-v1"), "a near-threshold false reject must disqualify the model")

	// Other estimator versions are unaffected.
	assert.False(t, IsLLMReviewPreflightEligible(model, "estimator-v2"))
	assert.False(t, IsLLMReviewPreflightEligible("unknown-model", "estimator-v1"), "missing data must fail open (not eligible)")
}
