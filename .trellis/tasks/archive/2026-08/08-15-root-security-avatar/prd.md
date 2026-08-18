# Remove Mandatory Security Proof Gates and Restore Legacy Avatar Access

## Goal

Stop requiring secondary 2FA/Passkey security proofs for any applicable role or workflow, while preserving ordinary authentication, authorization, browser-session checks, WebAuthn ceremonies, rate limits, audits, and account-security features. Restore the legacy `/data-assets/*` URL so the configured avatar and other approved data-volume images are served correctly.

## Background and Confirmed Findings

### Security-proof behavior

- `middleware.RequireSecurityProof` is the shared backend gate that currently requires a live dashboard session and a valid `X-Security-Proof` header.
- Current mandatory proof call sites are:
  - `POST /api/channel/:id/key` through `middleware.SecureVerificationRequired`; the route is still Root-only and also uses `AdminAuth`, `CriticalRateLimit`, and `DisableCache`.
  - LLM Review task detail, retry, and create-task handlers through `requireLLMReviewProof`; the route group separately uses `AdminAuth` and `authz.LLMReviewRead`.
  - Passkey registration begin/finish and deletion through proof helpers; the routes separately use `UserAuth`, and the handlers also use live session binding and WebAuthn/auth-version logic.
- LLM Review list, queue summary, and grace endpoints do not currently call the proof helper even though an old route comment says some of them do.
- The request-snapshot viewer was already changed to Root-only direct access and must remain proof-free, audited, rate-limited, non-cacheable, and protected by its existing stable error contracts.
- The proof JWT issuer/verifier, `/api/verify`, and Passkey proof begin/finish endpoints are separate optional compatibility features. The user wants the mandatory operation gates removed, not the 2FA/Passkey login, enrollment, deletion, or account-security features removed.
- Frontend forced-proof consumers are the channel drawer and Passkey account card. The generic secure-verification module is otherwise a compatibility layer; the current LLM Review UI does not send proof headers or open a proof dialog.

### Legacy avatar behavior

- Current `router.SetWebRouter` serves embedded `web/dist` files and returns the SPA index for unknown non-API paths. It has no `/data-assets/` handler.
- A previously deployed source used `serveDataAssets("/data-assets/", "/data")` to serve approved image extensions from the mounted data directory.
- Production `ssh2` contains `/opt/newapi/data/anon-removebg-preview.png` (344,251 bytes, mode 0644), mounted in the `newapi` container as `/data/anon-removebg-preview.png`.
- `GET https://newapi.uwoacrimson.com/data-assets/anon-removebg-preview.png` currently returns HTTP 200 `text/html` from the SPA fallback instead of the PNG. The file is present; routing is the defect.
- Docker builds the Go binary and embeds the frontend, while Compose supplies the existing `/data` volume. The avatar must remain a data-volume file; no new tracked asset or production configuration change is required.

## In Scope

### Mandatory proof removal

- Remove every current mandatory `X-Security-Proof` gate from channel-key disclosure, LLM Review detail/retry/create operations, and Passkey registration/deletion.
- Remove the corresponding frontend flows that proactively open or retry through a 2FA/Passkey proof dialog.
- Keep ordinary authentication and authorization boundaries:
  - channel-key disclosure remains Root-only;
  - LLM Review task operations retain `AdminAuth` plus `authz.LLMReviewRead`;
  - Passkey account operations retain `UserAuth`, enabled-user checks, live session binding, and WebAuthn ceremonies.
- Keep critical rate limiting, cache controls, manage/security audits, auth-version rotation, request-flow binding/consumption, and existing stable response behavior.
- Keep 2FA/Passkey login, enrollment, deletion, and account-security settings available. Existing proof APIs may remain for compatibility, but no product workflow in scope may require a secondary proof to proceed.

### Legacy data assets

- Add `GET /data-assets/<path>` backed by the process/container `/data` directory.
- Serve approved image files with their actual content type and normal file-serving behavior.
- Reject missing, directory, disallowed-extension, traversal, and outside-root/symlink paths with HTTP 404; these requests must not become SPA HTML.
- Preserve ordinary SPA fallback and `/api`, `/v1`, and `/assets` not-found behavior.

## Out of Scope

