# Current-state research

## Confirmed root cause

- RPM review deduplication currently lasts only while a user's task is `pending` or `reviewing`. `LLMReviewGrace.ActiveTaskId` is released as soon as `CompleteLLMReviewTask` finishes, even when the RPM sliding window that produced the 429 is still active.
- A fast `compliant` verdict therefore allows another rejected request in the same RPM window to create another task and call the reviewer again.
- Each duplicate compliant task increments `LLMReviewGrace.CompliantCount`, so one RPM window can incorrectly contribute several compliant verdicts and open the configured long grace period early.
- Grace-period and unavailable-review branches currently persist one skipped task for every trigger, which can also flood the task list even though no reviewer call occurs.

## Relevant paths

- RPM limiter and trigger: `middleware/model-rate-limit.go`
- RPM trigger forwarding: `service/rate_limit_error.go`
- Shared enqueue policy: `service/llm_review_enqueue.go`
- Active-task slot and merge: `model/llm_review_enqueue.go`, `model/llm_review_task_db.go`
- Grace/task models: `model/llm_review_task.go`
- Verdict application: `service/llm_review_worker.go`
- Input-token preflight: `controller/token_preflight.go`
- Input/output-token postflight: `service/token_postflight_review.go`, `service/text_quota.go`
- Current summary/redaction helpers: `common/review_snippet.go`, `common/review_redact.go`

## Request-context gap

- `LLMReviewTrigger` currently carries only `RequestSnippet`; `LLMReviewPayload` therefore sends only a summary plus limit metadata.
- RPM uses `common.ExtractRequestSnippet`, which copies the first 2048 raw bytes without credential redaction.
- Token-limit paths use the safer `ExtractLLMReviewSnippetFromContext`, but still do not include a separately structured request-body record or request headers.
- Request Snapshot storage is not suitable as the reviewer source: it is optional, node-local, encrypted full-fidelity storage intended for Root-only inspection and may contain secrets. Reviewer context must be captured independently, bounded, and redacted before the asynchronous enqueue begins.

## Recommended behavior

1. Add a per-user RPM review-window marker separate from `ActiveTaskId`.
2. The first rejected request in an RPM window claims the marker and may create one pending or skipped task. Later rejected requests in that same window return from enqueue without creating, merging, or calling the reviewer.
3. Keep the marker through compliant, violation, uncertain, failed, skipped, or superseded task completion. A manual retry retries the selected task; later 429 requests do not create replacements.
4. Permit a new RPM review only after the stored window end has passed.
5. Keep the existing long compliant grace mechanism, but ensure one RPM window can increment the compliant counter at most once.
6. Every task that actually reaches the reviewer carries the first triggering request's bounded, redacted summary, body record, and headers. Merged/deduplicated events do not initiate a second reviewer call.

## Concurrency and database requirements

- Do not implement `SELECT then CREATE`; concurrent application instances can both win.
- RPM window claim must use a conditional update/CAS in the model layer and remain compatible with SQLite, MySQL, and PostgreSQL.
- Window claim, task persistence, active-slot binding, and selected task ID must not leave an indefinitely claimed window after an ordinary persistence error. Use one model-domain operation/transaction or an explicit compare-and-clear rollback path.
- `ActiveTaskId` continues to enforce one active reviewer task per user; the RPM window marker only selects one trigger from an RPM episode and must not block independent token-limit work after the task finishes.

## Request-context safety requirements

- Capture all `gin.Context` body/header data synchronously before starting a goroutine.
- JSON request records preserve useful ordinary structure while masking credential-like fields and omitting/replacing media/base64 blobs.
- Multipart records contain bounded text fields and file metadata only, never file bytes.
- Non-JSON bodies use a bounded printable/masked prefix.
- Headers keep names for diagnostic value but replace sensitive values (`Authorization`, cookies, API keys, tokens, signatures, WebSocket auth protocols, etc.) with `***`.
- Bound read size, output size, nesting, collection counts, string lengths, header count, and header values.
- Continue to use `common.Marshal`/`common.Unmarshal` rather than direct `encoding/json` business calls.

## API/UI impact

- The existing detail API already returns `review_payload`, and the current drawer renders it. Adding `request_body` and `request_headers` to `LLMReviewPayload` is backward compatible and does not require new body/header database columns.
- Persist RPM trigger window start/end on the task and expose them in task details so operators can verify why later events were deduplicated.
- Frontend changes are only required if these new window fields receive dedicated display; otherwise the payload additions are visible through the existing review payload section.
