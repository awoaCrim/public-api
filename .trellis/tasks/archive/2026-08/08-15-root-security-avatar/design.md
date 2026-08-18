# Technical Design: Remove Mandatory Proof Gates and Restore Data Assets

## 1. Design Summary

The change has two independent runtime paths:

1. **Secondary verification**: remove only the mandatory proof checks from sensitive operation workflows. Keep normal authentication, authorization, browser-session requirements, WebAuthn ceremonies, rate limits, audits, auth-version rotation, and the existing optional proof/token APIs intact.
2. **Legacy assets**: restore a `/data-assets/` middleware-backed file handler in the embedded web router. It serves approved image types from `/data`, rejects invalid/missing/traversal requests with 404, and prevents those requests from reaching the SPA fallback.

The implementation should remove the proof requirement at each actual enforcement call site rather than turning `RequireSecurityProof` into a global no-op. This preserves the existing proof implementation for compatibility and avoids weakening any future caller that intentionally uses it.

## 2. Security Boundary and Data Flow

### 2.1 Secondary proof behavior

Current sensitive-flow path:

```text
Dashboard credential
  -> AdminAuth/RootAuth/UserAuth
  -> permission/rate/cache middleware
  -> RequireSecurityProof / SecureVerificationRequired
  -> controller operation
```

Target path:

```text
Dashboard credential
  -> existing AdminAuth/RootAuth/UserAuth
  -> existing permission/rate/cache middleware
  -> controller operation
```

`X-Security-Proof` remains accepted by the existing optional API wrappers and the proof issuer/verifier remains available for compatibility, but these product workflows no longer branch into a proof dialog or reject a request because the header is absent/expired/wrong-scope.

### 2.2 Channel-key disclosure

Change `router/channel-router.go` so `POST /api/channel/:id/key` retains:

- outer `AdminAuth`;
- route-level `RootAuth` (the operation remains Root-only);
- `CriticalRateLimit`;
- `DisableCache`;
- `controller.GetChannelKey` parsing, secret-bearing channel lookup, and manage audit.

Remove only `SecureVerificationRequired`. Do not broaden the endpoint to delegated administrators and do not remove its cache/rate protections.

Update `controller.GetChannelKey` comments so they no longer claim the controller depends on security verification.

### 2.3 LLM Review operations

Remove `requireLLMReviewProof` calls from the handlers that currently invoke it (`GetLLMReviewTaskDetail`, `RetryLLMReviewTask`, and `CreateLLMReviewTask`). Preserve:

- `AdminAuth` and `RequirePermission(authz.LLMReviewRead)` for the task route group;
- Root-only authorization for configuration/test/credential-management routes;
- task ID and request validation;
- task mutation and queue behavior;
- existing manage-audit records.

Keep `RequireSecurityProof`, the proof scope constants, and proof issuance/verification endpoints unless implementation inspection proves a now-unused declaration can be removed without breaking compatibility. The `requireLLMReviewProof` helper may be removed if no caller remains, but no shared proof verifier should be made permissive globally.

Update router/controller comments that currently describe proof as required.

### 2.4 Passkey registration and deletion

Remove the secondary proof checks from `PasskeyRegisterBegin`, `PasskeyRegisterFinish`, and `PasskeyDelete` while preserving the actual account-security flow:

- `UserAuth` route protection and `getAuthenticatedUser` enabled-user checks;
- Passkey feature setting check;
- live dashboard-session identity for registration flow binding and session rotation;
- WebAuthn creation ceremony, existing-credential exclusion, and registration flow-token creation/consumption;
- WebAuthn credential validation and persistence;
- deletion with auth-version protection;
- session auth-version advancement and user security audits.

The old delete helper also checks whether a credential exists. Preserve that user-visible behavior in a focused `ensurePasskeyExists`-style check (or equivalent direct branch) after removing its proof responsibility. A delete request for an unbound user must continue returning the existing `success:false` response instead of silently succeeding.

Do not remove `PasskeyVerifyBegin`/`PasskeyVerifyFinish`: they are optional proof-issuance compatibility endpoints and are not the mandatory gates being removed. Do not remove 2FA login, Passkey login, 2FA enrollment/deletion, or Passkey account management.

## 3. Frontend Design

### 3.1 Channel drawer

In `channel-mutate-drawer.tsx`, call the existing channel-key API directly from `handleRevealKey` instead of routing through `useSecureVerification.withVerification`. Remove the now-unused secure-verification hook/dialog state and rendering from this drawer. Keep the API wrapper's optional proof-token parameter if it is part of the compatibility surface, but the normal UI call sends no proof header.

The user-visible behavior becomes: click reveal, fetch the key, show the existing success/error toast; no 2FA/Passkey dialog is opened.

### 3.2 Passkey card

In `profile/components/passkey-card.tsx`:

- registration calls `register()` after the existing browser support check;
- deletion calls `remove()` after the existing confirmation dialog and capability/availability checks needed by the API;
- remove `useSecureVerification`, restricted-method selection, verification dialog state, and proof-specific copy from this component;
- keep the browser WebAuthn prompt, loading state, confirmation dialog, status refresh, and account-security presentation.

Keep optional proof-token arguments in the lower-level passkey API functions only if needed for compatibility; direct account-management calls do not supply them.

