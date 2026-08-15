# Backend quality guidelines

Backend changes are expected to protect a real API contract, accounting invariant, security property, data-compatibility rule, or regression path. The repository does not treat raw coverage or a test that merely executes a function as sufficient quality.

## Readability and change scope

- Keep new Go code direct and readable: early returns and clear branches are preferred to deep nesting.
- Avoid package-level helpers that have one mechanical caller. Extract a function when it represents a reusable domain concept, a required framework callback/exported API, or complex logic with direct tests.
- Preserve protected project identifiers and existing public response/protocol names. Do not rename or remove references to `new-api` or `QuantumNous`.
- Follow the existing package boundary. A controller should not become a database abstraction, a provider adapter should not own billing persistence, and RelayKit must not import root application packages.
- Keep changes scoped. When a behavior changes, update the nearest contract/regression test instead of broad refactoring unrelated code.

## JSON and request DTO quality

New and maintained root-module business code is required to use the wrappers in `common/json.go`: `common.Marshal`, `common.Unmarshal`, `common.UnmarshalJsonStr`, `common.DecodeJson`, and `common.GetJsonType`. Concrete compliant paths include `relay/alpha_search_handler.go`, `relay/gemini_handler.go`, `model/task.go`, and `controller/channel_upstream_update.go`. The repository still contains legacy direct `encoding/json` call sites; do not copy those when adding or changing code. `json.RawMessage` can be referenced as a type, but actual serialization belongs behind the wrapper. RelayKit is a deliberate module boundary: it uses `relaykit/relayconvert/kitutil` rather than importing root `common`.

For client request DTOs that are decoded and re-marshaled to a provider, optional scalar values use pointers plus `omitempty` (`*int`, `*uint`, `*float64`, `*bool`) so omitted and explicit zero/false remain distinguishable. Existing boundary tests include `relaykit/dto/openai_request_zero_value_test.go` and `relay/helper/max_tokens_bounds_test.go`. Do not introduce a non-pointer scalar with `omitempty` for an optional provider parameter.

Use `common.UnmarshalBodyReusable` for request flows that need to inspect/parse the body more than once. It preserves a replayable body and supports disk-backed JSON decoding; `middleware/distributor.go` and `relay/common/relay_utils.go` use it for distributed relay/task parsing.

## Testing conventions

Tests should assert externally meaningful behavior, not implementation details. Prefer deterministic table tests with explicit business inputs and exact expected outputs. Do not add random fuzz/stress loops, sleeps, timing comparisons, log-only assertions, coverage-only smoke tests, or duplicate cases that protect the same branch.

New or substantially rewritten Go tests use:

- `github.com/stretchr/testify/require` for setup and fatal preconditions (`require.NoError`, `require.NotNil`, `require.Equal`).
- `github.com/stretchr/testify/assert` for non-fatal value checks (`assert.Equal`, `assert.Contains`, `assert.ErrorIs`).
- Arrange/Act/Assert structure and a test name that describes the trigger and expected behavior.

Concrete examples:

- `model/auth_flow_test.go` asserts that an OAuth flow is bound and consumed once, expires correctly, and rolls back together with the callback action.
- `model/locking_test.go` uses a dry-run GORM dummy dialector to verify `FOR UPDATE` for MySQL/PostgreSQL and its absence for SQLite.
- `controller/auth_flow_test.go` initializes an in-memory GORM database and asserts the actual HTTP response plus persisted flow state.
- `router/channel_router_test.go` verifies route registration and required permissions without testing Gin internals.
- `relay/helper/openai_image_request_test.go`, `relay/common/relay_utils_test.go`, and `common/quota_math_test.go` protect billing/request boundary invariants.

When a test uses a database, request context, user, settings, cache, or global flag, initialize and clean it explicitly. Existing fixtures such as `setupAuthFlowControllerTest` and `setupChannelStatusTest` restore global DB/database-type/cache state with `t.Cleanup`.

