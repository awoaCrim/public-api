package operation_setting

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// LLMReviewSetting is the LLM compliance review configuration, registered
// under the "llm_review_setting" config block and persisted via the option
// table. Everything is disabled by default; enabling requires a configured
// base URL + model, policy text, and a passing structured-output capability test.
const (
	StructuredOutputModeStrictSchema = "strict_schema"
	StructuredOutputModeJSONObject   = "json_object"
	StructuredOutputModePromptJSON   = "prompt_json"
)

type LLMReviewSetting struct {
	// Enabled is the master switch. Disabled workers neither claim tasks nor
	// call the review LLM; new trigger events are recorded as skipped.
	Enabled bool `json:"enabled"`
	// BaseURL is the OpenAI-compatible review endpoint. It must pass SSRF
	// validation (http/https only).
	BaseURL string `json:"base_url"`
	// APIKeyEncrypted is the encrypted review API key (AES-256-GCM). The
	// config API only returns a mask derived from the decrypted tail.
	APIKeyEncrypted string `json:"api_key"`
	// ModelName is the reviewer model name.
	ModelName string `json:"model"`
	// PolicyText is the compliance policy text submitted with every review
	// payload. Empty policy forces uncertain verdicts instead of guessing.
	PolicyText string `json:"policy_text"`
	// TimeoutSeconds per-call timeout, default 30.
	TimeoutSeconds int `json:"timeout_seconds"`
	// MaxAttempts including the first call, default 3.
	MaxAttempts int `json:"max_attempts"`
	// RetryIntervalSeconds fixed retry interval, default 30.
	RetryIntervalSeconds int `json:"retry_interval_seconds"`
	// WorkerConcurrency in-flight review tasks, clamped to 1..20, default 2.
	WorkerConcurrency int `json:"worker_concurrency"`
	// AutoBanConfidence minimum violation confidence for auto permanent
	// disable, default 0.90.
	AutoBanConfidence float64 `json:"confidence_threshold"`
	// MaxCompliantCount compliant verdicts before a grace period, default 3.
	MaxCompliantCount int `json:"compliant_limit"`
	// GracePeriodHours grace window length, default 5.
	GracePeriodHours int `json:"immune_hours"`
	// LogRetentionDays task retention, default 90. Records tied to permanent
	// disables are never deleted.
	LogRetentionDays int `json:"retention_days"`
	// MaxOutputTokens reviewer max output tokens, default 1200.
	MaxOutputTokens int `json:"max_output_tokens"`
	// AllowPrivateAddress permits private address targets for the review
	// endpoint. Cloud metadata addresses stay blocked regardless.
	AllowPrivateAddress bool `json:"allow_private_url"`
	// SchemaTested marks a passing strict JSON schema capability test for the
	// current critical config (base URL/model/API key/private flag). It is
	// retained for backwards compatibility and remains strict-only.
	SchemaTested bool `json:"schema_tested"`
	// SchemaTestedAt timestamp of the last passing strict capability test.
	SchemaTestedAt int64 `json:"schema_tested_at"`
	// SchemaTestedModel reviewer model used in the last passing strict test.
	SchemaTestedModel string `json:"schema_tested_model"`
	// SchemaVersion of the strict capability test.
	SchemaVersion string `json:"schema_version"`
	// StructuredOutputMode is the explicitly tested request mode.
	StructuredOutputMode string `json:"structured_output_mode"`
	// StructuredOutputTested records a passing capability test for the selected mode.
	StructuredOutputTested      bool   `json:"structured_output_tested"`
	StructuredOutputTestedAt    int64  `json:"structured_output_tested_at"`
	StructuredOutputTestedModel string `json:"structured_output_tested_model"`
	StructuredOutputVersion     string `json:"structured_output_version"`
	// TestError masked last capability/connection test failure.
	TestError string `json:"test_error"`
}

