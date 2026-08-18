# Make usage analysis default to Root

## Goal

Open the root-only usage-analysis page with the system's Root user's usage selected by default instead of aggregating all users.

## Confirmed Current Behavior

- `web/src/features/usage-analysis/index.tsx` initializes `userId` to the `All Users` sentinel.
- The usage-analysis API omits `user_id` for that sentinel, and `model/usage_analysis.go` consequently aggregates all users.
- The route and controller already restrict access to root users; this task is about the default data scope, not authorization.
- The model layer has a canonical root-user lookup based on the root role, while the options API currently exposes user IDs and display names for the filter.

## Requirements

- Resolve and select the canonical system Root user for the initial usage-analysis state.
- Ensure the first settled data request is scoped to that Root user rather than silently presenting an all-user aggregate.
- Clearly show the selected Root user in the filter and preserve the existing ability to select another user, `All Users`, token, model, channel, and date-range filters.
- Keep the existing root-only route/sidebar authorization unchanged.
- Use a stable root identity supplied by the backend or an equivalent canonical mechanism; do not make the default depend only on a mutable display label when a stronger identity is available.
- If a root user cannot be resolved, fail safely and visibly rather than labeling an all-user result as Root. The recommended fallback is the existing `All Users` selection with an explicit visible state; it must preserve access to the page and the existing manual filter controls.
- Add or update focused tests for default selection, initial query scope, option loading, and preservation of manual filter changes.

## Acceptance Criteria

- [ ] With normal initialized data, the first completed usage-analysis query contains the Root user's ID and does not use the all-user sentinel.
- [ ] The user filter visibly identifies the Root user after options load.
- [ ] Selecting `All Users` or another user still changes the query and displayed results correctly.
- [ ] Root-only authorization behavior is unchanged.
- [ ] Missing/unavailable root metadata does not produce a misleading Root label or an unsafe hidden filter; the documented fallback is observable to the administrator.
- [ ] Focused frontend tests and the applicable frontend type-check/build pass.

## Out of Scope

- Changing usage-analysis aggregation formulas, date ranges, summaries, or database authorization.
- Changing which users a root administrator is allowed to inspect.
- Renaming the Root account or changing root-role initialization.

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Add `design.md` and `implement.md` before task activation if the implementation remains complex.