### 3.3 Generic verification module

Leave `web/src/features/auth/secure-verification/` available for compatibility with the backend proof endpoints and any future optional consumer. After the component changes, verify no product UI still calls `startVerification` or `withVerification` for channel-key or Passkey registration/deletion. Do not delete the generic module solely because these current consumers are removed.

No new user-facing copy is required. If existing request-snapshot/help copy still says a proof is required, update that copy through the normal i18n workflow in a separate focused edit rather than leaving a false claim.

## 4. Static Asset Design

### 4.1 Route placement

Restore the data-asset middleware in `router.SetWebRouter`, before gzip/cache/static frontend handling, matching the existing embedded-frontend architecture and the deployed `/data` mount. The production master currently uses this embedded frontend path. Do not change Docker, Compose, Caddy, environment variables, or the mounted data directory.

If implementation discovers that production runs with a non-empty `FRONTEND_BASE_URL`, move the same middleware registration to the common router setup before the frontend-mode branch; otherwise keep the smaller `SetWebRouter` change.

### 4.2 Handler contract

`serveDataAssets("/data-assets/", "/data")` should:

- recognize only `/data-assets` and `/data-assets/` descendants, not look-alike prefixes such as `/data-assets-other`;
- clean URL-relative paths and reject `.`/`..` escapes, including encoded traversal that reaches `URL.Path`;
- resolve the target under the configured root and reject symlink targets that resolve outside the root;
- permit only the legacy image extensions (`png`, `jpg`, `jpeg`, `gif`, `svg`, `ico`, `webp`), case-insensitively;
- serve only existing regular files from `/data` using `c.File`/the standard HTTP file-serving path so MIME detection, HEAD, and range behavior remain standard;
- set `RouteTagKey` to `web` and `Cache-Control: public, max-age=86400` for successful assets;
- abort with HTTP 404 for invalid, missing, directory, disallowed-extension, or traversal requests so they cannot reach the SPA fallback;
- leave unrelated requests untouched so existing embedded frontend, `/api`, `/v1`, and `/assets` fallback behavior remains unchanged.

The exact production avatar remains a data-volume file, not a new tracked frontend asset. The Docker image still contains only the binary; the existing Compose volume supplies `/data/anon-removebg-preview.png`.

## 5. Test Design

### 5.1 Backend security regressions

Update the proof-dependent tests rather than deleting proof-token unit coverage:

- LLM Review controller test: a task-detail request without `X-Security-Proof` must pass the handler gate and return the existing no-task result; a wrong/expired proof header must not become a new requirement. Keep separate route/authz coverage for `AdminAuth` and `LLMReviewRead`.
- Passkey controller test: rewrite the test that currently expects missing/wrong proof rejection so the same request gets past proof validation and fails only at the intentionally invalid WebAuthn credential boundary, while the registration flow remains unconsumed. Add/retain a success-path or begin-flow assertion that ordinary session binding still exists.
- Channel-key regression: exercise the Root-only route/controller without a proof header and assert successful key disclosure/audit behavior, while retaining tests for non-Root rejection, rate-limit middleware, and no-cache headers where existing fixtures support them. Do not test the absence of a middleware function by name when an observable route behavior can cover it.
- Retain `service/auth_token_test.go` proof binding/purpose tests and middleware proof-contract tests if the optional proof API remains.

### 5.2 Frontend regressions

Use the repository's existing Node test + happy-dom pattern for affected components where practical:

- channel-key action: direct reveal succeeds without rendering a secure-verification dialog;
- Passkey card: enable and remove actions invoke the normal Passkey API flow without rendering the security-verification dialog, while the browser support check, confirmation dialog, and loading behavior remain;
- run a repository search after edits to ensure no current product call site wraps these actions in `startVerification`/`withVerification`.

If the component harness becomes disproportionately expensive, preserve the observable contract through the smallest focused API/action test and document the remaining static call-site audit in the check manifest.

### 5.3 Static route tests

Add `router/web-router_test.go` (or the existing router web test file if one is found) using a temporary data directory and a small Gin test router around `serveDataAssets` plus a controlled SPA fallback:

- valid PNG returns 200, exact fixture bytes, and `image/png` content type;
- missing PNG returns 404 and not the SPA body;
- directory/disallowed-extension/traversal and encoded traversal requests return 404 and cannot read outside the root;
- ordinary `/some/client/route` still returns the SPA fallback;
- API fallback behavior remains covered by existing router tests or a small regression assertion if the new middleware affects the chain.

## 6. Compatibility, Operations, and Rollback

- No database schema or data migration is needed.
- No production environment, Compose, Caddy, routing-group, secret, or mounted-file changes are needed.
- Existing clients that send `X-Security-Proof` continue to receive normal endpoint responses; the header is simply ignored by these formerly gated operations.
- Existing clients that use `/api/verify` or Passkey step-up endpoints remain compatible because the token implementation is retained.
- Build the exact Git commit into a new Docker image. Before cutover, back up the source/config/database/image references using the established deployment procedure. Keep the current observability image for rollback.
- Post-deployment checks: container health/restart count, `/api/status`, root page, exact avatar URL content type/body signature, and one authenticated proof-free request for each changed operation where safe to exercise. Roll back to the prior image if startup, health, or asset checks fail.
