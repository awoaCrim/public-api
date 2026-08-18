# Design: Usage and Calibration Hardening

## Cache fallback validation

Keep provider precedence intact, but centralize validation of compatibility fallback counts at the OpenAI usage boundary. A fallback is valid only when `0 < value <= common.MaxQuota`. Invalid values behave as absent; they do not replace canonical values and do not become cache billing inputs.

Tests cover alias fields and response-body fields for DeepSeek/OpenCode Go plus preservation of valid standard `PromptTokensDetails.CachedTokens`.

## Calibration validation

At `RecordLLMReviewCalibrationSample`, require model name, non-negative bounded estimate, positive bounded actual, and a non-negative bounded limit. Use floating-domain subtraction (`math.Abs(float64(estimate)-float64(actual))`) after bounds validation so integer subtraction cannot overflow.

This protects both database compatibility and the preflight eligibility decision from absurd upstream usage samples.

## Audit behavior

Invalid values are ignored as invalid samples/fallbacks, matching existing fail-open calibration behavior. No new customer-visible error is introduced for a completed provider response.
