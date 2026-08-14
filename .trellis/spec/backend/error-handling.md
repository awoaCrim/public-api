# Backend error handling

Errors carry two separate concerns in this project: the internal Go error used for control flow/diagnostics, and the protocol-safe response exposed to the caller. Preserve both. Do not turn a useful sentinel or provider error into an untyped string merely to write a response.

## Layer responsibilities

- `model/` and `service/` return errors. They use sentinel errors for stable conditions (`model/errors.go`, `service` auth-session errors), `errors.Is`/`errors.As` for inspection, and `%w` when adding context.
- Controllers validate route/body input, call the application/model operation, and select the dashboard response helper. They should return immediately after writing an error.
- Relay handlers and middleware use `relaykit/types.NewAPIError` to preserve a protocol error code, HTTP status, retry behavior, and upstream error shape. `relaykit/types.NewAPIError.Unwrap` keeps `errors.Is`/`errors.As` working.
- Middleware owns authentication/distribution failures that occur before a controller. `middleware/utils.go:abortWithOpenAiMessage` writes the OpenAI-compatible error envelope and aborts the Gin chain.

A lower layer should not write to `*gin.Context`, except for an existing protocol-specific boundary that already owns the response stream. For example, a model function returns `model.ErrAuthFlowConsumed`; controllers such as `controller/telegram.go` decide how that condition is presented.

## Dashboard/API response envelope

The standard dashboard/API helpers are in `common/gin.go`:

```go
common.ApiSuccess(c, data) // HTTP 200, {"success":true,"message":"","data":...}
common.ApiError(c, err)    // HTTP 200, {"success":false,"message":err.Error()}
common.ApiErrorMsg(c, msg)
common.ApiErrorI18n(c, i18n.MsgInvalidParams)
common.ApiSuccessI18n(c, key, data)
```

Existing handlers such as `controller/channel.go` parse IDs or bind JSON, call the model, then use `common.ApiError`/`common.ApiErrorI18n` and return. Use `ApiErrorI18n` when the message is an existing backend translation key; do not invent a new translated key in a handler without adding it to `i18n/`.

The dashboard convention historically uses HTTP 200 with `success:false` for many business errors. Do not change that status contract casually. New auth-session endpoints are an explicit exception: `controller/auth_session.go:writeAuthSessionError` maps errors to HTTP status plus stable codes such as `AUTH_UNAUTHORIZED`, `AUTH_TOKEN_EXPIRED`, or `AUTH_INTERNAL_ERROR`, and logs only the internal-error case.

## Relay/OpenAI-compatible errors

Relay requests must retain the client protocol's error shape. Use the `relaykit/types` constructors rather than manually assembling provider errors:

```go
return types.NewError(
    err,
    types.ErrorCodeChannelModelMappedError,
    types.ErrOptionWithSkipRetry(),
)

return types.NewOpenAIError(
    err,
    types.ErrorCodeDoRequestFailed,
    http.StatusInternalServerError,
)

return types.NewErrorWithStatusCode(
    err,
    types.ErrorCodeInvalidRequest,
    http.StatusBadRequest,
)
```

Concrete examples are in `relay/alpha_search_handler.go`, `relay/chat_completions_via_responses.go`, `relay/helper/model_mapped.go`, and `controller/channel-test.go`. Choose `ErrOptionWithSkipRetry()` for errors that should not make channel distribution retry (invalid request, invalid conversion, bad channel configuration). Use a channel-prefixed error code for channel failures; `types.IsChannelError` and the distributor rely on that convention.

`types.NewAPIError` tracks the error code, type, status, optional upstream `OpenAIError`/`ClaudeError`, and retry/log options. Its `ToOpenAIError`/`ToClaudeError` paths mask sensitive message content. Keep provider response bodies and credentials out of user-visible errors; use the existing masking behavior instead of returning raw upstream payloads.

## Sentinel errors and wrapping

Define stable package-level errors when callers need to distinguish a condition. Existing examples include:

- `model.ErrDatabase`, `model.ErrInvalidCredentials`, `model.ErrEmailAlreadyTaken`, and `model.ErrAuthFlowConsumed` in `model/`.
- `model.ErrTwoFANotEnabled` and `model.ErrTwoFAAlreadyEnabled` for 2FA state transitions.
- `common.ErrRequestBodyTooLarge` and `common.IsRequestBodyTooLargeError` for request-body limits.

Wrap errors with `%w` when adding operation context:

```go
return fmt.Errorf("failed to migrate %s.%s to text: %w", tableName, columnName, err)
```

At the boundary, use `errors.Is`/`errors.As`, not string comparison. `controller/auth_session.go` checks `errors.Is(err, gorm.ErrRecordNotFound)` and service sentinel values; `model/auth_flow.go` checks `errors.Is(err, gorm.ErrRecordNotFound)` while consuming a flow.

## Validation and status selection

Validate before side effects. Bind/parse failures in controllers return a bad-parameter response (`common.ApiErrorI18n(c, i18n.MsgInvalidParams)` where the route uses i18n, or `common.ApiError` for a preserved error message). Relay request validation must return a 400 `NewAPIError` and `ErrOptionWithSkipRetry()` when the caller's request is invalid.

For system/resource failures, use the established status from the layer:

- Browser session errors use `service.AuthSessionErrorCode` and `writeAuthSessionError`.
- OpenAI-compatible middleware uses `abortWithOpenAiMessage` with the appropriate HTTP status and `types.ErrorCode`.
- Performance guard failures use `types.NewErrorWithStatusCode` in `middleware/performance.go` with 503 codes such as `system_cpu_overloaded`.
- A controller that serves a non-relay dashboard endpoint should generally preserve the `common.Api*` envelope rather than inventing an unrelated error JSON shape.

Do not expose database driver details, tokens, passwords, API keys, or complete upstream bodies. `model/gorm_logger.go` sanitizes driver errors in non-debug mode; relay errors mask sensitive information through `NewAPIError`.

## Logging failures while handling errors

An error response must remain possible even if error logging fails. Log internal details through `logger.LogError(c.Request.Context(), ...)`, `common.SysError`, or `common.SysLog` according to the logging rules; never replace the client response with a panic. `controller/auth_session.go` is the model example: clients receive a generic `AUTH_INTERNAL_ERROR`, while the request method/path and underlying error are sent to the request-aware logger.
