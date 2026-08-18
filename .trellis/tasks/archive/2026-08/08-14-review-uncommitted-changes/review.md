# 未提交定制代码独立 Review

日期：2026-08-14  
分支：`rebuild/customizations-20260812`  
基线：`ccd535ef`  
范围：当前 `HEAD` 以来的未提交产品代码、测试、配置、依赖、迁移和前端改动。Review 期间未修改任何产品源文件、未 stage、未 commit、未 push。

## 结论

**不建议当前直接 commit 或 release。** 发现 4 个 P1 问题，其中固定 Token 分组越权语义、LLM Review 原始请求片段泄露和 Vision 设置无法保存属于上线阻断项；路由迁移吞错可能造成授权数据静默丢失，也必须在提交前修复。另有 5 个 P2 问题以及新增前端 lint 失败。

建议顺序：

1. 修复 F-001、F-002、F-003、F-004，并补齐回归测试。
2. 修复 F-005、F-006、F-007、F-008、F-009。
3. 清理新增 lint/copyright 问题，重新跑完整验证矩阵。
4. 只在完整 Go 测试的基线失败被隔离、前端测试运行器问题被处理或明确豁免后，再考虑提交。

## 1. Candidate inventory

### 1.1 数量

由 `git status --porcelain=v1 --untracked-files=all` 生成的冻结盘点：

| 类别 | 数量 |
|---|---:|
| status 条目总数 | 305 |
| Trellis/agent/会话及规划排除项 | 102 |
| 非排除候选路径 | 203 |
| 已跟踪修改 | 83 |
| 未跟踪候选 | 120 |

非排除候选按顶层目录：

| 层/目录 | 路径数 |
|---|---:|
| 后端/RelayKit（`common`、`constant`、`controller`、`middleware`、`model`、`relay`、`relaykit`、`router`、`service`、`setting`） | 135 |
| 前端 `web/` | 59 |
| 根目录文件 | 6 |
| `docs/` | 3 |

### 1.2 完整路径清单

以下为 203 个非排除候选路径；状态以盘点时的 Git 状态为准，未重复展开 `M`/`??` 标记。

#### 根目录、文档

```text
.gitattributes
AGENTS.md
go.mod
go.sum
main.go
nul
docs/customization-migration-inventory.md
docs/customization-migration-report.md
docs/routing-group-migration-manual.md
```

`nul` 是一个空的未跟踪文件系统条目，不是 Go/前端源文件；本次未删除，提交前应确认是否为误生成物。`AGENTS.md`、`.gitattributes`、依赖文件和文档按治理/基础设施变更检查，不作为业务逻辑 findings。

#### Backend / RelayKit