- Removing or disabling 2FA/Passkey login, enrollment, deletion, or account-security settings.
- Removing ordinary `RootAuth`, `AdminAuth`, `UserAuth`, permission checks, live-session requirements that belong to a flow, WebAuthn validation, rate limits, audits, or request-snapshot protections.
- Changing the proof JWT format, issuer/verifier semantics, optional proof endpoints, production secrets, environment variables, Caddy configuration, database contents, routing-group data, or the mounted avatar file.
- Exposing the whole `/data` directory or adding a broad filesystem-serving rule.
- Adding a replacement avatar to the repository or Docker image.

## Observable Acceptance Criteria

- [x] A channel-key request with no `X-Security-Proof` succeeds for an otherwise authorized Root user and still has its existing Root-only, rate-limit, no-cache, secret lookup, and audit behavior.
- [x] LLM Review detail, retry, and create-task requests no longer reject missing/invalid secondary proof, while the existing admin permission and operation validation remain enforced.
- [x] Passkey registration and deletion no longer require a 2FA/Passkey proof, while browser support, WebAuthn ceremony validation, live session flow binding, auth-version rotation, and security audits remain intact.
- [x] The frontend channel-key and Passkey account-management flows do not open a secondary verification dialog or require a proof token.
- [x] Existing 2FA/Passkey login, enrollment, deletion, account-security settings, and optional proof endpoints remain available.
- [x] `GET /data-assets/anon-removebg-preview.png` returns HTTP 200, an image content type (specifically `image/png` for the production file), and the stored PNG bytes.
- [x] Missing, traversal, outside-root/symlink, directory, and disallowed-extension asset requests return 404 and never return the SPA index.
- [x] An ordinary client-side route still returns the SPA index, and existing API/relay not-found behavior remains unchanged.
- [x] Regression tests cover proof-free backend behavior, preserved auth/session/WebAuthn boundaries, frontend no-dialog behavior, and valid/missing/traversal static assets.
- [x] Focused backend/frontend tests, applicable typecheck/lint/format/build checks, root build, Relaykit independent build, and `git diff --check` pass.
- [x] If deployed, the change is backup-first and exact-commit; production health, avatar content, and proof-free workflow checks pass without changing configuration.

### Completion Record

- Product commit: `9d828d74`; documentation commit: `609750f4`; both pushed to `origin/rebuild/customizations-20260812`.
- Production image: `newapi-custom:20260815-security-avatar-9d828d74` (`sha256:d2e56d70f7bf4c8921b2c69a87b776e4d03c062422753b87825984214accd891`).
- Backup: `/opt/newapi/backups/deploy-20260815-145129-9d828d74`; SQLite backup size 202,997,760 bytes and `PRAGMA integrity_check` returned `ok`.
- Production checks: container running with restart count 0; `/api/status` and `/` returned 200; avatar returned exact stored PNG bytes; missing, POST, and traversal asset requests returned 404.
- Existing unrelated Trellis/agent runtime files and the modified backend quality spec remain outside the product commits.

## Key Decisions and Risks

- **Decision:** Remove mandatory proof enforcement at actual route/controller call sites, not by turning the shared verifier into a global no-op. This preserves optional proof compatibility and prevents future intentional callers from silently losing validation.
- **Decision:** Keep the channel-key Root authorization boundary; “no secondary proof” does not mean “ordinary admin access.”
- **Decision:** Serve the avatar from the existing `/data` mount rather than packaging a copy into `web/dist` or the Docker image.
- **Risk:** Removing secondary proof weakens protection for sensitive operations by design. Ordinary authentication, authorization, critical rate limits, no-cache controls, audits, and WebAuthn/session invariants are the compensating boundaries that must remain tested.
- **Risk:** SPA fallback can hide missing static files as HTTP 200 HTML. The asset handler must abort invalid asset paths before fallback.
- **Deferred compatibility note:** If production uses a non-empty `FRONTEND_BASE_URL`, verify whether the data-asset middleware must be registered before that frontend-mode branch; do not change production configuration to work around it.

## Planning Status

Requirements are resolved. Technical design is in `design.md`; ordered implementation and validation steps are in `implement.md`. No product-code implementation starts until the user approves the final planning summary and the Trellis task is activated.
