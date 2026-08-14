package service

// LLM review fixed identifiers and defaults. Changing the prompt or schema
// contract bumps the corresponding version so payloads stay traceable.

const (
	// ReviewPolicyID identifies the policy carried in review payloads.
	ReviewPolicyID = "policy-v1"
	// ReviewPromptVersion is the reviewer prompt template version.
	ReviewPromptVersion = "prompt-v1"
	// ReviewSchemaVersion is the strict JSON schema version.
	ReviewSchemaVersion = "schema-v1"

	ReviewDefaultTimeoutSeconds    = 30
	ReviewDefaultMaxAttempts       = 3
	ReviewDefaultRetryIntervalSec  = 30
	ReviewDefaultWorkerConcurrency = 2
	ReviewDefaultAutoBanConfidence = 0.90
	ReviewDefaultMaxCompliantCount = 3
	ReviewDefaultGraceHours        = 5
	ReviewDefaultLogRetentionDays  = 90
	ReviewDefaultMaxOutputTokens   = 1200
)

// validReviewCategories is the verdict category vocabulary.
var validReviewCategories = map[string]struct{}{
	"commercial_use":       {},
	"account_sharing":      {},
	"unauthorized_client":  {},
	"stress_test":          {},
	"abnormal_automation":  {},
	"limit_bypass":         {},
	"harmful_resource_use": {},
	"code_generation":      {},
	"other":                {},
	"none":                 {},
}

// autoBanReviewCategories limits automatic permanent disables to categories
// directly evidenced by request behavior. Content-semantic and catch-all
// categories keep the violation record but require human review.
var autoBanReviewCategories = map[string]struct{}{
	"commercial_use":      {},
	"account_sharing":     {},
	"unauthorized_client": {},
	"stress_test":         {},
	"abnormal_automation": {},
	"limit_bypass":        {},
}

// IsAutoBanReviewCategory reports whether a category may trigger an automatic
// permanent disable.
func IsAutoBanReviewCategory(category string) bool {
	_, ok := autoBanReviewCategories[category]
	return ok
}

// IsValidReviewCategory reports whether a category is in the vocabulary.
func IsValidReviewCategory(category string) bool {
	_, ok := validReviewCategories[category]
	return ok
}