```text
common/api_type.go
common/endpoint_type.go
common/ip.go
common/rate-limit.go
common/request_snippet.go
common/request_snippet_test.go
common/review_redact.go
common/review_redact_test.go
common/review_snippet.go
common/review_snippet_test.go
common/review_util.go
common/review_util_test.go
constant/api_type.go
constant/channel.go
controller/channel-test.go
controller/channel_authz.go
controller/channel_authz_test.go
controller/channel_test_internal_test.go
controller/group.go
controller/ip_blacklist.go
controller/llm_review.go
controller/llm_review_test.go
controller/passkey.go
controller/passkey_test.go
controller/pricing.go
controller/relay.go
controller/requestsnapshot.go
controller/requestsnapshot_test.go
controller/routing_group_migration.go
controller/routing_group_migration_test.go
controller/secure_verification.go
controller/token.go
controller/token_group_access_test.go
controller/token_preflight.go
controller/token_preflight_test.go
controller/token_test.go
controller/usage_analysis.go
controller/usage_analysis_test.go
controller/user.go
controller/user_ip_blacklist_test.go
middleware/auth.go
middleware/auth_test.go
middleware/distributor.go
middleware/ip_blacklist_auth.go
middleware/model-rate-limit.go
middleware/model_rate_limit_test.go
middleware/secure_verification.go
middleware/vision_intercept.go
model/ability.go
model/channel.go
model/channel_model_group.go
model/channel_model_group_test.go
model/clickhouse_log_test.go
model/ip_blacklist.go
model/ip_security_test.go
model/llm_review_calibration_test.go
model/llm_review_enqueue.go
model/llm_review_enqueue_test.go
model/llm_review_metadata.go
model/llm_review_task.go
model/llm_review_task_db.go
model/llm_review_task_test.go
model/log.go
model/main.go
model/option.go
model/request_snapshot.go
model/task_cas_test.go
model/usage_analysis.go
model/usage_analysis_test.go
model/user.go
model/user_group_grants.go
model/user_permanent_ban.go
model/user_search_ip.go
relay/channel/openai/helper.go
relay/channel/openai/relay-openai.go
relay/channel/openai/stream_usage_test.go
relay/channel/openai/usage.go
relay/channel/openai/usage_test.go
relay/channel/opencodego/adaptor.go
relay/channel/opencodego/adaptor_test.go
relay/common/relay_info.go
relay/constant/relay_mode.go
relay/relay_adaptor.go
relay/relay_adaptor_test.go
relaykit/dto/user_settings.go
router/api-router.go
router/relay-router.go
router/retired_frontend_routes_test.go
service/authz/authz_test.go
service/authz/requestsnapshot_authz_test.go
service/authz/resources_llmreview.go
service/authz/resources_requestsnapshot.go
service/group.go
service/group_access.go
service/group_access_test.go
service/group_resolver.go
service/llm_review_client.go
service/llm_review_client_test.go
service/llm_review_constants.go
service/llm_review_credential.go
service/llm_review_credential_test.go
service/llm_review_enqueue.go
service/llm_review_enqueue_test.go
service/llm_review_payload.go
service/llm_review_payload_test.go
service/llm_review_policy.go
service/llm_review_policy_test.go
service/llm_review_settings.go
service/llm_review_worker.go
service/llm_review_worker_test.go
service/quota.go
service/rate_limit_error.go
service/rate_limit_error_test.go
service/requestsnapshot/crypto.go
service/requestsnapshot/crypto_test.go
service/requestsnapshot/requestsnapshot.go
service/requestsnapshot/requestsnapshot_test.go
service/requestsnapshot/storage.go
service/routing_group_migration.go
service/routing_group_migration_strict_test.go
service/routing_group_migration_test.go
service/token_input_calibration.go
service/token_input_calibration_test.go
service/token_postflight_review.go
service/usage_metrics_test.go
service/vision/intercept.go
service/vision/vision.go
service/vision/vision_test.go
setting/operation_setting/ip_blacklist_setting.go
setting/operation_setting/llm_review_setting.go
setting/operation_setting/rate_limit_ban_setting.go
setting/requestsnapshot_setting/requestsnapshot_setting.go
setting/requestsnapshot_setting/requestsnapshot_setting_test.go
```

#### Frontend

```text
web/scripts/sync-i18n.mjs
web/src/features/auth/secure-verification/types.ts
web/src/features/channels/components/drawers/channel-mutate-drawer.tsx
web/src/features/channels/components/drawers/sections/channel-model-group-policies.tsx
web/src/features/channels/components/drawers/sections/index.ts
web/src/features/channels/constants.ts
web/src/features/channels/lib/__tests__/channel-base-url-switch.test.ts
web/src/features/channels/lib/__tests__/channel-model-group-modes.test.ts
web/src/features/channels/lib/__tests__/new-api-channel.test.ts
web/src/features/channels/lib/__tests__/opencode-go-channel.test.ts
web/src/features/channels/lib/channel-form.ts
web/src/features/channels/lib/channel-type-config.ts
web/src/features/channels/lib/channel-utils.ts
web/src/features/channels/types.ts
web/src/features/llm-review/api.ts
web/src/features/llm-review/components/llm-review-logs-columns.tsx
web/src/features/llm-review/components/llm-review-logs-table.tsx
web/src/features/llm-review/components/review-detail-drawer.tsx
web/src/features/llm-review/components/review-stats.tsx
web/src/features/llm-review/index.tsx
web/src/features/llm-review/lib/__tests__/format.test.ts
web/src/features/llm-review/lib/format.ts
web/src/features/llm-review/types.ts
web/src/features/profile/components/vision-interception-card.tsx
web/src/features/profile/index.tsx
web/src/features/profile/types.ts
web/src/features/system-settings/maintenance/request-snapshot-settings-section.tsx
web/src/features/system-settings/operations/index.tsx
web/src/features/system-settings/operations/section-registry.tsx
web/src/features/system-settings/security/llm-review-section.tsx
web/src/features/system-settings/security/section-registry.tsx
web/src/features/system-settings/types.ts
web/src/features/usage-analysis/__tests__/api.test.ts
web/src/features/usage-analysis/api.ts
web/src/features/usage-analysis/index.tsx
web/src/features/usage-analysis/lib/__tests__/usage-analysis.test.ts
web/src/features/usage-analysis/lib/usage-analysis.ts
web/src/features/usage-logs/components/__tests__/request-snapshot-section.test.tsx
web/src/features/usage-logs/components/dialogs/details-dialog.tsx
web/src/features/usage-logs/components/dialogs/request-snapshot-section.tsx
web/src/features/usage-logs/lib/__tests__/request-snapshot.test.ts
web/src/features/usage-logs/lib/request-snapshot.ts
web/src/features/users/components/users-mutate-drawer.tsx
web/src/features/users/lib/__tests__/user-form.test.ts
web/src/features/users/lib/user-form.ts
web/src/features/users/types.ts
web/src/hooks/use-sidebar-config.ts
web/src/hooks/use-sidebar-data.ts
web/src/i18n/locales/en.json
web/src/i18n/locales/fr.json
web/src/i18n/locales/ja.json
web/src/i18n/locales/ru.json
web/src/i18n/locales/vi.json
web/src/i18n/locales/zh-TW.json
web/src/i18n/locales/zh.json
web/src/lib/admin-permissions.ts
web/src/routeTree.gen.ts
web/src/routes/_authenticated/llm-review-logs/index.tsx
web/src/routes/_authenticated/usage-analysis/index.tsx
```

