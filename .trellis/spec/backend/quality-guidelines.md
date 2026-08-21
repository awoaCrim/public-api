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

LLM review request evidence must not parse a fixed-size prefix as if it were a complete JSON or multipart body. `common.CaptureLLMReviewRequestContext` may impose a separate bounded capture limit, but once that limit is exceeded it must skip parsing the partial bytes and emit an explicit safe omission marker. Valid payloads within the capture limit must retain their sanitized summary/body evidence, and capture must use independent body-storage readers so the downstream relay can replay the original request. Keep a regression test for a valid JSON request larger than the old capture boundary and for bodies exceeding the current capture limit.

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

## Cross-layer contract: Root-scoped usage analysis and fail-closed LLM review

### 1. Scope / Trigger

This contract applies when the root-only usage-analysis dashboard chooses its initial user scope or when the LLM compliance reviewer changes structured-output capability, policy readiness, response normalization, or automatic-ban behavior. Both flows cross the database/model, controller/service, API, and React UI boundaries.

### 2. Signatures

- `GET /api/usage-analysis/options` -> `controller.GetUsageAnalysisOptions`; response `data.root_user_id` is the first enabled `common.RoleRootUser` ID, or `0` when none exists.
- `GET /api/usage-analysis` -> `controller.GetUsageAnalysis`; the first UI query is issued only after options initialization and includes `user_id=<root_user_id>` when the ID is positive.
- `getInitialUsageAnalysisSelection(rootUserId, allValue)` -> pure frontend selection/fallback projection.
- `operation_setting.NormalizePolicyText(raw)` -> shared policy normalization used by readiness checks and reviewer payload construction.
- `operation_setting.GetReviewReadiness(*LLMReviewSetting)` -> shared readiness predicate for controller enablement, enqueue, worker claims, and in-flight processing.
- `ReviewClient.TestStructuredOutputCapability(ctx)` -> ordered strict-schema, JSON-object, then prompt-only JSON capability probe.
- `NormalizeRawLLMResponse(body)` / `ValidateLLMReviewVerdict(data)` -> provider-envelope normalization followed by semantic verdict validation.
- `ShouldAutoBanWithTrust(verdict, schemaPassed, cfg, outputMode, trustedRaw)` -> final strict-only automatic-ban gate.

### 3. Contracts

- Usage-analysis routes remain `RootAuth`-protected. The options response keeps existing users/tokens/models/channels and adds only the canonical root ID; the root option remains visible even if the bounded user list would otherwise omit it.
- The usage-analysis React query is disabled while options are loading or failed. On the first successful options response, editable and applied filters are initialized once to the positive root ID; later options refetches never overwrite manual user, token, model, channel, or range selections. A missing root uses the existing `all` sentinel only with a visible fallback warning.
- LLM readiness requires endpoint/model, administrator-provided sanitized non-empty policy text, and a passing selected structured-output capability. A legacy `schema_tested=true` row means strict-schema mode. Compatibility modes may process valid verdicts for audit/manual review but are never trusted for automatic bans.
- Capability fallback is attempted only for controlled structured-output rejection or invalid probe output. Authentication, transport, rate-limit, endpoint, and server failures are surfaced as failed capability state rather than hidden by retries of another mode.
- Accepted reviewer content is a string or an unambiguous array of text parts, optionally containing one fenced or embedded JSON object. Ambiguous, malformed, unsupported, or semantically invalid output is auditable failure/manual review and cannot ban. Strict validation requires all five verdict fields, rejects undeclared top-level fields, and only trusts a direct JSON object for strict capability; compatibility modes may tolerate non-semantic extra fields or repaired content but remain manual-review-only.
- `Policy Text` is explicit administrator input. Missing policy is not fetched or invented; legacy tasks complete as uncertain with actionable evidence, while newly triggered work on an unready service is skipped as `review_unavailable`.
- JSON serialization uses `common.Marshal`/`common.Unmarshal`; ordinary options/task persistence uses GORM and remains valid for SQLite, MySQL, and PostgreSQL.

### 4. Validation & Error Matrix

