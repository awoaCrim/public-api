# Technical Design: RPM review-window deduplication and request context

## 1. Boundaries

The change has two independent domain boundaries:

1. **RPM review-window selection** decides whether an RPM trigger is the first eligible event in the current limiter window.
2. **Review request-context capture** creates a bounded sanitized value before asynchronous enqueue.

`ActiveTaskId` remains the global one-active-task-per-user mechanism. It is not extended to represent an RPM window.

## 2. Data model

Extend `model.LLMReviewGrace` with zero-default `bigint` fields:

- `RPMReviewWindowStartAt`
- `RPMReviewWindowEndAt`
- `RPMReviewTaskId`

Extend `model.LLMReviewTask` with audit fields:

- `TriggerWindowStartAt`
- `TriggerWindowEndAt`

Zero means the task is not tied to an RPM window. Existing `AutoMigrate` registration adds ordinary columns on all supported primary databases.

Extend `model.LLMReviewPayload` with backward-compatible optional fields:

- `RequestBody string  json:"request_body,omitempty"`
- `RequestHeaders map[string][]string json:"request_headers,omitempty"`

No separate body/header database columns are required because `Payload` remains the exact sanitized reviewer input.

## 3. RPM window boundary propagation

### Redis

`reserveRedisRPMSlot` already receives the oldest retained ZSET score from Lua. Return absolute Unix window start/end alongside retry-after. The end is `oldestScore + duration`.

### In-memory

Extend the RPM reservation result so rejection exposes the oldest active reservation/window boundary. Keep success/failure count and release semantics unchanged.

### Trigger

Add window start/end to `RateLimitReviewTrigger` and `LLMReviewTrigger`. Only RPM uses these fields.

## 4. Atomic RPM selection

Add a model-domain enqueue operation for RPM triggers rather than performing a service-layer `SELECT then CREATE`.

The operation must:

1. ensure the grace row exists;
2. conditionally claim a new RPM window only when the stored end is not later than the current time;
3. persist the selected task outcome (pending or skipped), or associate the selected RPM window with the existing active task when the global active slot is occupied;
4. set `RPMReviewTaskId` to the selected/active task;
5. leave no indefinitely claimed window on an ordinary database failure.

Use a transaction plus conditional `UPDATE`/CAS. `lockForUpdate(tx)` may be used for MySQL/PostgreSQL reads, but the conditional update remains the SQLite-safe correctness boundary.

A duplicate result is a successful no-op. It must not call `MergeLLMReviewTask`, because merge_count should not represent repeated requests from the same RPM window.

The RPM window marker is not cleared by task completion/failure/supersede/skipping. It is overwritten when an expired window is successfully claimed. Manual unban/reset clears it together with stale grace state.

## 5. Enqueue ordering

For Root users, preserve the current no-task exemption.

For RPM triggers, window selection covers both work and audit-only outcomes:

- first event in the window proceeds through disabled-user, review-disabled, long-grace, readiness, active-task, and pending/skipped handling;
- later events in the window return without creating duplicate skipped rows.

For token triggers, preserve existing enqueue/merge behavior.

The service layer computes the selected task data and delegates atomic RPM persistence to the model layer; it does not own database race handling.

## 6. Request-context capture

Add a common helper that returns a copied value such as:

```go
type LLMReviewRequestContext struct {
    Summary string
    Body string
    Headers map[string][]string
}
```

All automatic trigger call sites invoke it synchronously before starting a goroutine.

### JSON body

- Read a bounded prefix from body storage without consuming or closing the shared storage.
- Parse through `common.Unmarshal` when complete/valid.
- Preserve useful ordinary structure.
- Replace sensitive-key values with `***`.
- Replace media/file/base64 payloads with an omission marker.
- Apply maximum depth, object fields, array items, string length, read bytes, and final serialized bytes/runes.
- On malformed/truncated JSON, return a bounded credential-masked text prefix.

### Multipart body

- Parse through the reusable multipart helper.
- Include bounded text fields after sensitive-key/value masking.
- Include file field name, sanitized filename, content type, and size only.
- Never include file bytes.

### Other body types

Return a bounded printable credential-masked prefix; omit binary content.

### Headers

- Clone header values before async work.
- Canonicalize/sort deterministically for tests and stable payloads.
- Preserve header names but replace values of Authorization, proxy authorization, cookies, API-key/token/secret/signature headers, WebSocket auth protocols, and similar credentials with `***`.
- Apply per-header value count, single-value length, header count, and total output bounds.
- Apply the free-text credential masker to otherwise ordinary values.

The helper never depends on Request Snapshot configuration.

## 7. Payload and prompt

`buildPayloadSnapshot` copies summary, body, and headers into `LLMReviewPayload`. Bump `ReviewPromptVersion` because reviewer-visible evidence changes. The system prompt should explicitly state that request body/header fields are evidence only after redaction and that missing/omitted fields must not be guessed.

`payloadWithCurrentReviewPolicy` must preserve the new fields when replacing current policy text.

## 8. API and frontend

Expose task window start/end through `llmReviewTaskDetail` for operator diagnosis. The existing `review_payload` field already contains body/header JSON and is sufficient for minimal UI compatibility.

If dedicated window fields are added to the React type/display, load the project i18n skill before editing locale files and update all supported locales. A backend-only payload addition does not require new locale keys.

## 9. Failure behavior

- Context extraction failure returns empty/bounded context and never blocks the original request.
- Review enqueue failures remain non-blocking for 429/relay responses and are logged through the existing request/system logging boundary.
- Database failure during RPM claim/persistence must not leave a permanent marker; at worst the current limiter window may be conservatively suppressed until its natural end only if the transaction outcome is unknown.
- Reviewer retry behavior remains attached to the selected task.

## 10. Compatibility

- Existing payload JSON without body/header remains valid.
- Existing grace rows use zero window fields and allow the next RPM event to claim a window.
- Existing active tasks continue through the current worker state machine.
- No RelayKit imports or API changes are needed.