重复路径 `relaykit/dto/user_settings.go` 在盘点命令输出中只计一次；上表为按目录整理后的清单。

### 1.3 排除项与未审阅项

明确排除：`.trellis/**`、`.agents/**`、`.pi/**`、`.codex/**`、会话 journal、任务规划/归档文件、`HANDOFF.md`、`UPSTREAM.md`、`task_plan.md`、`findings.md`、`progress.md`。这些文件只用于了解需求和已知基线，不作为产品 findings。

没有因路径无法读取而跳过的产品源文件。低风险/无高置信 finding 的区域在第 5 节列出。

## 2. Review 方法

- 阅读 `AGENTS.md`、`web/AGENTS.md`、`.trellis/spec/backend/` 及任务 PRD/design/implement。
- 对权限、迁移、请求体、LLM Review、Vision、计费 usage、前后端 settings contract 做 Storage → Service → Middleware/Controller → API/UI 的跨层追踪。
- 对 SQLite/MySQL/PostgreSQL 迁移和事务路径检查 GORM 使用、错误传播和独立 RelayKit 边界。
- 只把有代码证据、测试证据或命令输出支持的问题列为 finding；将全量测试共享状态/Windows 运行器问题与变更相关问题分开。
- Review 期间没有自动修复 findings，以遵守本任务的 report-only 范围。

## Spec update judgment

本次没有修改 `.trellis/spec/`：固定 Token 语义、billing saturation、RelayKit 独立构建和测试质量要求已经存在于 `task_plan.md`、`AGENTS.md` 与 `.trellis/spec/backend/quality-guidelines.md`；本次新增内容均是对这些既有 contract 的违反或待修复问题，不是已经确认的新 convention。F-001～F-009 修复并经用户确认后，如形成新的可复用边界，再在对应 spec 中记录 executable contract。

## 3. Findings

### F-001 — P1 — 固定分组 Token 可被 Playground 请求分组覆盖

**置信度：高。**

**证据：**

- `service/group_resolver.go:29-74` 的 `ResolveGroupSelection` 先验证 `tokenGroup`，但只要 `requestedGroup` 非空，就再次验证 requested group 并将 `UsingGroup` 设置为 requested group；注释也明确写着 requested 覆盖 token。
- `middleware/auth.go:477-493` 在 TokenAuth 阶段将固定 Token 的 `selection.TokenGroup` 写入 `ContextKeyUsingGroup`。
- `middleware/distributor.go:85-103` 对 `/pg/chat/completions` 读取 body 的 `playgroundRequest.Group`，随后以当前 `usingGroup` 作为 tokenGroup 再调用 resolver，并把 `selection.UsingGroup` 写回上下文。
- `task_plan.md` 和 `HANDOFF.md` 的已确认产品决策是“固定分组 Token 只能使用其固定分组；请求其他分组必须拒绝”。