| Condition | Expected result |
| --- | --- |
| Options success with enabled Root | First analysis query contains the Root ID; filter displays the Root option. |
| Options success with no Root | Analysis may use `all`, but the UI visibly warns that Root could not be resolved. |
| Options request failure | Analysis query remains disabled; the options error is shown. |
| User selects All Users/another user | Existing sentinel/ID query behavior remains authoritative; no later options refetch resets it. |
| Review enabled without endpoint/model, policy, or capability pass | Configuration is rejected or new work is recorded as `review_unavailable`; worker makes no reviewer call. |
| Strict capability rejected but JSON-object/prompt mode passes | Persist the selected compatibility mode and allow manual-review processing; automatic bans remain impossible. |
| Auth/network/rate/server capability failure | Do not fall back silently; retain a failed/untested readiness state with a masked diagnostic. |
| Fenced/prose/multiple-object/invalid verdict | Normalize only if unambiguous; otherwise record parse/schema failure and never auto-ban. |
| Missing policy on a legacy claimed task | Complete as `uncertain` with setup guidance and no claimed violation. |

### 5. Good / Base / Bad Cases

- **Good:** options resolve Root ID 42, the first settled usage query includes `user_id=42`, and a later manual switch to `all` remains selected; a non-strict reviewer passes `json_object`, stores that mode, and only produces manual-review verdicts.
- **Base:** no Root exists, so the page explicitly shows All Users plus the fallback warning; an unready LLM task is skipped/audited rather than sent to an unsupported endpoint.
- **Bad:** start the aggregate query before options resolve, label an `all` result as Root, retry an authentication failure through compatibility modes, trust fenced compatibility output for auto-ban, or claim that terms were retrieved when `Policy Text` is empty.

### 6. Tests Required

- Controller options contract: enabled/disabled Root filtering, returned `root_user_id`, visible Root option, and unchanged existing option lists.
- Frontend helper/query-boundary tests: positive Root selection, missing-root fallback warning, first query ordering/scope, options-failure no-query behavior, and existing manual filter helper behavior.
- LLM client tests: request shape per mode, ordered fallback, and no fallback for authentication-like errors.
- Parser/worker tests: text-part arrays, fenced/prose normalization, ambiguity rejection, policy-missing evidence, output-mode persistence, and compatibility/repaired verdicts never auto-ban.
- Settings/controller tests: readiness prerequisites, strict/compatibility persistence, stale capability reset, and actionable status/error fields. Run focused Go tests plus frontend usage/review tests, typecheck, build, i18n sync, affected-file lint, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```tsx
const analysisQuery = useQuery({
  queryKey: ['usage-analysis', queryParams],
  queryFn: () => getUsageAnalysis(queryParams),
})
// queryParams initially omits user_id while /options is still loading.
```

This can present an all-user aggregate as the initial Root view.

#### Correct

```tsx
const analysisQuery = useQuery({
  queryKey: ['usage-analysis', queryParams],
  queryFn: () => getUsageAnalysis(queryParams),
  enabled: filtersInitialized,
})
```

The options response initializes both filter states before the first data request. For LLM review, the equivalent safety boundary is the explicit `ShouldAutoBanWithTrust` check requiring strict mode, current strict capability, semantic validation, and trusted raw content.

## Cross-layer contract: GitHub OAuth registration-age restriction

### 1. Scope / Trigger

This contract applies when GitHub OAuth creates a new local user or when the system settings UI changes the minimum age required for new GitHub registrations. Existing GitHub-linked login and authenticated OAuth account binding are not registration flows and must remain available.

### 2. Signatures

- `common.GitHubOAuthMinimumAgeYearsOptionKey` -> `GitHubOAuthMinimumAgeYears`.
- `common.ParseGitHubOAuthMinimumAgeYears(value string) (int, error)` -> accepts integer years in the inclusive range `0..100`.
- `common.GitHubOAuthMinimumAgeYears` -> runtime value; default is `common.DefaultGitHubOAuthMinimumAgeYears` (`1`), and `0` disables the restriction.
- `oauth.OAuthUser.CreatedAt *time.Time` -> optional provider account creation time.
- `controller.checkGitHubRegistrationAge(now time.Time, createdAt *time.Time, minimumAgeYears int) error` -> pure age policy check used only before creating a new local GitHub OAuth user.
- GitHub user retrieval parses the provider `created_at` value as RFC3339; malformed optional metadata is represented as unavailable metadata rather than a fabricated timestamp.

