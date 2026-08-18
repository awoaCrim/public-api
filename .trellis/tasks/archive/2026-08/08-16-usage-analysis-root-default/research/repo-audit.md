# Repository audit: usage-analysis default scope

Date: 2026-08-16

- `web/src/features/usage-analysis/index.tsx` initializes both editable and applied filters with `userId: 'all'`.
- The analysis query is enabled immediately, in parallel with `/api/usage-analysis/options`; the first request can therefore aggregate all users before user options arrive.
- `web/src/features/usage-analysis/api.ts` currently models options as users/tokens/models/channels and has no root-user identity field.
- `controller/usage_analysis.go` returns enabled users sorted by username but no canonical root ID.
- `model/usage_analysis.go` treats `UserID <= 0` as no user filter, so omitted `user_id` means all users.
- The API routes are root-only already; this task must not change authorization.
- `common.RoleRootUser` is the canonical root role. `model.GetRootUser` selects the first row with that role, and initial setup creates username `root` with that role.

Recommended boundary: expose a canonical `root_user_id` in the options response, delay the aggregate query until options are resolved, initialize both filter states to that ID, and fall back visibly to the existing all-user sentinel only when a successful options response contains no resolvable root.
