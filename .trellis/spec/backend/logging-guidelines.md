# Backend logging guidelines

Backend logs have two distinct destinations: request-aware application diagnostics and durable user/admin audit/usage logs. Use the right one for the event; do not treat a database `Log` row as a replacement for an operational error log.

## Operational/application logs

Use `logger` for events associated with a request or relay operation:

```go
ctx := c.Request.Context()
logger.LogInfo(ctx, fmt.Sprintf("record consume log: userId=%d, params=%s", userID, common.GetJsonString(params)))
logger.LogWarn(ctx, fmt.Sprintf("..."))
logger.LogError(ctx, fmt.Sprintf("...: %v", err))
logger.LogDebug(ctx, "Client IP %s passed token restrictions", clientIP)
```

`logger/logger.go` provides `LogInfo`, `LogWarn`, `LogError`, and `LogDebug`. The helper extracts `common.RequestIdKey` from the context and writes the request ID (or `SYSTEM`) with the level and timestamp. `LogDebug` is a no-op unless `common.DebugEnabled` is true. Pass `c.Request.Context()` or the relay context so a log can be correlated with the request; use `nil` only for genuinely background/system events.

Use `common.SysLog` for startup, migration, cache, or other system messages that do not have a request context, and `common.SysError` for system failures:

- `main.go` uses `common.SysLog` for startup/shutdown and `common.SysError` for non-fatal initialization failures.
- `service/auth_cleanup.go` uses `common.SysError` when scheduled cleanup/counting fails.
- `model/channel.go` uses `common.SysLog` for cache/persistence maintenance failures.

`common.SysLog` and `common.SysError` write through Gin's default writers under `common.LogWriterMu`; do not bypass the writer with a new global output path. `logger.SetupLogger` rotates the configured log file and swaps both Gin writers under the same lock.

## Levels and message content

- **Info:** meaningful state transitions or durable workflow milestones, such as `oauth/registry.go` reporting loaded custom providers or `model/log.go` recording a consume operation.
- **Warn:** degraded behavior, suspicious-but-handled conditions, or policy rejection. `oauth/generic.go` logs an access-policy denial at warn; quota saturation warnings are request-correlated and audited.
- **Error:** an operation failed or a response cannot be trusted. Include the operation and stable identifiers needed to investigate.
- **Debug:** diagnostic request/provider details that are safe to emit only when debug logging is enabled. OAuth providers use it for endpoint/status and truncated response context.
- **Fatal:** startup cannot continue. `common.FatalLog` exits; use it only for unrecoverable initialization failures.

Messages should identify the operation and relevant IDs, as in `model/channel.go` (`channel_id`, status, error) and `service/auth_cleanup.go` (count/threshold/window). Avoid high-cardinality or full payload logging by default.

## Sensitive data and SQL logging

Never log API keys, passwords, access/refresh tokens, full authorization headers, cookies, or unbounded provider response bodies. Redact before logging. Existing examples include:

- `oauth/generic.go` truncates response bodies to 500 characters before debug logging.
- `controller/channel_upstream_update.go` removes a channel secret from an error message before returning/logging it.
- `relaykit/types.NewAPIError` masks sensitive information when converting provider errors for clients.

Use `logger.LogJson` only for debug diagnostics; it serializes through `common.Marshal` and returns without output when debug is disabled. Do not use a log-only assertion in tests.

GORM uses `model/gorm_logger.go` with `logger.Warn`, a configurable `SQL_SLOW_THRESHOLD_MS` (default 200 ms), and parameterized queries outside debug mode. `sanitizedLogWriter` collapses MySQL/PostgreSQL/ClickHouse/SQLite driver errors to safe error codes when debug is disabled. Preserve this configuration rather than adding a second SQL logger or printing raw SQL/driver errors.

## Durable audit and usage logs

`model/log.go` owns durable records in `LOG_DB`:

- `RecordLog` writes system/manage/topup/etc. entries and respects `common.LogConsumeEnabled` for consume logs.
- `RecordLoginLog` writes `LogTypeLogin` with structured `Other.op.action` and `Other.op.params`; keep this stable action/parameter shape rather than storing only localized natural-language text.
- `RecordErrorLog` and `RecordConsumeLog` write request/user/channel/model/token context and log failures through the operational logger.
- `RecordLogWithAdminInfo` stores admin-only detail under `Other.admin_info`.

`createLog` ensures a request ID exists before writing. When a field is admin-only, nest it under `admin_info`; `formatUserLogs` strips `admin_info` and `audit_info` for ordinary user views. This is the existing privacy boundary and should be preserved for new diagnostics, including quota-saturation markers.

Log type values in `model/log.go` are persisted data. The source explicitly says not to use `iota` because changing numeric values would change historical meaning. Additions must preserve existing values.

