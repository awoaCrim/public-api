# Independent audit findings

The existing core registration policy is positioned correctly and uses calendar-year arithmetic. Release gaps:

1. Zod integer/range errors are default English instead of translated UI text.
2. `oauth/github_test.go` tests only `parseGitHubCreatedAt`, not actual GitHub response decoding through `GetUserInfo`.
3. Legacy GitHub login/migration before the age gate lacks a regression test.
4. Startup option initialization and HTTP option GET/invalid PUT behavior lack end-to-end tests.
5. Typed callback errors and the real React settings form lack behavior-level tests.
6. Several runtime/test files are untracked and must be included explicitly in the final feature manifest.