Tests should live beside the owning package and use the established names (`*_test.go`). Model/database tests belong in `model/`, HTTP route tests in `router/`, handler contract tests in `controller/`, and protocol conversion tests in `relaykit/relayconvert/` or the corresponding `relay/` package.

## Cross-layer contract: proof-free sensitive operations and data assets

### 1. Scope / Trigger

This contract applies when a sensitive dashboard operation no longer requires a secondary 2FA/Passkey proof, or when the embedded web router exposes files from the mounted data directory. The change must remove enforcement at the concrete route/controller caller, not weaken the shared proof verifier globally.

### 2. Signatures

- `POST /api/channel/:id/key` -> `controller.GetChannelKey`; route middleware remains `AdminAuth`, `RootAuth`, `CriticalRateLimit`, and `DisableCache`.
- `GET /api/llm_review/tasks/:id`, `POST /api/llm_review/tasks/:id/retry`, and `POST /api/llm_review/tasks` -> LLM Review handlers; the route group remains `AdminAuth` plus `authz.LLMReviewRead`.
- `POST /api/user/passkey/register/begin`, `POST /api/user/passkey/register/finish`, and `DELETE /api/user/passkey` -> Passkey account handlers; routes remain under `UserAuth`.
- `GET /data-assets/<path>` -> `serveDataAssets("/data-assets/", "/data")`; only approved image extensions are served.

### 3. Contracts

- The three formerly proof-gated operation families accept a missing or invalid `X-Security-Proof` header once ordinary route authentication/authorization succeeds. The optional proof issuer, verifier, `/api/verify`, and Passkey proof ceremonies remain available for explicit compatibility callers.
- Channel-key access remains Root-only and returns the stored key with the existing manage audit and no-store response headers.
- Passkey account management still requires an enabled user, a live browser session where flow binding/session rotation needs it, valid WebAuthn ceremony data, auth-version advancement, and security audit records.
- `/data-assets/` resolves against the process/container `/data` directory. Valid regular image files use standard `c.File` behavior and receive `Cache-Control: public, max-age=86400`. Invalid asset requests abort with HTTP 404 before the SPA fallback.

### 4. Validation & Error Matrix

| Condition | Expected result |
| --- | --- |
| Authorized Root channel-key request without proof | Existing successful key response; no proof error |
| Non-Root channel-key request | Existing Root authorization failure |
| LLM Review request without or with invalid proof | Normal handler validation/result; no `SECURITY_PROOF_*` error |
| Passkey account request without proof | Normal session/WebAuthn flow; no secondary-proof branch |
| Missing, directory, disallowed-extension, traversal, or outside-root/symlink asset | HTTP 404; never SPA HTML |
| Existing regular approved image | HTTP 200 with file bytes and image content type |
| Unrelated client-side route | Existing SPA fallback |

### 5. Good / Base / Bad Cases

- **Good:** a Root PAT or dashboard session reads `/api/channel/1/key` without a proof header; a browser registers a Passkey through the WebAuthn ceremony; `/data-assets/anon-removebg-preview.png` returns the mounted PNG.
- **Base:** an optional caller still issues and verifies a scoped proof through `/api/verify` or Passkey verification endpoints; the shared verifier continues enforcing identity, method, scope, and expiry.
- **Bad:** making `RequireSecurityProof` always return true, replacing `RootAuth` with `AdminAuth`, serving all of `/data`, or allowing a missing asset to fall through as HTTP 200 SPA HTML.

### 6. Tests Required

- Controller tests prove missing/arbitrary proof reaches the normal LLM Review and Passkey validation boundaries without consuming invalid flows prematurely.
- Router tests prove proof-free Root channel-key access, non-Root rejection, audit persistence, and preserved no-cache headers.
- Web-router tests prove valid PNG/HEAD behavior, missing/disallowed/directory/traversal/outside-root rejection, symlink confinement, and unrelated SPA fallback.
- Frontend API tests prove normal channel-key and Passkey account-management calls do not send `X-Security-Proof`; retain service/middleware tests for optional proof token binding.