**影响：** 用户给出一个固定到 A 组的 Token 后，可以在 Playground body 中请求其账户另有权限的 B 组。虽然 B 仍受用户权限校验，但 Token 级固定分组隔离被绕过，可能改变渠道、模型可见性和计费分组；这是明确的上线阻断条件。

**建议：** resolver 分离 fixed/auto 语义：当 tokenGroup 是非空固定组时，requestedGroup 只能为空或与 tokenGroup 完全相同，否则返回稳定 403；只有 `auto` Token 才允许显式请求某个有效固定组。补充 middleware/Playground 回归测试，覆盖固定 Token + 其他有效组、固定 Token + 相同组、Auto Token + 有效组。

### F-002 — P1 — RPM 触发的 LLM Review 持久化并发送原始请求片段

**置信度：高。**

**证据：**

- `middleware/model-rate-limit.go:266-280` 在 RPM 超限触发器中调用 `common.ExtractRequestSnippet(c)`。
- `common/request_snippet.go:9-28` 直接读取前 2048 bytes 并返回原始字符串，不做 JSON 白名单、凭据遮罩或内容类型过滤。
- 已存在的安全实现 `common/review_snippet.go:31-163` 提供 `ExtractLLMReviewSnippetFromContext`，会跳过 image/audio/file/base64、限制长度并遮罩凭据，但 RPM 路径没有使用它。
- `service/rate_limit_error.go:25-41` 原样转发 `RequestSnippet`。
- `service/llm_review_enqueue.go:86-103,133-150` 将其直接写入 `LLMReviewTask.RequestSnippet`，并传入 `buildPayloadSnapshot`。
- `service/llm_review_payload.go:116-147` 的注释声称 payload 已 sanitized，但 `RequestSnippet` 实际仍为 `trigger.RequestSnippet`。
- review disabled、permanent-ban、grace 分支也会先构造带 raw snippet 的 `baseTask` 并持久化 skipped 记录；enabled 分支还会将它送给外部 reviewer。

**影响：** 前 2048 bytes 可能包含 Bearer/API key、cookie、密码、系统提示词、PII 或 base64 前缀。结果同时进入本地数据库，并可能进入外部 LLM Review 服务；这与“review payload 不含凭据”的设计注释矛盾。Malformed legacy payload 的 `legacy_payload_raw` 也需要同样的防御性处理。

**建议：** RPM 入口改用 `ExtractLLMReviewSnippetFromContext(c)`；更重要的是在 `EnqueueLLMReview` 边界再次按 body/trigger 类型做 sanitizer，不能信任调用方已经脱敏。为 task 字段和最终 outbound payload 增加含 Bearer、cookie、base64、structured image 的回归测试，断言原始 secret 不会落库或外发。请求 Snapshot 的“完整原文保存”是另一条明确产品语义，不应复用为 Review payload 语义。

### F-003 — P1 — Vision 设置卡片的保存请求违反后端 settings API contract

**置信度：高。**

**证据：**

- `web/src/features/profile/components/vision-interception-card.tsx:70-81` 的 `handleSave` 只发送 `{ vision: ... }`。
- `controller/user.go:1490-1501` 的 `UpdateUserSettingRequest` 仍把 `notify_type` 和 `quota_warning_threshold` 作为非指针基础字段。
- `controller/user.go:1511-1521` 在处理任何请求时都要求 `notify_type` 是 email/webhook/bark/gotify 之一，并要求 threshold > 0。
- 前端 `web/src/features/profile/types.ts` 将这些 notification 字段全部声明为 optional，说明当前 UI 认为可以提交 partial settings。

**影响：** 用户在 Vision Interception 卡片点击 Save 时，后端收到空的通知类型和 0 threshold，直接返回 400；Vision 新功能无法通过自身 UI 保存。

**建议：** 首选新增专用 Vision settings endpoint；或者把通用 endpoint 改成明确的 partial-merge DTO，在服务层基于现有设置补全必需通知字段。不要让每个局部卡片复制并回传可能包含敏感通知配置的完整 settings。补充前端 API contract 测试和后端 partial/merge regression test。

### F-004 — P1 — 路由兼容迁移吞掉旧授权表查询错误，可能错误标记完成

**置信度：高。**

**证据：**

