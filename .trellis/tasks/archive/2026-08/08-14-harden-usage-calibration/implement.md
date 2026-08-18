# Implementation Plan: Usage and Calibration Hardening

1. Add OpenAI usage tests for negative and `common.MaxQuota+1` cache aliases/body fallbacks across affected channel branches.
2. Add a bounded cache-fallback validator and apply it consistently without changing canonical-value precedence.
3. Add calibration tests for `MaxInt`/out-of-bound estimate, actual, and limit plus valid relative-error behavior.
4. Bound calibration samples and replace overflow-prone subtraction.
5. Run:
   - `go test ./relay/channel/openai ./service ./model -count=1`
   - relevant quota/billing tests if shared normalization is touched;
   - root `go build ./...`;
   - `gofmt` and `git diff --check`.

## Verification

- `go test ./relay/channel/openai -run 'TestApplyUsagePostProcessing|TestNormalizeAndSettle' -count=1`: pass.
- `go test ./model -run 'TestRecordLLMReviewCalibrationSample' -count=1`: pass.
- Focused input-calibration service tests: pass.
- Full `go test ./relay/channel/openai ./model -count=1`: pass.
- Full `go test ./service -count=1`: only the documented unrelated channel-affinity shared-state test failed (`expected 1`, observed accumulated `3`).
- `go build ./...`: pass.
- `gofmt` and `git diff --check`: pass.

## Rollback

Cache fallback and calibration changes are independent and require no schema migration.
