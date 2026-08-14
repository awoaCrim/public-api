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
