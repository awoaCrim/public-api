package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// InputTokenEstimatorVersion identifies the current input-token estimator
// algorithm. Changing the algorithm requires bumping this version so model
// acceptance is re-evaluated from scratch.
const InputTokenEstimatorVersion = "estimator-v1"

// InputCalibrationConfig is the acceptance parameter set for the input-token
// estimator per model.
type InputCalibrationConfig struct {
	MinSamples         int     // minimum valid samples (default 1000)
	RequiredPassRate   float64 // required share within MaxRelativeErr (default 0.95)
	MaxRelativeErr     float64 // maximum relative error (default 0.05)
	NearThresholdRatio float64 // "near threshold": actual >= limit*ratio (default 0.9)
}

// DefaultInputCalibrationConfig returns the acceptance parameters.
func DefaultInputCalibrationConfig() InputCalibrationConfig {
	return InputCalibrationConfig{
		MinSamples:         1000,
		RequiredPassRate:   0.95,
		MaxRelativeErr:     0.05,
		NearThresholdRatio: 0.9,
	}
}

// EvaluateInputCalibration is the pure acceptance predicate (used by tests
// and observability). Runtime eligibility is delegated to
// model.IsLLMReviewPreflightEligible.
func EvaluateInputCalibration(sampleCount, passCount, nearFalseReject int, cfg InputCalibrationConfig) bool {
	if cfg.MinSamples <= 0 {
		cfg.MinSamples = 1000
	}
	if cfg.RequiredPassRate <= 0 {
		cfg.RequiredPassRate = 0.95
	}
	if sampleCount < cfg.MinSamples {
		return false
	}
	if sampleCount > 0 && float64(passCount)/float64(sampleCount) < cfg.RequiredPassRate {
		return false
	}
	if nearFalseReject > 0 {
		return false
	}
	return true
}

// RecordInputCalibrationSample asynchronously persists one (estimate, actual)
// input-token sample. Values must fit the durable quota range. The write never
// blocks the request hot path and failures only log.
func RecordInputCalibrationSample(modelName string, estimate, actual, limit int) {
	if !model.IsValidLLMReviewCalibrationSample(modelName, estimate, actual, limit) {
		return
	}
	common.RelayCtxGo(context.Background(), func() {
		if err := model.RecordLLMReviewCalibrationSample(modelName, estimate, actual, limit, InputTokenEstimatorVersion); err != nil {
			common.SysLog(fmt.Sprintf("RecordInputCalibrationSample failed: model=%s estimate=%d actual=%d: %v", modelName, estimate, actual, err))
		}
	})
}

// GetInputCalibrationStats returns per-model calibration statistics for admin
// and diagnostic use. It never participates in the hot path.
func GetInputCalibrationStats(modelName string) *model.LLMReviewCalibrationStatsResult {
	stats, err := model.GetLLMReviewCalibrationStats(modelName, InputTokenEstimatorVersion)
	if err != nil {
		return nil
	}
	return stats
}

// preflightEligibilityCacheTTL bounds how long a model acceptance decision is
// cached before the database is consulted again.
const preflightEligibilityCacheTTL = 60 * time.Second

var (
	preflightEligibilityCacheMu sync.Mutex
	preflightEligibilityCache   = map[string]*preflightEligibilityEntry{}
)

type preflightEligibilityEntry struct {
	eligible bool
	expireAt time.Time
}

// IsModelPreflightEligible reports whether the estimator passed acceptance for
// the model. Missing data or query errors return false and callers must fail
// open. Results are cached per model for 60 seconds; after expiry the
// database is authoritative again.
func IsModelPreflightEligible(modelName string) bool {
	now := time.Now()
	preflightEligibilityCacheMu.Lock()
	if entry, ok := preflightEligibilityCache[modelName]; ok && now.Before(entry.expireAt) {
		preflightEligibilityCacheMu.Unlock()
		return entry.eligible
	}
	preflightEligibilityCacheMu.Unlock()

	eligible := model.IsLLMReviewPreflightEligible(modelName, InputTokenEstimatorVersion)

	preflightEligibilityCacheMu.Lock()
	preflightEligibilityCache[modelName] = &preflightEligibilityEntry{
		eligible: eligible,
		expireAt: now.Add(preflightEligibilityCacheTTL),
	}
	preflightEligibilityCacheMu.Unlock()
	return eligible
}
