package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEvaluateInputCalibration pins the acceptance contract for the pure
// predicate: minimum samples, required pass rate and zero near-threshold
// false rejects.
func TestEvaluateInputCalibration(t *testing.T) {
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
		{"defaults applied", 1000, 1000, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, EvaluateInputCalibration(tt.samples, tt.pass, tt.nearFalseReject, DefaultInputCalibrationConfig()))
		})
	}
}

// TestIsModelPreflightEligibleFailsOpenWithoutDatabase verifies that a
// missing database or missing calibration data reports ineligible, which the
// preflight caller treats as fail-open.
func TestIsModelPreflightEligibleFailsOpenWithoutDatabase(t *testing.T) {
	assert.False(t, IsModelPreflightEligible("model-without-calibration-data"))
	assert.Nil(t, GetInputCalibrationStats("model-without-calibration-data"))
}

// TestDefaultInputCalibrationConfigDocumentedDefaults pins the documented
// acceptance parameters.
func TestDefaultInputCalibrationConfigDocumentedDefaults(t *testing.T) {
	cfg := DefaultInputCalibrationConfig()
	assert.Equal(t, 1000, cfg.MinSamples)
	assert.InDelta(t, 0.95, cfg.RequiredPassRate, 1e-9)
	assert.InDelta(t, 0.05, cfg.MaxRelativeErr, 1e-9)
	assert.InDelta(t, 0.9, cfg.NearThresholdRatio, 1e-9)
}