The log database can be SQLite/MySQL/PostgreSQL or ClickHouse. `model/log.go` and `model/main.go` already branch ClickHouse ordering, filtering, display IDs, DDL, and TTL handling. New log queries must use `LOG_DB` and account for ClickHouse behavior where the existing helper branches.

## Scenario: Root-only request snapshot reads

### 1. Scope / Trigger

This contract applies whenever `GET /api/log/:request_id/snapshot`, request-snapshot authorization, or the Usage Log request-body viewer changes. Snapshot bodies can contain credentials and personal data, so backend authorization, audit ordering, cache headers, and frontend visibility must move together.

### 2. Signatures

- API: `GET /api/log/:request_id/snapshot`
- Route middleware: `RootAuth()`, `CriticalRateLimit()`, `DisableCache()`
- Controller: `controller.GetRequestSnapshot(c *gin.Context)`
- Audit writer: `model.CreateRequestSnapshotAccess(*model.RequestSnapshotAccess)`
- Frontend loader: `getRequestSnapshot(requestId: string): Promise<RequestSnapshotResponse>`

### 3. Contracts

- Only roles accepted by `RootAuth` may reach the controller; request snapshot access is not a delegatable `authz` resource and does not use 2FA/Passkey security proofs.
- The body remains absent from usage-log list responses and is fetched only after the Root user clicks the viewer.
- A successful response contains `request_id`, `content_type`, `size`, and exact `content_base64` bytes.
- Before reading bytes, `RequestSnapshotAccess` storage must be available. Before returning successful content, the success audit row must be durable.
- Responses use `DisableCache` headers. The frontend keeps decrypted content only in component state, invalidates in-flight responses on close/row change/unmount, and rejects a payload whose `request_id` differs from the active row.

### 4. Validation & Error Matrix

- Missing/invalid dashboard credential -> authentication middleware rejection.
- Non-Root role -> `403 AUTH_INSUFFICIENT_PRIVILEGE`; controller and snapshot audit table are not reached.
- Audit storage unavailable -> `500 SNAPSHOT_AUDIT_FAILED`; no body bytes are returned.
- Missing/deleted/local-file-missing snapshot -> stable `SNAPSHOT_NOT_FOUND`, `SNAPSHOT_DELETED`, or `SNAPSHOT_MISSING` response and failed audit row.
- Snapshot owned by another node -> `409 SNAPSHOT_WRONG_NODE` with `owner_node` and failed audit row.
- Integrity failure -> `500 SNAPSHOT_CORRUPT` and failed audit row.

### 5. Good/Base/Bad Cases

- Good: Root clicks View Request Body, the endpoint reads and audits exact bytes, and the component clears them when the dialog closes.
- Base: Root requests an unknown ID, receives `SNAPSHOT_NOT_FOUND`, and the failed read is audited without exposing paths.
- Bad: adding `AdminAuth`, a delegatable permission, or a proof header on only one side; returning bytes before the success audit; or rendering a late/mismatched payload after switching rows.

### 6. Tests Required

- Router regression: admin is rejected before the handler; Root reaches the handler without a proof; no-cache headers are present.
- Controller regression: exact byte round-trip, stable error codes, per-result audit rows, and success fail-closed when audit storage fails.
- Frontend gating regression: only Root roles with a request ID see the control; legacy permission maps do not grant visibility.
- Frontend lifecycle regression: direct click fetch, close/unmount/row-change invalidation, mismatched response-ID rejection, copy/download behavior, and localized error display.

### 7. Wrong vs Correct

#### Wrong

```go
logRoute.GET("/:request_id/snapshot", middleware.AdminAuth(), controller.GetRequestSnapshot)
```

This exposes the handler to ordinary administrators and omits the required rate-limit/no-cache boundary.

#### Correct

```go
logRoute.GET("/:request_id/snapshot",
    middleware.RootAuth(),
    middleware.CriticalRateLimit(),
    middleware.DisableCache(),
    controller.GetRequestSnapshot,
)
```

The frontend Root-role gate is a discoverability control only; `RootAuth` remains the authoritative security boundary.

## Error logging at boundaries

Return a safe/generic client error while logging enough internal detail to diagnose the incident:

- `controller/auth_session.go:writeAuthSessionError` returns only `AUTH_INTERNAL_ERROR` and logs the method/path/error at `LogError` for 500s.
- `middleware/utils.go:abortWithOpenAiMessage` emits the protocol error, aborts the request, and records the message with the request context.
- `model/log.go` logs failures to persist audit/consume records rather than panicking or recursively trying to write another durable log.

Do not log the same expected client validation error at error level in every layer. Log unexpected infrastructure failures once at the boundary that can add request context.