### 7. Wrong vs Correct

#### Wrong

```go
// Weakens every current and future proof caller.
func RequireSecurityProof(c *gin.Context, scope string, methods []string) bool {
    return true
}
```

#### Correct

```go
// Remove only the proof middleware/call from the explicitly approved route or
// handler, while retaining ordinary authz and the optional verifier.
channelRoute.POST("/:id/key",
    middleware.RootAuth(),
    middleware.CriticalRateLimit(),
    middleware.DisableCache(),
    controller.GetChannelKey,
)
```

## Database compatibility checks

Every model or migration change must work with SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6. Use GORM where possible; for raw SQL use `common.UsingMainDatabase`/`common.UsingLogDatabase`, reserved-column helpers, boolean constants, and explicit dialect fallbacks. Exercise migrations as repeatable operations and include a regression test when the change concerns DDL or concurrent state.

For concurrent read-modify-write operations, use a transaction and `model/lockForUpdate` for standard row locks. SQLite requires an atomic update/compare-and-swap or SQLite-safe transaction behavior because it does not support `FOR UPDATE`; `model/user_session.go` and `model/locking.go` document the existing approach.

## Billing and quota safety

Billing changes require defense in depth across validation, estimation, pre-consume, settle/refund, and durable logs:

- Read `pkg/billingexpr/expr.md` before changing expression-based pricing.
- Bound user-controlled multipliers before quota calculation. Reuse `dto.MaxImageN`, `relaycommon.MaxTaskDurationSeconds`, and `relay/helper/valid_request.go:maxTokensLimit` rather than adding ad-hoc duplicates. Validate passthrough `Extra["parameters"]`, task metadata, multipart values, and unsigned fields too.
- Convert float/decimal quota through `common/quota_math.go`: `common.QuotaFromFloat`, `common.QuotaRound`, `common.QuotaFromDecimal`, or their `*Checked` variants. Never use an unbounded `int(float64(...))`, `int(math.Round(...))`, or `decimal.IntPart()` for a computed charge.
- Use `types.PriceData.AddOtherRatio`; do not write directly to `PriceData.OtherRatios`. It rejects non-positive, NaN, and +Inf multipliers.
- If a checked helper reports a clamp, retain it on `relayInfo.QuotaClamp` (or task settlement state), then call `attachQuotaSaturation` in `service/log_info_generate.go` before writing the consume/task log. The marker is nested under `other.admin_info.quota_saturation` and a request-correlated warning is logged.
- Ensure oversized values fail pre-consume with insufficient quota rather than wrapping into a negative charge. Test the boundary where the invariant is enforced, not just the helper's happy path.

## RelayKit independence

Any change under `relaykit/` must be valid without the root workspace or root module. The module has its own `go.mod`, DTOs, error types, converter registries, and dependency-free JSON/log helpers. It must not import `github.com/QuantumNous/new-api/common`, `model`, `service`, Gin, database drivers, or root-only settings.

Verify every RelayKit change with:

```text
cd relaykit && GOWORK=off go build ./...
```

Run focused tests such as `go test ./relayconvert/...` from `relaykit/` when conversion behavior changes. A successful root build does not verify this boundary.

## Verification checklist

Before reporting a backend change complete:

1. Run `gofmt` on changed Go files (documentation-only changes do not need it).
2. Run focused tests for changed packages and inspect the latest output.
3. Run the root checks appropriate to the change, commonly `go test ./...` or the repository's CI-equivalent command from `.github/workflows/ci.yml`.
4. For database/migration changes, verify all dialect branches and relevant migration tests.
5. For RelayKit changes, run the independent `GOWORK=off` build/test command above.
6. For documentation/spec changes, validate that all five backend spec files exist, contain no placeholder sections, and reference real paths/symbols from the repository.