### 3. Contracts

- The system settings payload exposes `GitHubOAuthMinimumAgeYears` in the existing OAuth settings section. The UI accepts whole calendar years from `0` through `100`, displays `0` as disabled, and localizes the label and description in all supported frontend locales.
- The default is one calendar year. The comparison cutoff is `now.AddDate(-minimumAgeYears, 0, 0)`: an account created exactly at the cutoff is accepted, while a timestamp after the cutoff is rejected.
- The age check runs after existing numeric/legacy GitHub-ID lookup and registration-enable checks, but immediately before inserting a new local user. Existing linked GitHub users bypass the check.
- Authenticated OAuth bind flows bypass the registration-age check and retain their existing validation and update behavior.
- When the restriction is enabled and `created_at` is missing or malformed, new registration fails closed with the localized age-verification error. Existing login and bind flows remain usable.
- Invalid option values are rejected before persistence/publication. Startup normalization and runtime initialization restore the safe default when a legacy value is empty, malformed, negative, fractional, or above `100`.
- The OAuth callback must not create a local user before the age check succeeds, and must not map an all-purpose provider error to a misleading “too young” message when metadata is unavailable.

### 4. Validation & Error Matrix

| Condition | Expected result |
| --- | --- |
| Setting is `0` | Restriction disabled; a new GitHub account may register even when `created_at` is unavailable. |
| Setting is `1..100`, account is older than the cutoff | New local user is created normally. |
| Setting is `1..100`, account timestamp equals the cutoff | Registration is allowed. |
| Setting is `1..100`, account timestamp is newer than the cutoff | No local user is created; return the localized too-new error with the configured year count. |
| Setting is enabled, `created_at` is absent or malformed | No local user is created; return the localized unable-to-verify-age error. |
| Existing numeric/legacy GitHub-linked user logs in | Existing login behavior is preserved; age metadata is not required. |
| Authenticated user binds GitHub account | Existing bind behavior is preserved; age metadata is not required. |
| Option update is outside `0..100` or not an integer | Reject the update and keep the last valid persisted/runtime value; do not publish the invalid value. |

### 5. Good / Base / Bad Cases

- **Good:** parse GitHub `created_at` into `*time.Time`, compare it with `AddDate`, reject only a new registration that is too recent, and return a distinct localized error when the timestamp cannot be verified.
- **Base:** use the default one-year setting, preserve existing linked-user login and bind flows, and allow an administrator to set `0` to disable the restriction explicitly.
- **Bad:** compare only the mutable GitHub username, use a fixed 365-day duration instead of calendar years, treat missing metadata as an old account, apply the check to existing login/bind flows, or create the local user before validating age.

### 6. Tests Required

- `common/github_oauth_test.go` covers default, disabled, trimmed, maximum, negative, fractional, empty, malformed, and out-of-range option values.
- `oauth/github_test.go` covers valid, empty, and malformed GitHub `created_at` parsing without failing the general user-info request.
- `model/github_oauth_option_test.go` covers invalid-update rejection, safe fallback, and persistence of a valid value.
- `controller/github_oauth_registration_age_test.go` covers too-new rejection with no user creation, unavailable metadata fail-closed behavior, successful old-account creation, exact calendar cutoff/leap-year behavior, disabled mode, existing-user bypass, and bind-flow bypass.
- Frontend age-setting tests cover normalization boundaries; affected-file lint/format, i18n synchronization, type-check, and build must pass.

### 7. Wrong vs Correct

#### Wrong

```go
// A fixed duration is not equivalent to calendar years, and missing metadata
// would incorrectly allow an unverifiable new registration.
if createdAt == nil || time.Since(*createdAt) >= 365*24*time.Hour {
    createUser()
}
```

#### Correct

```go
if existingUser != nil {
    return existingUser, nil // preserve existing GitHub login
}
if err := checkGitHubRegistrationAge(time.Now(), oauthUser.CreatedAt, minimumAgeYears); err != nil {
    return nil, err // no local user is inserted on failure
}
return createNewOAuthUser(oauthUser)
```

## Cross-layer contract: user Vision interception threshold and Responses rewriting

### 1. Scope / Trigger

