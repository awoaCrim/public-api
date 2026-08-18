# Implementation Plan: Remove Mandatory Proof Gates and Restore Data Assets

## Execution Rules

- Do not change product code until the user approves the completed planning summary and the task is moved from `planning` to `in_progress` with `task.py start`.
- Work only in `E:/myCode/public-api`; do not modify the read-only legacy repository.
- Preserve unrelated existing/untracked Trellis and agent runtime files.
- Never commit or push during implementation unless separately requested; production push/deployment is a later explicit operation.

## Phase A — Pre-Implementation Review Gate

1. Re-read `prd.md` top-to-bottom after the final convergence pass.
2. Re-read `design.md` and this checklist.
3. Confirm the resolved product boundary:
   - no role is forced to send a secondary 2FA/Passkey proof for the formerly gated operations;
   - 2FA/Passkey login, enrollment, deletion, and account-security features remain available;
   - ordinary authentication, authorization, browser-session requirements, WebAuthn ceremonies, rate limits, audits, and snapshot protections remain.
4. Run `python ./.trellis/scripts/task.py validate root-security-avatar` before activation.
5. After explicit approval, activate with `python ./.trellis/scripts/task.py start root-security-avatar`.
6. Load the implementation context through the applicable Trellis workflow (`trellis-before-dev` for inline mode, or the curated JSONL context for sub-agent mode).

## Phase B — Backend Security Gate Removal

1. Modify `router/channel-router.go`:
   - remove only `middleware.SecureVerificationRequired()` from `POST /:id/key`;
   - retain `AdminAuth`, `RootAuth`, `CriticalRateLimit`, `DisableCache`, and `controller.GetChannelKey` in their existing order.
2. Update `controller/channel.go` comments for `GetChannelKey` so they describe Root authorization/audit behavior without claiming a proof middleware dependency.
3. Modify `controller/llm_review.go`:
   - remove calls to `requireLLMReviewProof` from detail, retry, and create-task handlers;
   - remove or update proof-only helper/comment declarations only when no compatibility caller depends on them;
   - preserve authorization, validation, queue mutation, and audit behavior.
4. Update stale LLM Review route comments in `router/api-router.go` so they no longer state that proof is required.
5. Modify `controller/passkey.go`:
   - remove registration proof checks from begin/finish;
   - remove deletion proof enforcement while preserving an explicit existing-credential check and its current response contract;
   - preserve session identity retrieval for auth-flow binding and auth-version rotation;
   - retain all WebAuthn protocol validation, flow-token consumption, persistence, and security audit behavior.
6. Keep the optional proof implementation (`middleware/secure_verification.go`, `controller/secure_verification.go`, `service/auth_token.go`, and Passkey proof begin/finish) unless compilation and compatibility review show a safe, necessary cleanup. Do not turn the shared verifier into an unconditional allow.
7. Run `gofmt` on changed Go files immediately.

## Phase C — Frontend Forced-Flow Removal

1. Modify `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`:
   - remove secure-verification hook/dialog state and rendering;
   - call the existing channel-key API directly from the reveal action;
   - preserve loading, success/error toast, and key display behavior.
2. Modify `web/src/features/profile/components/passkey-card.tsx`:
   - remove secure-verification hook/dialog state, method selection, and proof-specific callbacks;
   - call the existing `register` and `remove` actions directly after browser support/confirmation checks;
   - preserve status refresh, loading, and the confirmation dialog.
3. Keep lower-level optional proof-token parameters only if they remain part of a compatibility surface; normal UI calls must not provide a token or open a proof dialog.
4. Search `web/src` for `startVerification` and `withVerification` to confirm no current product caller still forces these formerly gated actions.
5. If a user-facing text still says request snapshots require secure verification, update it using the frontend i18n skill and all supported locales; do not introduce untranslated text.
6. Run targeted formatter/linter on changed TS/TSX files.

## Phase D — Legacy `/data-assets/` Route