- `service/routing_group_migration.go:113-120` 的 `scanLegacyRoutingGrants` 在 `tx.Find(&grants)` 失败时直接 `return`，没有返回 error。
- `migrationScan` 只接收 group scan 和 token scan 的 error（`service/routing_group_migration.go:69-84`），因此 grant 查询失败会被当成“没有 grants”。
- `RoutingGroupMigrationReadiness`、`MigrateRoutingGroupCompatibilityDataStrict` 会基于这个空报告继续 readiness/migration；严格迁移随后可能写入 version marker。

**影响：** 数据库权限错误、临时连接错误或旧表 schema/query 错误会静默跳过已有 grants；readiness 可能误报 ready，迁移可能标记完成而不导入授权，造成数据丢失或权限回退，违反 fail-closed 迁移决策。

**建议：** 让 `scanLegacyRoutingGrants` 返回 error 并由 `migrationScan`、preview、readiness、strict run 全链路传播；任意查询错误都必须阻断 readiness 和 version marker。增加可控 query failure 的测试，确认“零写入、无 version marker”。

### F-005 — P2 — Usage Analysis `page_size=1` 会因整数溢出拒绝合法第一页

**置信度：高。**

`controller/usage_analysis.go:66-75` 使用：

```go
if page > (int(^uint(0)>>1)/pageSize)+1 {
    return invalidPage
}
```

当 `pageSize == 1` 时，`maxInt + 1` 溢出为最小负整数，因此合法的 `page=1&page_size=1` 也满足判断并返回 `errUsageAnalysisInvalidPage`。现有测试只覆盖 `page_size=100` 的极限，没有覆盖最小 page size。

**影响：** API 宣称支持 1..100 page size，但最小值不可用；第三方客户端或小屏 UI 可能无法分页。

**建议：** 使用不产生加一溢出的检查（例如比较 `(page-1)` 与 `maxInt/pageSize`，或安全计算 offset），并增加 `page=1&page_size=1`、最大 page/size 边界测试。

### F-006 — P2 — Auto Token 候选组 UI 可展示额外授权组，但保存时拒绝

**置信度：高。**

**证据：**

- `controller/token.go:190-207` 的 `GetTokenAutoGroups` 已通过 `GetUserEffectiveGroups`/`GetAutoGroupsForUser` 返回用户的额外 active grants。
- `controller/token.go:106-139` 的 `setTokenAutoGroups` 却只调用 `service.IsUserSelectableGroup(userGroup, group)`，该函数基于账户 tier 的 `GetUserUsableGroups`，不读取 user id 对应的 `user_group_grants`。

**影响：** 用户可以在候选组列表看到自己被授予的额外组，但提交 Auto Token 时被 `MsgTokenAutoGroupsInvalid` 拒绝，导致展示与写入 contract 不一致；不是越权，但使额外分组功能不可用。

**建议：** 保存路径使用同一 effective access resolver（按 user id 验证 active grant、目录和过期时间），并保留 duplicate/max-count 校验。补充“额外授权组可获取、可保存、撤销后拒绝”的 controller 回归测试。

### F-007 — P2 — Vision 子请求没有继承父请求的 cancellation/deadline

**置信度：高。**

`service/vision/vision.go:371` 接收 `ctx context.Context`，但 `analyzeImageWithRelay` 后续没有使用它；`service/vision/vision.go:390` 创建子上下文时调用 `newVisionSubContext(c)`；`service/vision/vision.go:580-584` 使用 `parent.Request.Clone(context.Background())`。最终 outbound 请求在 `service/vision/vision.go:458` 使用这个 background-derived request。

**影响：** 客户端断开、父请求超时或上游 deadline 到期后，Vision 子调用仍可能继续执行、占用上游连接和 worker、产生额外计费/日志；高并发图片请求时会放大资源泄漏。

**建议：** 子 context 使用传入的父 `ctx`/`parent.Request.Context()`（保留 identity context values 的同时只替换 request context），并在 channel selection、HTTP DoRequest、response read 处统一沿用 deadline。增加 canceled parent 不发起/及时取消 outbound 的测试。

### F-008 — P2 — OpenAI-compatible cache usage fallback 接受负数并进入计费

**置信度：高。**

- `relay/channel/openai/usage.go:135-151` 的 DeepSeek/OpenCodeGo 分支用 `PromptCacheHitTokens != 0` 判断，负值也会被写入 cached tokens。
- `relay/channel/openai/usage.go:185-233` 的 body fallback 对 `cached_tokens`/`prompt_cache_hit_tokens` 只检查字段存在，不检查 `>0` 或上限。
- `service/text_quota.go:257-263,281-327` 将这些值直接用于 cache/base token 的 billing arithmetic；日志侧的后置 clamp 不能修复已经完成的 charge calculation。

