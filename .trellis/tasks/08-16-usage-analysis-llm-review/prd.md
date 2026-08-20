# Fix usage analysis and LLM review reliability

## Goal

Make the root-only usage-analysis page open on the Root user's data by default, and make the LLM review service usable across providers without weakening compliance or auto-ban safety.

## Background

- The usage-analysis route is already restricted to root users, but its current default user filter is `All Users`, so the first view aggregates every enabled user.
- The LLM review client currently sends one strict `json_schema` response format to every configured model. Many models reject that request, and the service can be left enabled while the latest schema capability test is false.
- Review tasks can be created without a usable policy text. In that state the worker intentionally produces an uncertain/manual-review result, but the setup problem is not sufficiently actionable for an administrator.

## Requirements

- Change the usage-analysis default to the system's Root user without changing the existing root-only authorization boundary or removing the user's ability to change filters.
- Provide a tested, user-visible behavior for LLM providers that do not support the current strict structured-output request. The selected behavior must avoid repeated retries of an unsupported request and must preserve fail-closed review semantics.
- Make response extraction and validation robust enough to handle the structured-content variants supported by the chosen compatibility behavior, while rejecting ambiguous or unsafe output.
- Keep automatic banning gated by a fully validated, trusted review result and an explicitly allowed capability/mode; compatibility handling must not silently turn malformed output into a ban.
- Make terms/policy availability explicit. The service must not claim that it retrieved terms when no usable policy text exists, and administrators must be able to understand and correct that setup state.
- Preserve existing task history, manual-review paths, configuration compatibility, internationalized UI behavior, and SQLite/MySQL/PostgreSQL support.

## Acceptance Criteria

- [ ] The usage-analysis page's settled initial query is scoped to the system's Root user, displays that selection, and still supports switching to another user or all users.
- [ ] Usage-analysis root-only authorization remains unchanged and regression-tested.
- [ ] A configured model that rejects strict structured output receives the selected compatibility behavior with clear capability/status diagnostics instead of an endless identical retry loop.
- [ ] Valid supported response variants are normalized and validated; malformed, ambiguous, or schema-invalid responses become an auditable uncertain/manual-review result and never trigger an automatic ban.
- [ ] The persisted configuration and worker agree on whether review processing is actually allowed; an enabled-but-untested/unsupported configuration cannot silently process as if strict capability had passed.
- [ ] Missing policy text produces a clear setup state and preserves the existing fail-closed/manual-review behavior; no false "terms retrieved" evidence is emitted.
- [ ] Relevant backend and frontend tests, type checks/builds, and focused regression tests pass.

## Task Map

- `08-16-usage-analysis-root-default`: default the usage-analysis view to the Root user.
- `08-16-llm-review-reliability`: harden structured output compatibility, validation, configuration gating, and policy availability handling.

## Confirmed Product Decision

- Non-strict models may be enabled through an explicit compatibility path. Compatibility-mode results remain manual-review-only and cannot trigger automatic bans.
- Policy text remains an explicit administrator-provided prerequisite. The repository has no approved alternate terms source, so missing policy must be surfaced clearly and handled fail-closed rather than fabricated or silently fetched.

## Scope Boundaries

- Do not broaden the usage-analysis page beyond its root-only administrative purpose.
- Do not invent provider-specific behavior or a new legal/compliance policy without an approved product decision and source text.
- Do not weaken validation, auditability, or manual-review safeguards merely to increase the apparent success rate.

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Complex child tasks require `design.md` and `implement.md` before activation.
