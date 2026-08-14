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

## Error logging at boundaries

Return a safe/generic client error while logging enough internal detail to diagnose the incident:

- `controller/auth_session.go:writeAuthSessionError` returns only `AUTH_INTERNAL_ERROR` and logs the method/path/error at `LogError` for 500s.
- `middleware/utils.go:abortWithOpenAiMessage` emits the protocol error, aborts the request, and records the message with the request context.
- `model/log.go` logs failures to persist audit/consume records rather than panicking or recursively trying to write another durable log.

Do not log the same expected client validation error at error level in every layer. Log unexpected infrastructure failures once at the boundary that can add request context.