**影响：** 恶意或异常上游响应中的负 cache count 可改变 base token 与 cache ratio，造成错误收费；过大的值还会触发不必要的饱和/异常审计。现有测试覆盖了负值 merge source，但没有覆盖 body fallback 和 `!= 0` 分支。

**建议：** 所有 upstream cache metrics 统一使用正数、有限上限的 normalization helper；在 body fallback、channel-specific mapping、billing usage normalization 入口都执行，必要时记录 quota clamp。补充 DeepSeek/OpenCodeGo 负数 JSON 和极大值测试，断言 billing usage 不含负 cache tokens。

### F-009 — P2 — LLM Review calibration 的 token 差值在校验前可能整数溢出

**置信度：中高。**

`model/llm_review_task_db.go:760-783` 只检查 `estimate >= 0`、`actual > 0`，随后执行 `float64(estimate-actual)`。这些样本来自结算后 usage/estimate（`service/text_quota.go:544-548`），含有上游/请求相关 token 数；在极大合法 `int` 输入下，减法会先在整数域溢出，再转 float。

**影响：** `relative_error` 可能被记录为错误值，进而污染“估算器已验收”统计；异常样本可能改变输入 Token preflight 的 fail-open/eligible 判定。

**建议：** 在样本入口复用统一 token/quota 上限，先做有界 `int64`/decimal 差值或饱和绝对差，再计算 relative error；对 `estimate=MaxInt`、`actual=1`、超界/负数样本增加持久化边界测试。

### F-010 — P3 — 新增 LLM Review 前端无法通过 targeted lint

**置信度：高。**

`bunx oxlint -c .oxlintrc.json` 对新增/受影响路径报告：

- `web/src/features/llm-review/lib/format.ts:93` 缺少 curly braces；
- `web/src/features/llm-review/components/llm-review-logs-columns.tsx:156,195,218,231,246` 缺少 curly braces；
- `web/src/features/llm-review/components/review-detail-drawer.tsx:316` 使用 array index 作为 React key；
- 当前修改的 `web/src/features/users/lib/user-form.ts:30` 有 import type side effect lint error。

**影响：** 新增功能未达到仓库的受影响文件 lint gate；后续 lint 失败会掩盖真正的回归。

**建议：** 修复上述机械 lint 问题，再单独运行 affected-file lint；不要用禁用规则绕过。

## 4. Validation matrix

| 命令 | 结果 | 分类/说明 |
|---|---|---|
| `E:/myCode/.tools/go1.26.1/go/bin/go.exe build ./...` | PASS | 在先完成前端 build 生成 `web/dist` 后通过 |
| `cd relaykit && GOWORK=off .../go.exe build ./...` | PASS | RelayKit 独立构建通过 |
| `go test ./common ./controller ./middleware ./model ./service/vision ./relay/channel/openai ./relay/channel/opencodego -count=1` | PASS | 受影响后端包通过 |
| `go test ./... -count=1` | FAIL | 两类已知/可隔离 baseline：Windows HTTP/2 socket abort 用例；service channel-affinity 测试共享全局状态 |
| `go test ./relay/channel -run TestUpstreamGetBody_HTTP2CannotRetryWithoutGetBody -count=1` | PASS | 失败仅出现在全量并发运行环境，单独复跑通过 |
| `go test ./service -run 'TestObserveChannelAffinityUsageCacheByRelayFormat_(MixedMode\|UnsupportedModeKeepsEmpty)$' -count=1` | PASS | 共享状态污染，单独复跑通过 |
| `cd web && bun run typecheck` | PASS | `tsgo -b` |
| `cd web && bun run build` | PASS | Rsbuild production build |
| `cd web && bun test` | FAIL | 123 pass、3 个 API-key-group-cell baseline failures、14 个 `node:test describe`/Bun runner errors；共 140 tests/40 files |
| `cd web && bun run lint` | FAIL | 全量存在上游既有大量 lint baseline；同时新增 LLM Review 路径有 F-010 的 targeted errors |
| affected-file `bunx oxlint` | FAIL | F-010；影响范围内其余主要路径未发现额外 lint error |
| affected-file `bunx oxfmt --check ...` | PASS | 新增/受影响重点路径格式通过 |
| `cd web && bun run format:check` | FAIL | `scripts/add-copyright.mjs`、`scripts/format-with-protected-headers.mjs`、`api-key-group-cell.tsx`、`redemption-form.ts` |
| `cd web && bun run copyright:check` | FAIL | `oauth-callback-mode.ts`、`channel-field-update.ts`、`model-categories.ts` 需要更新 header |
| 七 locale flattened key scan | PASS | 7 个 locale，5430 个 key，0 missing/extra |
| `git diff --check` | PASS | 仅报告 `.gitattributes`/`go.mod`/`go.sum`/`sync-i18n.mjs` 的 LF→CRLF 警告，无 whitespace error |

