# Design: usage-analysis default and LLM review reliability

## Change boundary

The smallest behavior gaps are at two independent cross-layer boundaries:

1. Usage analysis currently defaults to the all-user sentinel before the backend has identified the canonical Root user.
2. LLM review couples every provider to strict JSON Schema and allows stale enabled configuration to reach the queue path, while the policy source is only an optional manual setting.

The parent task coordinates two child implementations. The children should remain independently testable and should not share a new abstraction merely because they are delivered together.

## Child 1: usage-analysis data flow

```text
Root role in DB
  -> GET /api/usage-analysis/options: users + canonical root_user_id
  -> frontend resolves initial filter before enabling the analysis query
  -> GET /api/usage-analysis?user_id=<root id>
  -> existing model aggregation with UserID > 0
  -> Root user's displayed totals
```

### Boundary contract

- Extend the options response with a non-secret canonical `root_user_id` (zero/null when no enabled root can be resolved). The identity is selected by the Root role, not by matching a mutable display name.
- Keep the existing `users`, `tokens`, `models`, and `channels` shapes and keep the route under `RootAuth`.
- The frontend must not enable its analysis query until the options request has succeeded and the initial user selection has been resolved. This prevents a transient all-user request from being presented as the default.
- On a successful options response with a valid root ID, initialize both editable and applied filters to that ID. If the response contains no usable root ID, use the existing `All Users` sentinel and render an explicit unavailable-root notice; do not label the result Root.
- Subsequent user actions own the filter state. Refreshing/stale-updating options must not overwrite a selection the administrator has already changed.

### Expected files and ownership

- `controller/usage_analysis.go`: add the canonical root ID to the options response using the existing root-role convention and query context.
- `web/src/features/usage-analysis/api.ts`: type the new response metadata.
- `web/src/features/usage-analysis/index.tsx`: gate initialization/query execution, select Root, and render the safe fallback state.
- `web/src/features/usage-analysis/lib/usage-analysis.ts`: keep pure default/initialization helpers testable if needed.
- Existing usage-analysis tests plus a focused controller/options contract test cover the API and UI boundary.

## Child 2: LLM structured-output and policy flow

```text
settings + policy text
  -> capability test: strict_schema -> json_object -> prompt_json
  -> persisted output mode/readiness metadata
  -> enqueue/worker readiness gate
  -> request builder uses the tested mode
  -> response content extraction + deterministic JSON normalization
  -> mode-aware semantic validation
  -> task result (mode recorded, schema-valid flag recorded)
  -> auto-ban only when mode is strict_schema and all existing gates pass
```

### Output modes and compatibility

Introduce an internal persisted mode with three values:

- `strict_schema`: current `response_format.type=json_schema`, strict schema.
- `json_object`: `response_format.type=json_object` plus the explicit verdict contract in the prompt.
- `prompt_json`: no `response_format`, but the deterministic prompt still requires a single JSON object.

The capability test tries the modes in that order and stores the first mode that returns a parseable, semantically valid verdict. It falls back only for a controlled capability/request failure (typically HTTP 400) or a 200 response that cannot satisfy the current mode; authentication, endpoint, network, rate-limit, and server failures remain test failures rather than being hidden by fallback attempts. The production worker uses the selected mode; it does not silently mutate global configuration during a normal task.

Persist mode/readiness metadata in the existing option table through new setting keys, without a separate settings table or dialect-specific migration. Preserve `schema_tested` as the strict capability bit for backward compatibility. A legacy configuration with `schema_tested=true` and no new mode metadata is treated as strict mode. A compatibility pass sets structured-output readiness but leaves `schema_tested=false`.

The setting layer should expose one readiness predicate that requires:

- base URL and model;
- sanitized, non-empty administrator policy text;
- either a passing strict capability or a passing compatibility capability.

The controller uses that predicate when enabling. The enqueue path records a skipped/unavailable audit task instead of creating work that cannot run. The worker refuses to claim new tasks when the predicate is false, which repairs the observed enabled-but-untested production state. In-flight tasks re-check readiness before making a call.

### Response parsing and validation

Keep `ParseRawLLMResponse` as the provider-envelope extractor for string and text-part-array content. Add a separate normalization step used by capability tests and the worker that:

1. trims a BOM and surrounding whitespace;
2. accepts one fenced JSON object;
3. accepts one unambiguous JSON object surrounded by harmless prose;
4. rejects multiple objects, arrays, missing objects, or ambiguous text;
5. validates the normalized object with the existing verdict/category/confidence/evidence rules.

Strict mode retains the strict contract. Compatibility modes may tolerate non-semantic extra properties while still requiring every verdict field, supported vocabulary, bounded confidence, and non-empty evidence. Parse and validation failures remain uncertain/manual-review or audited task failures; they never become a violation.

### Auto-ban and audit boundary

- Add the selected output mode to the persisted task/detail contract so an administrator can distinguish strict from compatibility results.
- A semantically valid compatibility result may be stored as a normal compliant/violation/uncertain result for manual review, but `ShouldAutoBan` must additionally require the effective mode to be `strict_schema` (as well as the existing strict capability, confidence/category/evidence, root exemption, and manual-override checks).
- Malformed output keeps `SchemaPassed=false`; a fallback parser must not turn arbitrary prose into a trusted verdict.
- Preserve masked attempts/raw responses and existing retry/manual override behavior. HTTP 400 capability/configuration failures must not be retried as identical production calls.

### Policy/terms handling

The repository has no approved remote or built-in terms source. Keep `Policy Text` as the explicit source of truth:

- make its configured/unconfigured state part of the config/status API;
- require non-empty sanitized policy text before enabling or processing new work;
- allow explicit clearing only while the service is disabled, so an enabled service cannot silently lose its governing policy;
- retain the existing no-policy uncertain/manual-review path for legacy/directly processed tasks, but replace the ambiguous evidence wording with an actionable missing-policy explanation;
- never claim that terms were fetched when they were not.

### Frontend settings boundary

Update the settings UI to describe "structured output" rather than implying every model must pass strict Schema. Show strict support versus compatibility mode, show that compatibility mode cannot auto-ban, and show a clear policy-required warning. Add all new `t(...)` keys to all seven locales through the mandated i18n script and run the sync report.

## Compatibility, rollout, and rollback

- Existing strict configurations continue to use the current request shape and can auto-ban after the same safety gates.
- Existing `schema_tested=true` rows remain runnable as strict mode even before new metadata is written.
- Existing `enabled=true` plus no passing capability becomes idle/unavailable rather than issuing unsafe calls; the UI exposes the reason and the administrator can retest.
- New task columns are ordinary GORM model fields and must be included in the existing `AutoMigrate` model list; no raw dialect-specific DDL is required.
- Rollback of the feature code leaves new option keys/task mode values inert; old code can ignore unknown option keys, while task rows retain their existing core fields. Do not use destructive data migrations.

## Explicitly not changing

- Root-only authorization or the usage-analysis aggregation formula.
- The LLM review policy content, provider-specific arbitrary adapters, SSRF protections, sensitive-data redaction, manual overrides, or the existing strict auto-ban categories.
- Protected project identifiers or unrelated uncommitted user changes.
