# Backend directory structure

This repository is a Go/Gin API gateway. The backend is organized by runtime responsibility rather than by one package per endpoint. New backend code should fit the existing request path and package ownership below.

## Repository-level layout

| Path | Responsibility | Existing examples |
| --- | --- | --- |
| `main.go` | Process startup, resource initialization, router assembly, graceful shutdown | `InitResources`, `gin.CustomRecovery`, signal handling in `main.go` |
| `router/` | Route registration and route-level middleware composition | `router/api-router.go`, `router/relay-router.go`, `router/video-router.go`, `router/channel-router.go` |
| `controller/` | Gin handlers for dashboard/API operations; bind input, call model/service/relay code, write the response | `controller/channel.go`, `controller/auth_session.go`, `controller/channel-test.go` |
| `service/` | Cross-cutting/application workflows and integrations that are not persistence models | `service/auth_cleanup.go`, `service/auth_session.go`, `service/authz/` |
| `model/` | GORM models, database queries, transactions, migrations, caches tightly coupled to model state | `model/user.go`, `model/channel.go`, `model/main.go`, `model/log.go` |
| `middleware/` | Authentication, authorization, request body handling, distribution, rate limiting, logging-related request behavior | `middleware/auth.go`, `middleware/distributor.go`, `middleware/utils.go` |
| `relay/` | Provider-independent relay handlers, request conversion, billing/usage integration, and provider adapters | `relay/relay_adaptor.go`, `relay/common/`, `relay/channel/openai/`, `relay/helper/` |
| `relaykit/` | Standalone protocol DTO/conversion module; no dependency on the root module | `relaykit/dto/`, `relaykit/relayconvert/`, `relaykit/types/` |
| `dto/` | Root-module request/response DTOs used by controllers and relay code | `dto/task.go`, `dto/video.go`, `dto/suno.go` |
| `constant/` | Stable enums and keys shared across layers | `constant/context_key.go`, `constant/channel.go`, `constant/finish_reason.go` |
| `common/` | Root-module infrastructure helpers and shared policy implementations | `common/json.go`, `common/database.go`, `common/quota_math.go`, `common/body_storage.go` |
| `types/` | Root-module shared data structures and concurrency-safe containers | `types/price_data.go`, `types/rw_map.go` |
| `logger/` | Request-aware application logging and log rotation | `logger/logger.go` |
| `i18n/` | Backend translations (`en`, `zh`) | `i18n/locales/`, `i18n/i18n.go` |
| `oauth/` | OAuth provider implementations and provider registry | `oauth/github.go`, `oauth/oidc.go`, `oauth/registry.go` |
| `setting/` | Runtime setting groups and setting validation/serialization | `setting/operation_setting/`, `setting/system_setting/`, `setting/config/` |
| `pkg/` | Internal reusable packages and integrations | `pkg/cachex/`, `pkg/ionet/`, `pkg/billingexpr/`, `pkg/perf_metrics/` |
| `bin/` | Operational migration/scripts | `bin/migration_v0.2-v0.3.sql`, `bin/time_test.sh` |

The root `go.mod` is the main application module. `relaykit/go.mod` is a separate module even though it is nested under the repository root.

## Request flow

The normal HTTP flow is:

1. A file under `router/` registers a route and composes middleware.
2. Middleware authenticates, limits, stores/replays the request body, and/or selects a channel. For example, `router/relay-router.go` uses `middleware.TokenAuth()` and `middleware.Distribute()` for `/v1` relay routes.
3. A controller under `controller/` handles dashboard/API operations. A relay endpoint may enter `relay/` directly after `middleware.Distribute()`.
4. Controllers and relay handlers call `service/`, `model/`, `common/`, or provider adapters as appropriate.
5. `model/` owns persistent state and database transactions; it should not be bypassed with ad-hoc persistence in controllers.
6. The handler writes the protocol response: dashboard endpoints commonly use `common.ApiSuccess`/`common.ApiError`, while OpenAI-compatible relay endpoints use `relaykit/types.NewAPIError` and the OpenAI error shape.

The route files are the source of truth for endpoint ownership and middleware order. For example, `router/api-router.go` groups `/api/user`, `/api/subscription`, `/api/channel`, `/api/log`, and `/api/system-task` with different `UserAuth`, `AdminAuth`, or `RootAuth` requirements.

## Where to put a change

- Add a dashboard/API handler beside related handlers in `controller/`, then register it in the matching `router/*-router.go` file. Keep authorization in the route/middleware composition and enforce resource-level checks in the handler/service where needed.
- Add a database-backed entity, query, transaction, or migration helper in `model/`. Add the entity to `migrateDB` in `model/main.go` when it is a root database model; log-only entities must follow `migrateLOGDB`/ClickHouse handling.
- Add reusable request/response shapes in `dto/`; provider protocol shapes shared by the standalone converter belong in `relaykit/dto/` instead. For example, OpenAI zero-value request coverage is under `relaykit/dto/`, while root task/video DTOs remain under `dto/`.
- Add a provider adapter under the existing `relay/channel/<provider>/` convention and connect it through the relay adaptor registry. Do not put provider-specific transport logic in a controller.
- Add cross-cutting request behavior to `middleware/`; use `common/` only for helpers that are genuinely shared outside HTTP middleware.
- Add a billing expression feature only after reading `pkg/billingexpr/expr.md`; keep expression parsing/evaluation in `pkg/billingexpr/` and relay-specific extraction in `relay/helper/` or the relevant relay package.
- Add tests next to the owning package using the repository's existing `*_test.go` naming. Model integration/regression tests live in `model/`, route tests in `router/`, controller contract tests in `controller/`, and relay conversion/boundary tests in `relay/` or `relaykit/relayconvert/`.

## RelayKit boundary

`relaykit/` must remain independently buildable. Code in that module may use its own `relaykit/relayconvert/kitutil` helpers and `relaykit` packages, but must not import root `common`, `model`, `service`, `setting`, Gin, or root configuration. The module's own `relaykit/README.md` documents the supported protocol-conversion surface and is the concrete boundary for additions.
