# Implementation Plan: RPM review deduplication and request context

## 1. Build red-capable regression tests

- Add a model/service test that creates one RPM task, completes/releases it, then enqueues another trigger with the same window and expects no second task. Run it before the fix and confirm it fails.
- Add payload/context tests that expect sanitized `request_body` and `request_headers`; confirm they fail before implementation.
- Use deterministic fixed window timestamps and isolated SQLite fixtures; do not use sleeps.

## 2. Implement bounded request-context capture

- Add common types/helpers for synchronous summary/body/header extraction.
- Support JSON, multipart metadata/text fields, and safe fallback content.
- Add strict bounds and credential/media redaction.
- Replace RPM's raw `ExtractRequestSnippet` use with the unified helper.
- Update input-token preflight and token postflight call sites to capture and copy the same context before goroutines.
- Add focused common/middleware/controller/service tests proving body storage preservation and secret omission.

## 3. Propagate RPM window boundaries

- Extend Redis limiter result handling to expose absolute start/end.
- Extend the in-memory RPM reservation result with equivalent boundaries.
- Add trigger/task window fields and tests for retry-after/window calculations.
- Preserve 429 headers and reservation release behavior.

## 4. Add atomic RPM window selection

- Add grace/task model fields and a cross-database-safe model operation using transaction + conditional update/CAS.
- Ensure only the first trigger in a live window creates/selects a task.
- Ensure duplicates do not create skipped records or increment merge counters.
- Keep the marker after every terminal task status; clear/overwrite only on expiry/new claim or explicit user reset.
- Add concurrent and lifecycle regression tests.

## 5. Integrate service enqueue and payload

- Route RPM triggers through the new window-aware model operation while preserving Root/disabled/readiness/grace/manual safety gates.
- Keep token trigger behavior unchanged.
- Add body/header fields to `LLMReviewPayload`, preserve them during policy refresh, and bump the prompt version.
- Expose task window fields in the detail API.

## 6. Verify behavior

Run focused checks:

```bash
go test ./common -run "Test(MaskReview|RedactReview|ExtractLLMReview)"
go test ./middleware -run "TestRPM"
go test ./model -run "Test(EnqueueLLMReview|RecordLLMReviewCompliant|CheckLLMReviewGrace|.*RPM.*Window)"
go test ./service -run "Test(EnqueueLLMReview|BuildPayloadSnapshot|.*Postflight.*|.*ReviewWorker.*)"
go test ./controller -run "Test.*InputTokenPreflight"
```

Then run:

```bash
go test ./common ./middleware ./model ./service ./controller
go build ./...
git diff --check
```

Run `go test ./...` when focused checks are green. If frontend files change, also run the project i18n workflow and `cd web && bun run build`.

## 7. Independent quality review

- Dispatch `trellis-check` with the PRD/design/implementation context.
- Fix correctness, privacy, cross-database, and race findings.
- Re-run the red-capable tests and final validation.

## Rollback points

- Request-context helpers are additive and can be reverted independently before payload integration.
- RPM window columns are zero-default and backward compatible; reverting runtime use leaves harmless columns.
- Do not remove existing active-slot or long-grace mechanisms during rollback.