1. Modify `router/web-router.go`:
   - add the required `os`/`filepath` imports;
   - restore the `/data-assets/` middleware backed by `/data`;
   - keep registration before frontend static serving and fallback handling;
   - enforce path-boundary, traversal/symlink confinement, regular-file, and allowed-image-extension checks;
   - abort invalid/missing asset paths with 404 so they cannot become SPA HTML;
   - set web route tagging, cache headers, and standard file serving for valid assets.
2. Confirm the middleware does not alter `/api`, `/v1`, `/assets`, or ordinary SPA fallback behavior.
3. Add focused router tests using temporary files and a controlled fallback, including the production avatar-shaped PNG contract, missing files, traversal, disallowed extensions, and SPA fallback.
4. Do not add the avatar file to Git, change Docker/Compose, or expose the entire `/data` directory.
5. Run `gofmt` on the router and test files.

## Phase E — Regression Tests

1. Update `controller/llm_review_test.go` to assert a no-proof detail request reaches the normal handler result and that authz remains a separate boundary.
2. Update/add `controller/passkey_test.go` to assert no-proof registration reaches the WebAuthn parsing/validation boundary without consuming the flow prematurely, and that session-bound flow behavior remains.
3. Add/update channel-key route/controller coverage for proof-free Root access, preserving Root-only rejection and audit behavior.
4. Retain proof token service tests and optional `/api/verify`/Passkey proof compatibility tests.
5. Add or update frontend behavior tests in the repository's existing Node/happy-dom style where feasible. At minimum, assert that direct channel-key and Passkey account-management actions do not render the secondary verification dialog.
6. Keep all tests deterministic and focused on observable API/UI behavior; do not add sleeps, random inputs, or implementation-only middleware-name assertions.

## Phase F — Focused Verification

Run from the repository root after each relevant iteration:

```text
gofmt -w <changed Go files>
go test ./controller ./middleware ./router ./service ./service/authz
```

Run the affected frontend tests using the repository's existing Bun/Node test invocation (determine the exact command from current project practice), then:

```text
cd web
bun run typecheck
bun run lint -- <affected files or the configured equivalent>
bun run format:check
cd ..

git diff --check
```

Also run the existing focused frontend regression set that covers request snapshots/usage analysis if the shared secure-verification or router code affects it. Verify with `git grep` that no formerly mandatory product call site remains.

## Phase G — Full Quality Check

1. Run the repository's applicable root Go test/build checks identified from CI and current scripts.
2. Run `go build ./...` from the root module.
3. Run `cd relaykit && GOWORK=off go build ./...` because the final validation policy requires the independent module build when the repository-wide change set is checked, even though no RelayKit file should change.
4. Inspect `git diff --stat`, `git diff --check`, and the full diff for accidental changes to protected identifiers, configuration, runtime files, or unrelated behavior.
5. Use the `trellis-check` quality workflow/agent for a fresh-context review. Resolve all findings before finishing.

## Phase H — Deployment (Only If Explicitly Authorized)

1. Confirm the final commit is pushed only to `origin/rebuild/customizations-20260812` if a push is requested; never push to `upstream`.
2. On `ssh2`, back up `/opt/newapi` source/config/database and current image references before cutover. Do not alter `.env`, Caddy, secrets, optional settings, or routing-group data.
3. Build/tag the exact commit image and cut over `/opt/newapi` using the established backup-first procedure.
4. Validate:
   - container is running with restart count unchanged;
   - `/api/status` and root page return 200;
   - `GET https://newapi.uwoacrimson.com/data-assets/anon-removebg-preview.png` returns the stored PNG bytes and an image content type;
   - missing asset returns 404 rather than SPA HTML;
   - a safe authenticated check confirms changed workflows do not require `X-Security-Proof` while their ordinary auth/permission boundaries remain.
5. If startup, health, or avatar checks fail, stop and roll back to the prior image using the recorded backup. Do not modify production configuration to mask a code defect.

## Rollback Points

- Before backend edits: restore only the working files changed by this task if a design defect is found.
- Before frontend edits: backend tests must show proof-free operation without weakening authz/session behavior.
- Before deployment: all focused/full checks and diff review must pass.
- After deployment: previous image and backup remain available; rollback is image-based and does not modify the database or environment.
