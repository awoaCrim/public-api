# Implementation Plan: Vision Contract Fixes

1. Add backend tests proving a Vision-only update succeeds and preserves unrelated user settings.
2. Add the focused Vision settings request DTO, controller, and authenticated route using the existing API envelope.
3. Add frontend API tests for the focused endpoint and update the Vision card call site.
4. Add a Vision service regression test for inherited cancellation and an outbound relay test server/adapter that observes cancellation.
5. Pass the parent context through `newVisionSubContext`, and create the shared non-form upstream HTTP request with the Gin request context so cancellation reaches the provider call.
6. Run:
   - `go test ./controller ./service/vision ./middleware ./relay/channel -count=1`
   - `cd web && bun test <focused Vision/profile tests>`
   - `cd web && bun run typecheck`
   - affected-file `oxlint` and `oxfmt --check`
   - root Go build and `git diff --check`.
7. If `relaykit/dto/user_settings.go` remains touched, re-run `cd relaykit && GOWORK=off go build ./...`.

## Verification

- `go test ./controller -run '^TestUpdateUserVisionSetting' -count=1`: pass.
- `go test ./service/vision -count=1`: pass.
- `go test ./relay/channel -run '^TestDoApiRequestPropagatesGinRequestCancellationUpstream$' -count=1`: pass.
- `go test ./middleware -run Vision -count=1`: pass.
- `go build ./...`: pass after the frontend build generated `web/dist`.
- `cd relaykit && GOWORK=off go build ./...`: pass.
- Focused frontend API test, typecheck, affected-file oxlint/oxfmt, and production build: pass.
- `git diff --check`: pass.
- Broad package rerun retained the documented unrelated baselines: the middleware RPM/LLM-review shared fixture and Windows HTTP/2 socket-abort relay tests.

## Rollback

The focused API/UI change and context-propagation change are separable. Neither requires a database migration.