This contract applies when a user saves Vision interception settings or middleware replaces image inputs with descriptions for Chat Completions, Claude-style messages, or the OpenAI Responses API. Threshold validation belongs at every settings write boundary, while malformed legacy data is normalized again at runtime.

### 2. Signatures

- `service/vision.MinPhashThreshold` / `MaxPhashThreshold` -> inclusive domain `0..64`.
- `vision.ValidatePhashThreshold(int) error` -> rejects new values outside the domain.
- `vision.NormalizePhashThreshold(int) int` -> returns the value when valid and safe-disabled `0` for malformed legacy values.
- `PUT /api/user/setting/vision` and legacy `PUT /api/user/setting` -> both validate and normalize `settings.Vision` before persistence.
- `vision.ExtractImages(root)` / `vision.InterceptImages(...)` -> bounded extraction and all-or-nothing request-tree replacement.
- `/v1/responses` image part `{type:"input_image", image_url:...}` -> `{type:"input_text", text:<description>}`.

### 3. Contracts

- Values `1..64` enable perceptual-hash grouping. Values outside `0..64` are invalid API input and cannot replace stored user settings.
- Threshold `0` disables perceptual grouping: each image begins in a separate group and the grouping path performs no image download, decode, or pHash calculation. Exact-URL, request, and LRU caches remain available because they are cache identity, not perceptual clustering.
- Runtime middleware treats an invalid legacy threshold as `0`, preventing a value above the 64-bit Hamming-distance maximum from merging unrelated images.
- Blank prompts normalize to `vision.DefaultPromptTemplate`; nonblank custom prompts are preserved verbatim. Backend and frontend canonical defaults must remain byte-for-byte equivalent.
- Middleware does not publish a partially mutated request. Extraction or analysis failure restores/preserves the complete original body and continues fail-open.
- Successful Responses rewriting updates `Request.Body`, `common.KeyRequestBody`, and `vision_intercepted`, removes image-only fields, and preserves the client-requested `model`.

### 4. Validation & Error Matrix

| Condition | Expected result |
| --- | --- |
| New threshold `< 0` or `> 64` through either settings endpoint | Reject request; previously stored user settings remain unchanged. |
| Legacy runtime threshold outside `0..64` | Normalize to `0`; no perceptual grouping. |
| Threshold `0` with multiple images | Separate groups; no pHash decoder invocation; exact cache hits may still be reused. |
| Threshold `1..64` | Existing bounded pHash grouping and cache behavior. |
| Responses analysis succeeds | Replace `input_image` with `input_text`, update reusable body/context marker, preserve model. |
| Extraction or analysis fails after an in-memory partial replacement | Downstream receives the full original request and interception is not marked successful. |
| Blank prompt is saved | Persist/use the canonical default prompt. |

### 5. Good / Base / Bad Cases

- **Good:** validate both current and legacy write endpoints, normalize old invalid data to `0`, skip pHash entirely at `0`, and commit the rewritten request only after every image succeeds.
- **Base:** a valid positive threshold groups near-duplicate images and uses the existing isolated caches and billed Vision sub-call.
- **Bad:** trust the HTML `min`/`max` attributes, allow `65` to merge every 64-bit hash, compute pHash while claiming disabled, mutate `Request.Body` after only one image succeeds, or rewrite the original model to a Vision suffix/base.

### 6. Tests Required

