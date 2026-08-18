# Implementation Plan: Remediate Review Findings

The parent is not an implementation target. Execute and verify the children in order.

1. Complete `08-14-fix-routing-access-migration`.
2. Complete `08-14-fix-vision-contracts`.
3. Complete `08-14-harden-usage-calibration`.
4. Complete `08-14-fix-frontend-quality`.
5. Run integration checks:
   - focused Go tests for `controller`, `middleware`, `model`, `service`, `service/vision`, and `relay/channel/openai`;
   - root `go build ./...` and appropriate full tests;
   - `cd relaykit && GOWORK=off go build ./...`;
   - `cd web && bun run typecheck`, `bun run build`, affected-file lint/format, and relevant tests;
   - `git diff --check`.
6. Confirm F-002 paths were not changed as part of remediation.
7. Update the parent review status with final validation and remaining baseline failures.

## Final integration verification

### Passing gates

- All four child-task focused regression suites pass.
- `go build ./...`: pass.
- `cd relaykit && GOWORK=off go build ./...`: pass.
- Frontend focused tests for the changed Vision/profile, LLM Review format, and user-form behavior: pass.
- `cd web && bun run typecheck`: pass.
- `cd web && bun run build`: pass.
- Targeted `oxlint` and `oxfmt --check` for the F-010 files: pass.
- `git diff --check`: pass (only line-ending warnings on unrelated existing files were observed).

### Full-suite classification

- Root `go test ./... -count=1 -timeout=20m` was rerun. Remaining failures are unrelated baselines:
  - `middleware`: RPM/OpenAI-compatible 429 test depends on the existing missing `llm_review_tasks` fixture.
  - `relay/channel`: two Windows HTTP/2 retry tests observe environment-specific socket-abort/forcibly-closed behavior.
  - `service`: channel-affinity usage cache tests observe accumulated shared global state.
- `cd web && bun test` was rerun: 124 passed; the remaining failures are the existing API-key visual expectations and Bun's nested `node:test describe()` runner incompatibility.
- Full frontend lint/format/copyright checks were rerun. Remaining errors are repository-wide baselines outside the F-010 files; all affected-file checks are green.

### Scope and exclusion audit

- F-002 request-snippet/payload files were compared against the pre-remediation snapshot and remain unchanged.
- `model/llm_review_task_db.go` changed only in the F-009 calibration path; no request-snippet/payload behavior changed.
- No locale files or protected project identifiers were changed by this remediation.
- No unrelated working-tree changes were reverted. The reviewed product scope was committed as `da678d51` and deployed to `ssh2` as `newapi-custom:20260814-remediated-da678d51`; no push or task archive has been performed.
- An isolated final review agent could not access the parent's uncommitted overlay; the parent session therefore completed the full-scope review directly against the actual working tree and task-scoped checks.

### Additional final-review findings (resolved)

The user approved expanding the routing migration child to address both follow-up findings:

1. Active legacy grants whose routing group cannot be mapped are now reported under `UnmappableGrants` and block strict migration before any grant/token/marker write. Expired orphan grants are ignored as non-authoritative.
2. Grant preview/status now compares the merged legacy grant with the current target row and reports only real creates or source/expiry updates. A successful idempotent migration now reports `PendingGrants=0` and `InSync=true` when no other blockers remain.

Focused service/controller tests, root build, RelayKit independent build, `gofmt`, and `git diff --check` pass. The full service package retains only the previously classified channel-affinity shared-state failures. Post-deployment health checks pass; startup diagnostics correctly blocked strict routing migration on unmappable legacy key `渠道1`.

### Spec update judgment

- No `.trellis/spec/` change is required for the completed fixes. They enforce existing authorization, database fail-closed, billing-boundary, RelayKit-independence, frontend quality, and test-contract rules already documented in `AGENTS.md` and `.trellis/spec/backend/quality-guidelines.md`; they do not introduce a new reusable convention.

## Review gates

- Each child must be checked independently before starting the next child.
- Do not silently broaden a child to unrelated baseline cleanup.
- Do not commit until the user explicitly approves a commit plan.