## Cross-layer contract: configurable daily check-in balance threshold

### 1. Scope / Trigger

This contract applies when daily check-in rewards are limited by the user's current balance. It covers the `checkin_setting` options, the model mutation, the dashboard settings UI, and localized API errors.

### 2. Signatures

- `checkin_setting.balance_threshold_enabled` -> boolean switch, independent of `checkin_setting.enabled`.
- `checkin_setting.balance_threshold` -> finite positive `float64` entered in the active display unit.
- `model.UserCheckin(userID)` -> `(*Checkin, error)`; threshold rejection and authoritative balance-read failures use stable sentinel errors.
- `model.GetUserQuota(userID, true)` -> authoritative primary-database quota read; a missing user row is an error rather than an implicit zero balance.

### 3. Contracts

- The threshold value is stored unchanged in the display unit and converted at check-in time using the current `common.QuotaPerUnit` and display exchange rate. USD uses rate `1`, CNY uses `USDExchangeRate`, CUSTOM uses `CustomCurrencyExchangeRate`, and TOKENS is interpreted as USD.
- The effective threshold is a decimal raw-quota value; do not round it before comparing. Reject when `quota >= threshold`, and check this before creating the daily record, generating the reward, or updating quota.
- Invalid threshold options are rejected before database persistence and before publication to `common.OptionMap`. Malformed legacy values loaded at startup leave the safe positive default in place.
- A balance read error, including a missing user row, must fail closed and map to the generic localized database error. Do not expose the underlying database message.
- Existing daily uniqueness and the SQLite/manual-rollback or MySQL/PostgreSQL transaction paths remain unchanged.

### 4. Validation & Error Matrix

| Condition | Expected result |
| --- | --- |
| Threshold switch is false | Existing check-in behavior; no balance read |
| Threshold value is zero, negative, NaN, or infinite | Reject option update; do not mutate/publish the invalid value |
| Authoritative quota `<` effective threshold | Continue through existing duplicate/reward/accounting flow |
| Authoritative quota `>=` effective threshold | Stable threshold error; no check-in row or reward |
| Quota query fails or finds no user row | Stable balance-read error; generic localized API error; no side effect |
| TOKENS display mode | Treat the configured number as USD and label the UI fallback explicitly |

### 5. Good / Base / Bad Cases

- **Good:** read quota from the primary database, convert the current display-unit threshold with decimal arithmetic, reject equality, and preserve the saved numeric value when display rates change.
- **Base:** threshold enforcement is disabled and the existing daily check-in path is used unchanged.
- **Bad:** read Redis/cache quota for enforcement, treat a missing row as zero, round a fractional threshold to an integer, or write an invalid persisted option into `OptionMap` before validation.

### 6. Tests Required

- Operation-setting tests cover positive-only validation and USD/CNY/CUSTOM/TOKENS conversion with dynamic rates and fractional values.
- Model tests cover disabled bypass, below/equal/above/fractional boundaries, no side effects, duplicate uniqueness, reward/accounting behavior, missing-user/read failures, and invalid option publication/persistence.
- Controller tests assert localized threshold rejection and generic balance-read failure without leaking database details.
- Frontend tests assert positive decimal validation, TOKENS-to-USD fallback labeling, independent switch persistence, unchanged decimal payloads, and active-unit labels. Run typecheck, build, lint, format, i18n sync, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
quota, _ := model.GetUserQuota(userID, false) // cache may be stale; read errors are ignored
if quota < int(threshold) {
    grantCheckin()
}
```

#### Correct

```go
quota, err := model.GetUserQuota(userID, true)
if err != nil {
    return nil, ErrCheckinBalanceRead
}
if decimal.NewFromInt(int64(quota)).GreaterThanOrEqual(effectiveThreshold) {
    return nil, ErrCheckinBalanceThreshold
}
```