- Controller tests cover `-1`, `0`, `64`, and `65` through both settings endpoints and assert invalid no-mutation behavior plus blank-prompt normalization.
- Service tests prove invalid legacy normalization, zero decoder calls at threshold `0`, separate zero-threshold groups, and positive-threshold clustering.
- Middleware tests exercise real `/v1/responses` success, `Request.Body` plus `KeyRequestBody`, `vision_intercepted`, model preservation, extraction fail-open, and partial-mutation analysis fail-open.
- Frontend component tests cover the canonical default, blank save, valid/invalid threshold submission, saving accessibility state, API failure, auth-store updates, and profile refresh.
- Run focused Go/frontend tests, typecheck, affected lint/format/copyright checks, build, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
// Values above 64 merge every valid pHash, and zero still hashes images.
groups := clusterImages(entries, config.PhashThreshold)
```

#### Correct

```go
threshold := NormalizePhashThreshold(config.PhashThreshold)
if threshold == 0 {
    return separateImageGroups(entries) // no download/decode/pHash
}
groups := clusterImages(entries, threshold)
```

## Cross-layer contract: RPM review-window deduplication and reviewer request evidence

### 1. Scope / Trigger

This contract applies when the dedicated per-user RPM limiter rejects a relay request or when RPM/input-token/output-token limits build the payload sent to the LLM compliance reviewer.

### 2. Signatures

- `common.InMemoryRateLimiter.ReserveWithWindow(...)` and `middleware.reserveRedisRPMSlot(...)` -> return the absolute limiter-window start/end for a rejected request.
- `model.EnqueueLLMReviewTaskForRPMWindow(userID, task, windowStart, windowEnd, now, skipReason)` -> atomically selects at most one event for an RPM window.
- `common.CaptureLLMReviewRequestContext(c)` -> synchronously returns copied `Summary`, bounded redacted `Body`, and bounded redacted `Headers`.
- `model.LLMReviewPayload` -> optional `request_body` and `request_headers` reviewer evidence.

### 3. Contracts

- `LLMReviewGrace.ActiveTaskId` limits active work; the separate RPM window marker limits which rejected request represents one limiter episode. Completing a task releases the active slot but does not release its RPM window.
- The first RPM event in a window may create one pending task, one skipped audit task, or merge once into an already-active task. Later events with the same window identity are successful no-ops: they create no task, skipped row, merge count, or reviewer call, including when asynchronous enqueue is delayed until after the window end.
- A newer window may be selected after the previous window expires. Window claim, task/audit persistence, active-slot binding, and selected task ID use a transaction plus conditional update/CAS that works on SQLite, MySQL, and PostgreSQL.
- One RPM window contributes at most one compliant result to the long grace counter. Manual user re-enable clears stale RPM window state.
- Automatic limit call sites capture request evidence before starting a goroutine. Request Snapshot storage is not a reviewer dependency.
- Request evidence is bounded and fail-safe. Credential-like body keys and headers are masked; media/base64/file bytes are omitted; multipart keeps bounded text fields and file metadata only. Capture failure never blocks the existing 429 or relay response.
- The existing OpenAI-compatible 429 headers/body, readiness gates, manual retry, and strict-only automatic-ban trust boundary remain unchanged.

### 4. Validation & Error Matrix

| Condition | Expected result |
| --- | --- |
| Concurrent RPM rejections with the same window | Exactly one selected event/task; all responses remain 429. |
| Same-window duplicate processed after window expiry | Still a duplicate because the stored window identity matches. |
| First event is review-disabled/unavailable/grace/disabled-user | One skipped audit row with the real reason; later same-window events create no rows. |
| Selected task completes while the limiter window is live | Active slot is released; same-window RPM events remain suppressed. |
| A later independent RPM window is rejected | It may select a new event, subject to the existing global active-task rule. |
| Authorization/Cookie/API-key/token/JWT/base64/file bytes appear in the request | Raw values are absent from task payload and reviewer request. |
| Body/header capture cannot parse the request | Empty or omission-marked bounded evidence; original request handling continues. |

### 5. Tests Required

- Model tests cover concurrent single-winner selection, completion followed by same-window suppression, delayed duplicate idempotence, skipped-audit deduplication, stale active-slot repair, compliant counting once, new-window selection, and SQLite migration defaults for existing rows.
- Middleware tests cover Redis and in-memory window boundaries plus unchanged 429 response and reservation-release behavior.
- Common/service/controller tests cover every automatic trigger path, body-storage preservation, JSON/multipart/header bounds, credential/media redaction, payload policy refresh, and reviewer payload fields.
- Run focused package tests, `go build ./...`, `go test ./...` when repository assets/dependencies are available, and `git diff --check`.

### 6. Wrong vs Correct

#### Wrong

```go
// Completion clears ActiveTaskId, so another 429 in the same limiter window
// can immediately create a second reviewer task.
active, created, _ := EnqueueLLMReviewTask(userID, task)
```

#### Correct

```go
// Window identity is claimed with the task/audit outcome; later identical
// events are idempotent even after the active task finishes.
_, selected, _, err := EnqueueLLMReviewTaskForRPMWindow(
    userID, task, windowStart, windowEnd, now, skipReason,
)
```
