# Design: close GitHub OAuth age release gaps

## Backend contracts

Use the existing option key and runtime variable. Extend tests around `InitOptionMap`, `UpdateOption`, and the HTTP option controller rather than introducing a new configuration system.

Add a deterministic test seam for `GitHubProvider.GetUserInfo` so an `httptest` GitHub response exercises JSON decoding, required user fields, and optional `created_at` projection. Keep production defaults pointed at GitHub and avoid global network mutation or real API calls.

Keep registration policy ordering unchanged: numeric lookup, legacy lookup/migration, registration-enabled gate, then age check, then all new-user side effects. Add HTTP-level callback tests for the typed age errors and language selection while keeping bind routed before registration.

## Frontend contracts

Construct the Zod schema with translated integer/range messages. Reuse the existing numeric-field adapter and render errors through `FormMessage`. Add a focused React Testing Library test for the actual GitHub tab/form and option mutation boundary; use behavior queries and controlled API mocks, not implementation-state assertions.

All new/changed locale values are applied through `web/scripts/add-missing-keys.mjs`, synchronized, reported clean, and the temporary script removed.

## Compatibility

No database schema change is needed. Existing records and existing linked users remain compatible. Provider metadata remains optional because only new registration requires it.
