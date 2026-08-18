# Fix Vision Settings and Cancellation

## Goal

Correct F-003 and F-007 so users can save Vision configuration independently and Vision provider calls stop with the parent request.

## Requirements

- Provide a focused authenticated Vision-settings update API that accepts only the Vision configuration.
- Updating Vision must preserve notification, language, security, and all other user settings.
- The frontend Vision card must use the focused API and keep the existing success/error UX.
- Optional Vision scalar fields must preserve explicit zero/false values.
- Vision subrequests must use the parent request context/deadline for outbound work.
- Cancellation before or during the provider request must stop the subrequest and avoid post-consume settlement as a successful response.

## Out of Scope

- Redesigning the general notification settings endpoint.
- Changing Vision fail-open behavior, cache identity, image limits, provider selection, or F-002.

## Acceptance Criteria

- [x] Saving only Vision settings succeeds for a user with existing valid notification settings.
- [x] All unrelated settings remain unchanged after the Vision update.
- [x] The Vision card calls the focused API and updates local profile/auth state only after success.
- [x] A canceled/deadline-exceeded parent request cancels the outbound Vision request.
- [x] Existing Vision cache, billing refund, and interception tests continue to pass.
- [x] Backend focused tests, frontend focused tests/typecheck, and affected-file lint pass.
