# Design: Vision Contract Fixes

## API boundary

Add a dedicated authenticated route under the existing user settings area, for example `PUT /api/user/setting/vision`, with a DTO containing the Vision object only. The controller loads the current user settings, replaces only `UserSetting.Vision`, persists through the existing user-setting storage path, and returns the standard dashboard envelope.

The existing general `/api/user/setting` notification contract remains unchanged, avoiding a broad presence-semantics migration.

## Frontend flow

Add a focused profile API function and update `VisionInterceptionCard` to call it. On success, merge the returned/saved Vision value into existing local settings exactly as today; on failure, keep local state unchanged and show the existing localized error.

## Cancellation flow

Thread the supplied `context.Context` into `newVisionSubContext` and clone the parent request with that context rather than `context.Background()`. The shared non-form HTTP relay boundary must create the upstream `http.Request` with `c.Request.Context()`; otherwise the adaptor discards the Gin request context while constructing a new request. A Vision helper regression and an outbound relay regression together protect the complete cancellation chain.

## Error and billing behavior

Cancellation is an ordinary Vision analysis failure under the existing fail-open interception policy. Any pre-consumed quota remains covered by the existing deferred refund when settlement did not complete.