// reviewSetting defaults: everything off.
var reviewSetting = LLMReviewSetting{
	Enabled:                false,
	TimeoutSeconds:         30,
	MaxAttempts:            3,
	RetryIntervalSeconds:   30,
	WorkerConcurrency:      2,
	AutoBanConfidence:      0.90,
	MaxCompliantCount:      3,
	GracePeriodHours:       5,
	LogRetentionDays:       90,
	MaxOutputTokens:        1200,
	AllowPrivateAddress:    false,
	StructuredOutputMode:   StructuredOutputModeStrictSchema,
	SchemaTested:           false,
	StructuredOutputTested: false,
}

func init() {
	config.GlobalConfig.Register("llm_review_setting", &reviewSetting)
}

// GetLLMReviewSetting returns the in-memory review settings.
func GetLLMReviewSetting() *LLMReviewSetting {
	return &reviewSetting
}

// IsLLMReviewEnabled reports whether the review master switch is on.
func IsLLMReviewEnabled() bool {
	return reviewSetting.Enabled
}

// SetLLMReviewEnabled updates the runtime switch (used by internal logic and
// tests).
func SetLLMReviewEnabled(enabled bool) {
	reviewSetting.Enabled = enabled
}

// ClampWorkerConcurrency clamps the worker concurrency to 1..20.
func ClampWorkerConcurrency(v int) int {
	if v < 1 {
		return 1
	}
	if v > 20 {
		return 20
	}
	return v
}

// IsValidMaxAttempts requires at least one attempt.
func IsValidMaxAttempts(v int) bool {
	return v >= 1
}

// IsReviewConfigured reports whether base URL and model are both present.
// An empty API key is allowed (some endpoints are anonymous).
func IsReviewConfigured(cfg *LLMReviewSetting) bool {
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.BaseURL) != "" && strings.TrimSpace(cfg.ModelName) != ""
}

// EffectiveStructuredOutputMode returns the explicitly selected mode. Legacy
// rows with SchemaTested=true and no new metadata are strict-schema rows.
func EffectiveStructuredOutputMode(cfg *LLMReviewSetting) string {
	if cfg == nil {
		return StructuredOutputModeStrictSchema
	}
	switch cfg.StructuredOutputMode {
	case StructuredOutputModeStrictSchema, StructuredOutputModeJSONObject, StructuredOutputModePromptJSON:
		return cfg.StructuredOutputMode
	default:
		if cfg.SchemaTested {
			return StructuredOutputModeStrictSchema
		}
		return StructuredOutputModeStrictSchema
	}
}

// IsPolicyConfigured reports whether administrator-provided policy text exists.
func IsPolicyConfigured(cfg *LLMReviewSetting) bool {
	return cfg != nil && NormalizePolicyText(cfg.PolicyText) != ""
}

// ReviewReadiness is the shared effective configuration gate used by the
// controller, enqueue path, worker, and in-flight task guard.
type ReviewReadiness struct {
	Ready            bool
	Reason           string
	Mode             string
	PolicyConfigured bool
	CapabilityTested bool
	StrictTrusted    bool
}

func GetReviewReadiness(cfg *LLMReviewSetting) ReviewReadiness {
	readiness := ReviewReadiness{Mode: EffectiveStructuredOutputMode(cfg)}
	if cfg == nil {
		readiness.Reason = "review configuration is missing"
		return readiness
	}
	readiness.PolicyConfigured = IsPolicyConfigured(cfg)
	readiness.CapabilityTested = cfg.StructuredOutputTested || (readiness.Mode == StructuredOutputModeStrictSchema && cfg.SchemaTested)
	readiness.StrictTrusted = readiness.Mode == StructuredOutputModeStrictSchema && (cfg.SchemaTested || cfg.StructuredOutputTested)
	if !IsReviewConfigured(cfg) {
		readiness.Reason = "base_url and model are required"
		return readiness
	}
	if !readiness.PolicyConfigured {
		readiness.Reason = "policy text is required"
		return readiness
	}
	if !readiness.CapabilityTested {
		readiness.Reason = "structured-output capability test is required"
		return readiness
	}
	readiness.Ready = true
	return readiness
}

