# Harden Usage and Calibration Bounds

## Goal

Correct F-008 and F-009 so invalid upstream cache metrics cannot distort billing and extreme token samples cannot corrupt estimator calibration.

## Requirements

- Cache-token fallback values from OpenAI-compatible response aliases/body fields must be accepted only when positive and within the project's durable quota/token bound.
- Invalid fallback values must not overwrite a valid canonical cached-token value.
- Billing arithmetic must not receive a negative cache-token value from these compatibility fallbacks.
- Calibration samples must reject out-of-bound estimate, actual, or limit values before persistence.
- Relative-error calculation must avoid integer subtraction overflow.
- Existing valid OpenCode Go, DeepSeek, Zhipu, Moonshot, and OpenAI cache mappings must remain unchanged.

## Out of Scope

- Repricing cache semantics or changing provider-specific valid field precedence.
- F-002 or broad normalization of unrelated provider formats.

## Acceptance Criteria

- [x] Negative and oversized cache fallback values are ignored/rejected and never reach billed cached tokens.
- [x] Existing valid canonical cached-token values are preserved.
- [x] Valid positive fallbacks still work for affected channels.
- [x] Extreme calibration samples are not persisted and cannot affect eligibility statistics.
- [x] Valid calibration relative errors remain correct.
- [x] Focused relay/service/model billing and calibration tests pass.