全量 Go/前端测试的失败没有被伪报为通过；隔离复跑结果已单独记录。前端 Bun 的 `node:test` 嵌套 describe 是当前运行器兼容性问题，但新增测试仍因此无法由标准 `bun test` 全量命令执行，提交前应给出明确 runner 方案或豁免记录。

## 5. 无高置信 finding 的区域

- Request Snapshot：默认关闭、AES-GCM/HKDF、节点本地受控存储、owner/permission proof、成功读取前审计和清理流程均已检查，未发现比产品“完整原文保存”语义更高置信的问题。
- IP blacklist、永久禁用后的 auth/session/token invalidation、RPM 原子窗口、429 contract 和失败槽位释放：受影响 focused tests 通过，未发现新的高置信越权/失效问题。
- OpenCode Go adaptor、OpenAI usage stream 合并、RelayKit DTO/独立模块边界：focused tests 和 `GOWORK=off` build 通过，未发现新的高置信协议/模块问题；F-008 是 usage negative-input 防御缺口。
- Channel model group 三态、事务/ability 生命周期、用户额外组的基本读写 presence semantics：focused tests 通过；F-006 是 Token Auto 保存路径的跨层不一致。
- 七语言 locale key parity：结构化 key 集合一致，未发现缺 key/多 key。

## 6. Follow-up / release gate

本 review 不修改产品代码。建议先建立后续修复任务处理 F-001～F-009，另以小修复处理 F-010 和 copyright/format gate。完成后至少重新运行：

```text
E:\myCode\.tools\go1.26.1\go\bin\go.exe test ./... -count=1
E:\myCode\.tools\go1.26.1\go\bin\go.exe build ./...
cd relaykit && GOWORK=off E:\myCode\.tools\go1.26.1\go\bin\go.exe build ./...
cd web && bun run typecheck
cd web && bun run build
cd web && bun run lint
cd web && bun test
cd web && bun run format:check
cd web && bun run copyright:check
git diff --check
```

在 P1 问题、targeted lint 和 settings/migration regression 修复完成前，结论保持：**不安全直接提交或发布**。

## 7. Remediation follow-up（2026-08-14）

后续任务 `.trellis/tasks/08-14-remediate-review-findings` 已完成 F-001、F-003～F-010 的代码修复和回归验证；F-002 经用户明确接受，保持现有 request-snippet/payload 行为，不在本轮修改。

- F-001、F-004、F-005、F-006：固定 Token/Auto Token 授权、迁移查询 fail-closed、分页边界的 focused tests 通过。
- F-003、F-007：新增 Vision 专用设置接口并贯通父请求 cancellation；后端、前端 focused tests 通过。
- F-008、F-009：cache-token fallback 和 calibration 样本均增加有界校验及回归测试。
- F-010：受影响文件 targeted `oxlint`/`oxfmt --check` 通过。
- `go build ./...`、RelayKit `GOWORK=off go build ./...`、前端 typecheck/build、task-scoped tests、`git diff --check` 均通过。
- 全量 Go/Bun/lint/format/copyright 的剩余失败已分类为本任务范围外的既有 baseline 或 Windows/Bun 环境差异，详见 remediation parent 的 `implement.md`。
- 最终复核另发现的两个迁移问题也已在用户批准扩展范围后修复：活跃的 unmappable/orphan legacy grant 会阻断严格迁移，过期孤儿授权不阻断；grant preview/status 会比较目标记录，成功迁移后 pending grant 可归零且 `InSync=true`。

本 follow-up 已完成：产品范围提交为 `da678d51`，并部署到 `ssh2` 镜像 `newapi-custom:20260814-remediated-da678d51`；未 push、未 archive。部署后健康检查通过，严格 routing-group 迁移因旧 key `渠道1` 无法映射而按设计保持阻断。