// CanEnableReview requires a configured policy and a passing capability test
// for the selected mode. Compatibility modes are allowed but never trusted
// for automatic bans.
func CanEnableReview(cfg *LLMReviewSetting) bool {
	return GetReviewReadiness(cfg).Ready
}

// ResetSchemaCapability clears the capability state after critical config
// changes (base URL / model / API key / private flag).
func ResetSchemaCapability() {
	reviewSetting.SchemaTested = false
	reviewSetting.SchemaTestedAt = 0
	reviewSetting.SchemaTestedModel = ""
	reviewSetting.SchemaVersion = ""
	reviewSetting.StructuredOutputTested = false
	reviewSetting.StructuredOutputTestedAt = 0
	reviewSetting.StructuredOutputTestedModel = ""
	reviewSetting.StructuredOutputVersion = ""
	reviewSetting.TestError = ""
}

// SchemaTestStatus returns passed / failed / untested for the capability test.
// An untested state must never be reported as "unsupported".
func SchemaTestStatus(cfg *LLMReviewSetting) string {
	if cfg == nil {
		return "untested"
	}
	if cfg.StructuredOutputTested || cfg.SchemaTested {
		return "passed"
	}
	if cfg.TestError != "" {
		return "failed"
	}
	return "untested"
}

// MarkSchemaTested records a passing capability test.
func MarkSchemaTested(modelName, schemaVersion string) {
	reviewSetting.SchemaTested = true
	reviewSetting.SchemaTestedAt = time.Now().Unix()
	reviewSetting.SchemaTestedModel = modelName
	reviewSetting.SchemaVersion = schemaVersion
	reviewSetting.StructuredOutputMode = StructuredOutputModeStrictSchema
	reviewSetting.StructuredOutputTested = true
	reviewSetting.StructuredOutputTestedAt = reviewSetting.SchemaTestedAt
	reviewSetting.StructuredOutputTestedModel = modelName
	reviewSetting.StructuredOutputVersion = schemaVersion
	reviewSetting.TestError = ""
}

// MarkStructuredOutputTested records a passing capability test. Only strict
// mode updates the legacy SchemaTested flag.
func MarkStructuredOutputTested(mode, modelName, version string) {
	now := time.Now().Unix()
	reviewSetting.StructuredOutputMode = mode
	reviewSetting.StructuredOutputTested = true
	reviewSetting.StructuredOutputTestedAt = now
	reviewSetting.StructuredOutputTestedModel = modelName
	reviewSetting.StructuredOutputVersion = version
	reviewSetting.TestError = ""
	if mode == StructuredOutputModeStrictSchema {
		reviewSetting.SchemaTested = true
		reviewSetting.SchemaTestedAt = now
		reviewSetting.SchemaTestedModel = modelName
		reviewSetting.SchemaVersion = version
		return
	}
	// A compatibility pass must invalidate any legacy strict pass. Otherwise a
	// stale strict flag could accidentally make a fallback verdict look trusted.
	reviewSetting.SchemaTested = false
	reviewSetting.SchemaTestedAt = 0
	reviewSetting.SchemaTestedModel = ""
	reviewSetting.SchemaVersion = ""
}

// MarkReviewTestError records a masked capability/connection test failure.
func MarkReviewTestError(errMsg string) {
	reviewSetting.TestError = common.MaskReviewCredentialText(common.MaskSensitiveInfo(errMsg))
}

// MaskAPIKey derives a display mask from the decrypted plaintext tail. Callers
// must decrypt first; an encrypted value cannot be masked.
func MaskAPIKey(plain string) string {
	if plain == "" {
		return ""
	}
	if len(plain) <= 8 {
		return "****"
	}
	return plain[:3] + "****" + plain[len(plain)-4:]
}
