# 交接文档：New API 定制重建

> 更新日期：2026-08-15
> 交接范围：阶段 1–7、Review remediation、产品提交、推送和 `ssh2` 部署均已完成。最新产品提交：`bd8b8746`（`feat: restore admin observability UX`）；部署镜像：`newapi-custom:20260815-observability-bd8b8746`。分支已推送至 `origin/rebuild/customizations-20260812`；严格 routing-group 迁移因旧 key `渠道1` 阻断而未执行。交付报告：`docs/customization-migration-report.md`；迁移/回滚手册：`docs/routing-group-migration-manual.md`。
> 接手须知：本文是后续开发者的唯一入口。先读本文，再读 `docs/customization-migration-inventory.md`（功能清单）、`findings.md`（发现与基线验证）、`task_plan.md`（阶段计划）、`progress.md`（进度记录）。

---

## 1. 项目背景与目标

用户要求基于最新版 New API 重建一个来源边界混乱的旧 fork，而不是继续打补丁。

### 1.1 两个仓库

| 角色 | 路径 | 说明 |
|---|---|---|
| 旧仓库（只读） | `E:\myCode\myapi` | 上游 `awoaCrim/myapi.git`，HEAD `bd14bea`。首个提交 `e747e19` 为无父整仓快照。**永不修改** |
| 新仓库（唯一开发位置） | `E:\myCode\public-api` | fork `https://github.com/awoaCrim/public-api.git`，工作分支 `rebuild/customizations-20260812` |

### 1.2 固定上游基线

- 上游：`https://github.com/QuantumNous/new-api.git`
- 基线提交：`ccd535ef8e50cf6e5846a59278c40b7ff59d1b7d`（`fix: harden concurrent quota and status updates`）
- 已配置 remote：`origin=awoaCrim/public-api`、`upstream=QuantumNous/new-api`

### 1.3 重建原则（用户明确要求）

1. 先提交功能级定制清单并获用户确认，再迁移业务实现（已确认）。
2. 上游已具备的功能直接采用上游实现，不重复移植旧代码。
3. 仅对本地独有功能按最新版架构重新设计。
4. 禁止用旧仓库文件整文件覆盖新版文件；禁止整体 cherry-pick。
5. 修复外部审查指出的安全、权限、事务、迁移、前端兼容与性能问题。
6. 默认不 commit、不 push；最终提交推送方式由用户另行确认。
7. 不操作生产环境。

---

## 2. 已确认的产品决策（用户逐项确认）

| # | 决策 | 确认时间/方式 |
|---|---|---|
| 1 | 按正式清单迁移全部建议功能 | 2026-08-12 结构化提问确认 |
| 2 | **请求体快照保存完整内容，不脱敏不截断**；但必须默认关闭、专用敏感权限、访问审计、受控存储、容量限制、保留期清理 | 同上 |
| 3 | RPM 采用旧仓库**最终代码语义**：Redis ZSET/Lua 原子滑动窗口、OpenAI 兼容 429、失败释放槽位并触发 LLM Review；**不恢复**早期短期/长期自动封禁 | 同上 |
| 4 | 用户保留唯一账户主分组，可额外获得多个原有分组权限（拟 `user_group_grants` 表） | 早期会话确认 |
| 5 | 固定 Token 只能使用其固定分组；请求其他组或 `auto` 返回 403 | 早期会话确认 |
| 6 | Auto Token 保留最新版每 Token 候选组能力，实际候选必须与用户当前有效权限取交集；显式空快照不回退全局 | 早期会话确认 |
| 7 | 渠道模型分组采用 `inherit` / `custom` / `disabled` 三态 | 早期会话确认 |
| 8 | 旧 `routing_groups` 平行模型不恢复为运行时权威，仅做 fail-closed dry-run 兼容迁移 | 早期会话确认 |

---

## 3. 已完成工作

### 3.1 阶段 1：盘点与基线验证（已完成）

已产出的正式文档：

- `docs/customization-migration-inventory.md` —— 正式功能级定制清单（核心交付物）
- `findings.md` —— 发现记录与基线验证结果
- `task_plan.md` —— 阶段计划（含上线阻断条件）
- `progress.md` —— 进度记录

关键盘点结论：

1. **属于官方上游历史、不迁移**（11 个提交）：管理员细粒度权限、ClickHouse LIKE、Playground/Markdown、认证文案、Responses/Chat 转换、渠道 UI。已用 `git cat-file` 在本地仓库逐个核验存在。
2. **本地独有、需重建**：Vision Interception、请求体日志、RPM/IP 封禁、OpenCode Go、usage/cache 归一化、输入 Token 前置硬限制与估算器校准、用户额外分组、渠道模型三态、Usage Analysis 管理页与聚合 API（`controller/usage_analysis.go` + `web/default/src/features/usage-analysis/`）。
3. **补充发现**：旧变更日志未单列的"输入 Token 前置硬限制与估算器校准验收"（`controller/token_preflight.go`、`service/token_input_calibration.go`）是完整业务定制，需迁移。
4. **重要纠偏**：旧文档声称 Vision 已抽到 `plugin/`，但旧仓库实际代码仍在 `middleware/vision_intercept.go` 与 `service/vision/vision.go`，以实际代码为准。
5. **类型冲突**：旧 OpenCode Go 使用渠道/API 类型 59；最新版上游已将 59 分配给 Sub2API、60 分配给 New API。迁移需新编号 + dry-run 识别。
6. **已排除误报（均属上游已有，不迁移）**：通过逐文件核验确认 Chat2Link、rankings、wallet/pricing/subscriptions 前端页面，以及 `oauth/linuxdo.go`、`service/rankings.go`、`service/codex_credential_refresh_task.go`、`common/custom-event.go`、`controller/video_proxy_gemini.go`、`middleware/email-verification-rate-limit.go` 在最新版上游（public-api）中均已存在。这些不是本地独有定制。

### 3.2 基线验证结果（已记录于 findings.md）

| 项目 | 结果 |
|---|---|
| 前端依赖安装 | `bun install --frozen-lockfile --force` 成功，锁文件未改写（首次普通安装因 tarball 完整性校验失败，`--force` 重下解决） |
| 前端 typecheck / production build | 通过 |
| 前端全量 lint / format | **上游基线既有失败**，不要顺手修复，要求不新增 |
| `relaykit` 独立构建 | `GOWORK=off go build ./...` 通过 |
| 完整 `go test ./...` | 仅 `TestObserveChannelAffinityUsageCacheByRelayFormat_UnsupportedModeKeepsEmpty` 因上游测试共享状态污染失败（单独 `-count=1` 复跑通过） |
| `git diff --check` | 通过 |

### 3.3 批次 A 部分：OpenCode Go（已完成，含第三轮复核 3 项缺口修复）

已实现（测试先行，受影响 Go 包测试全部通过）：

- 渠道类型：`Sub2API=59`、`NewAPI=60`、`OpenCodeGo=61`、`Dummy` 顺延 `62`；API type 同步注册。
- `ChannelType2APIType`、`GetAdaptor`、endpoint capability、`streamSupportedChannels` 注册。
- 默认 base URL `https://opencode.ai`；前端 type 61、顺序紧邻 New API、OpenAI 图标、模型拉取、提示与七种 locale i18n。
- OpenAI 流式请求强制 `stream_options.include_usage=true`；nil request 报错；非流式不改写。
- Claude 转换（经三轮 review 已修正的部分）：
  - 删除 billing header 专用块；字符串 system 若 billing-only 清空；billing header 与正文分行共存时保留正文；
  - `cch=<token>` 清理支持 `;,)]}.` 等边界；
  - `messages[].role=system` 大小写不敏感改 `user`，top-level 正常 system 保留；
  - 转换后移除所有消息内容块 `cache_control`；
  - 重复 sanitize 幂等。
- usage：OpenCode Go 仅从 `PromptCacheHitTokens` 或 body `usage.prompt_cache_hit_tokens` 回填（专门的 `extractOpenCodeGoCachedTokensFromBody`），不覆盖标准值；DeepSeek 行为未回归。

第三轮复核 3 项缺口修复（均有回归测试）：

1. **billing header 清理收窄**：`removeBillingHeaderLines` 只剥离 billing header 区域（`billingHeaderRegionEnd`：从 `x-anthropic-billing-header` 起、到 `cch=` token 结束并吞掉尾随 `;` 与空白；无 `cch=` 字段时整行视为 header），同行非 header 正文保留。测试：`TestRemoveBillingHeaderLinesPreservesSameLineContent`、`TestConvertClaudeRequestKeepsSameLineContentBesideBillingHeader`、`TestConvertClaudeRequestPreservesNormalTextBesideBillingHeader`、`TestConvertClaudeRequestPreservesMixedBillingBlockTextAndFields`。
2. **CCH token 边界完整**：`isCCHTokenChar` 按 CCH token 合法字符（hex 数字、字母、`-`、`_`）界定 token 结束，`:`、`!`、`?`、引号等标点之后的正文不再被吞；`removeVolatileCCH` 复用同一界定并处理 `;`/尾部空白。测试：`TestRemoveVolatileCCHPreservesTextAfterPunctuation`。
3. **VolcEngine type-45 base URL 缺口**：`CHANNEL_TYPE_CONFIGS[45]` 补齐默认 `https://ark.cn-beijing.volces.com`（原硬编码在 `channel-mutate-drawer.tsx`，改为 `getDefaultBaseUrl(45)`）；新增 `getBaseUrlForChannelTypeChange` 统一逻辑：既有渠道的配置默认值在切换类型时被替换、真正自定义 URL 被保留。测试：`web/src/features/channels/lib/__tests__/channel-base-url-switch.test.ts`（默认值替换 + 自定义保留 + 跨渠道往返）。

当前所有改动均未提交，全部位于 `rebuild/customizations-20260812` 分支（具体改动清单以 `git status` 为准）。

### 3.4 批次 A：OpenAI-compatible usage 归一化（原 4.2.1，已完成）

实际范围与行为（以代码为准）：

- 仅 OpenAI chat/completions（`relay/channel/openai` 的 `OpenaiHandler` 非流式 seam 与 `OaiStreamHandler` 流式）；不涉及 Responses 等其他 relay 格式。
- `input_tokens`/`output_tokens` 别名仅在标准 `prompt_tokens`/`completion_tokens` 为零时回填（fill-if-zero），非零标准值优先；`total_tokens` 缺失时按已填充对重算；仅 total-only 的 usage 不臆造 input/output 拆分。
- 流式任意 SSE chunk 的 usage 均被捕获（`extractStreamUsage` 深拷贝 + 归一化）；最后一次携带可计费 totals 的 chunk 优先于最终空 chunk（`lastStreamUsage`），空尾 chunk 不再回退本地估算。
- 跨 chunk 的 cache/detail 字段合并（`mergeUsageDetails`/`mergeInputTokenDetails`）：只填零值字段、不覆盖非零标准值、负值不作为填充源；`InputTokensDetails`/`BillingUsage` 指针深拷贝，避免跨 chunk 别名。
- 仅 cache-only 或 total-only 的 chunk 不会抑制估算回退（`service.ValidUsage` 语义未变）。
- OpenCode Go body fallback 保持窄范围：仅 `usage.prompt_cache_hit_tokens`（专门的 `extractOpenCodeGoCachedTokensFromBody`），不覆盖标准值；DeepSeek 行为未回归（`applyUsagePostProcessing` 内 DeepSeek/OpenCodeGo 分支独立）。
- `service.ValidUsage` 与 relaykit 公共 API 未改动；`relaykit` GOWORK=off 独立构建通过。

测试：`relay/channel/openai/usage_test.go`（别名归一化、跨 chunk 合并、负值拒绝、深拷贝、OpenCode Go fallback 窄化）、`relay/channel/openai/stream_usage_test.go`（中间 chunk usage、跨 chunk 缓存合并、cache-only 不抑制估算、alias-only 不触发估算）。

### 3.5 批次 A：安全请求快照（已完成）

已按用户确认的“完整保存，不脱敏、不截断”语义在最新版架构中完成安全重写：

- 默认关闭；仅在显式配置 `CRYPTO_SECRET` 或 `SESSION_SECRET` 且进程已加载稳定密钥时运行，否则 fail-closed 并记录安全状态码，不写明文文件。
- 在认证和请求校验成功、进入后续敏感词/配额/上游处理前，精确复制当前请求 body storage 的完整字节；OpenAI Realtime/WebSocket 跳过；同一 `request_id` 只保存一次。
- `service/requestsnapshot` 使用 HKDF-SHA256 域分离派生 AES-256-GCM 密钥，随机 nonce，AAD 绑定 request id 与相对文件名；目录 `0700`、文件 `0600`、同目录临时文件 + rename 原子写入，安全哈希文件名和节点目录名防路径穿越。
- 元数据和访问审计只写主数据库：`RequestSnapshot` / `RequestSnapshotAccess` 同时注册普通/快速迁移；正文仅存在节点本地加密文件中，不进入日志列表、主库字段或日志库/ClickHouse。
- 容量与生命周期：单体/节点总容量限制；每节点独立清理循环；按 retention 再 capacity 的确定性 oldest-first 删除；孤儿文件/缺失文件对账；失败、missing、tombstone、access 记录均按保留期有界清理；设置值带上下界和溢出保护。
- 权限链路：请求体内容不可委派给普通管理员，仅 Root 超级管理员可读；路由为 `RootAuth + CriticalRateLimit + DisableCache`，读取不再要求 2FA/Passkey proof。
- 每个 Root 授权后的成功/失败读取尝试写独立审计；成功内容读取在审计存储可用且成功审计落库前绝不返回正文；通用 I/O/DB 错误仅返回稳定安全码，不暴露本地路径。
- 前端：Usage Log 详情仅在 Root 超级管理员且存在 request id 时显示按需按钮；点击后直接读取，正文仅保存在组件内存并在关闭、切换日志或迟到响应时清除，支持滚动全文、复制和原始字节下载；System Settings > Operations > Request Snapshots 提供独立的默认关闭设置页面。
- i18n：新增文案通过 `web/scripts/add-missing-keys.mjs` 写入七种 locale，随后执行 `bun run i18n:sync`；报告为全 locale `0 missing / 0 extras / 0 untranslated`。

验证：request snapshot/authz/settings/controller focused Go tests、model/middleware tests、`go build ./...`、11 个 focused Bun tests、`bun run typecheck`、受影响文件 oxlint、`relaykit` `GOWORK=off go build ./...`、`git diff --check` 均通过；无 staged 文件。独立 security/lifecycle/authz/frontend/oracle review 后修复了 proof scope 不可签发、成功审计未 fail-closed、failed/missing 行无界、no-store/rate limit、通用错误泄露路径及设置溢出等问题。

剩余非阻断风险：cleanup 的 own-node 文件对账当前会读取该节点全部 metadata 行；配置热更新仍沿用项目现有 lock-free setting 模式；更换 `CRYPTO_SECRET` 会使旧快照不可读，部署时必须把密钥轮换与 retention/清理计划配套。

### 3.6 批次 A：结构化缓存指标 + SQL Usage Analysis（已完成）

已按“日志写入时结构化、在线查询纯 SQL”的目标重建：

- `logs` 新增 `cache_read_tokens`、`cache_write_tokens`、`cache_write_tokens_5m`、`cache_write_tokens_1h`、`input_tokens_total`；普通日志库走 GORM migration，ClickHouse 同时更新 CREATE TABLE 和幂等 `ADD COLUMN IF NOT EXISTS`。
- 文本日志从 `effectiveBillingUsage`/`textQuotaSummary` 写入；Audio、Realtime/WSS 与渠道测试共用 `ApplyUsageMetricsToConsumeLogParams`；Anthropic total input 保留 cache read/write，OpenAI-style 使用 prompt/input tokens。Midjourney、任务计费、违规费等没有 Token usage 的 consume logs 保持 structured fields=0，作为 legacy rows 可见。
- `model.GetUsageAnalysis` 使用三类有界 SQL：summary aggregate、grouped count + paginated rows、hourly trend。在线查询不读取原始日志切片、不解析 `other` JSON；legacy rows 单独计数，且从 cache-rate 分子/分母排除。
- API：`GET /api/usage-analysis`、`GET /api/usage-analysis/options`；`RootAuth + CriticalRateLimit + DisableCache`；默认 24 小时、最大 90 天、15 秒 timeout（504）、page size 1..100；主查询的 summary/count/rows/trend、options 查询与渠道名解析均绑定同一 deadline；options 具备结果上限，模型选项只扫描最近 90 天；渠道名从主库按当前页 ID 批量解析。
- 前端：`web/src/features/usage-analysis/`；root-only route/sidebar；用户/API Key/模型/渠道 + 日期筛选；summary cards、Recharts 小时趋势、分页 breakdown；错误/空状态、反向日期校验；Refresh 在筛选未变化时也会显式 refetch，分页保留上一页数据显示 loading，业务/HTTP 错误统一页内显示；七 locale i18n。
- 测试覆盖结构化聚合、分页 summary 稳定、legacy exclusion、ClickHouse columns、query range/page validation、root auth、route registration、usage metric normalization/负数与 Int32 饱和、frontend query/trend/filter helper 和页内错误配置。

验证：focused Go tests、focused Bun tests、`bun run typecheck`、changed-file oxlint、`bun run build`、root `go build ./...`、relaykit `GOWORK=off go build ./...`、`git diff --check` 均通过；无 staged 文件。完整 `go test ./...` 仍受既有 channel-affinity 全局状态污染影响，另有 Windows 下 HTTP/2 GOAWAY retry 用例在全量并发时偶发 socket abort，定向复跑通过。

---

## 4. 当前待办（接手者优先处理）

### 4.1 阶段 3：RPM/IP 安全链路（已完成）

已完成：

1. Redis Lua + ZSET atomic sliding-window 和同语义 memory fallback；OpenAI-compatible 429、`Retry-After`/`X-RateLimit-*` headers；downstream failure release 当前 request slot。
2. RPM 两条 limit path 都 non-blocking 触发 review。当前 upstream 没有 LLM Review module，默认 hook 持久化 `rate_limit_review_events`；enabled 用户为 `pending`，已 disabled 用户保留 `skipped_disabled` 审计状态，root 跳过。
3. 默认关闭的 exact IPv4 blacklist；root exemption；永久禁用递增 auth version、invalidate cache/token、revoke sessions，并从 `LOG_DB` 收集 distinct historical IPv4 + triggering IP。
4. login/register/dashboard/session/API Token/relay Token auth enforcement；relay denial 使用 OpenAI-compatible envelope。
5. root-only audited `/api/ip_blacklist` list/add/remove；local cache + Redis per-IP positive/negative cache（miss 直接查询主库后短 TTL 回填）；exact IPv4 users search；enable user 时清理 rate-limit keys。
6. focused tests 已覆盖 RPM atomic/headers/release/review，以及 exact-IP/permanent-disable/security-audit/root/search/event persistence 和 blacklisted registration 403/no-token contract。

- 完整验证：Phase 3 focused Go selector 全绿；frontend typecheck/build、root build、relaykit standalone build 通过；全量 Go 仅既有 service channel-affinity shared-state 与 Windows HTTP/2 GOAWAY flakes，逐个复跑通过；
- 独立 security/correctness review 已完成，multi-node cache poisoning、multipart preservation、stream failure release、register denial、audit/snippet/count/stream flag 等 actionable findings 均已处理；
- 下一步进入阶段 4：Vision 与完整 LLM Review worker/module。

### 4.2 后续批次

- **批次 B（安全与审查）**：RPM/IP 安全链路已完成；剩余完整 LLM Review worker/module 与输入 Token 前置限制/校准，归入阶段 4。
- **批次 C**：Vision Interception（按最新版架构重建，默认可关闭，SSRF 威胁模型，缓存隔离，最新版请求管线接入）。
- **批次 D（复杂度最高，产品语义已确认）**：用户额外分组授权与版本化缓存、统一组解析器（固定/Auto/显式请求组）、用户更新 DTO presence 语义、渠道模型三态、ability/cache 原子投影、旧 routing group dry-run 迁移与 readiness。
  - 旧仓库已有可参考的回归测试资产（只读参考，勿整体复制）：`service/group_access_test.go`（主分组+授权并集/过期过滤）、`service/group_resolver_test.go`（固定/auto/显式请求组、撤销即时生效、Root 特权）、`model/channel_group_override_test.go`（覆盖行生命周期与 `Channel.Update()` 全量重建 abilities）、`service/routing_group_migration_test.go`（可映射/不可映射、幂等、唯一索引冲突）、`controller/token_group_access_test.go`（固定 Token 越权拒绝）、`controller/model_list_test.go`（auto 组能力并集）。
  - 数据库迁移注意事项（参考旧仓库 `model/main.go`）：新增表必须同时注册 `migrateDB()` 与 `migrateDBFast()`，否则快速迁移/重启建表失败；SQLite 用 `PRAGMA table_info` + `ALTER TABLE ADD COLUMN` 手工路径；注意 PostgreSQL 双引号/`true`-`false` 与 MySQL/反引号/`1`-`0` 的方言归一化差异、boolean `default:true` 与 decimal 价格列在三方言的行为差异。
- **阶段 6**：兼容迁移、数据对账、回滚文档（fail-closed，SQLite/MySQL/PostgreSQL）。
  - 数据对账基线（来自旧仓库 `progress.md`，仅作参考）：旧生产已部署镜像 `newapi-custom:20260812-routing-unified`，生产库为 SQLite（`/opt/newapi/data/one-api.db`），`user_group_grants` 已导入 `default=234`、`vip=1`；旧表 `routing_groups`/`user_routing_group_grants`/`user_routing_preferences` 及 Token 旧列保留未删。本重建项目不应直接复用这些生产数据，仅用于核对映射语义。
- **阶段 7**：完整验证与交付报告（含迁移手册、回滚手册、验证报告）。

---

## 5. 验证环境与命令

### 5.1 工具链

| 工具 | 路径/版本 |
|---|---|
| 便携 Go | `E:\myCode\.tools\go1.26.1\go\bin\go.exe`（已解压验证，**未改系统 PATH**） |
| Bun | `1.3.10`（本机可用） |
| 项目 Go 版本 | go.mod 要求 1.25.1；Docker 镜像用 golang:1.26.1-alpine |

### 5.2 常用命令

```powershell
# 后端测试（受影响包）
Set-Location "E:\myCode\public-api"
& "E:\myCode\.tools\go1.26.1\go\bin\go.exe" test <pkg...> -count=1

# relaykit 独立构建（务必单独验证）
$env:GOWORK='off'
Set-Location "E:\myCode\public-api\relaykit"
& "E:\myCode\.tools\go1.26.1\go\bin\go.exe" build ./...

# 前端
Set-Location "E:\myCode\public-api\web"
bun run typecheck
bun run build
bun test <focused-test-file>          # 定向
bunx oxlint -c .oxlintrc.json <files> # 定向
bunx oxfmt --check <files>            # 定向
node scripts/sync-i18n.mjs            # 修改 locale 后必须执行
bun run copyright:check               # 新增/修改带版权头文件后检查
bun run knip                          # 可选：检查未使用依赖/导出
```

### 5.3 基线已知问题（不要顺手修复，只要求不新增）

- 前端全量 `lint`、`format:check` 在上游基线即失败。
- 完整 `go test ./...` 有既有 channel-affinity 全局状态污染；Windows 下两个 HTTP/2 GOAWAY retry 用例在全量并发时偶发 socket abort，定向复跑通过。
- `git diff --check` 对 `web/scripts/sync-i18n.mjs` 有 LF/CRLF 提示（无实际错误）。

---

## 6. 硬性约束（接手者必须遵守）

1. 旧仓库 `E:\myCode\myapi` 只读，永不修改。
2. 不整体 cherry-pick、不用旧文件覆盖新文件。
3. 不 commit、不 push、不碰生产，除非用户明确确认。
4. 新功能测试先行（先新增测试并观察失败，再实现）。
5. 业务代码使用项目 `common` JSON wrapper，禁止直接 `encoding/json` marshal/unmarshal。
6. Go 测试使用 testify（assert/require）。
7. 前端新文案必须接入 i18n（七种 locale），不硬编码中文。
8. 不修改品牌、归属与受保护标识。
9. `relaykit` 是独立 module，任何改动后必须 `GOWORK=off` 独立构建。
10. 规划文档（`docs/customization-migration-inventory.md` 等）保留，不删除、不覆盖。

---

## 7. 关键参考文件

| 文件 | 用途 |
|---|---|
| `docs/customization-migration-inventory.md` | 正式功能清单（状态、优先级、批次、验收门槛） |
| `findings.md` | 盘点结论 + 基线验证结果 + 等待确认记录 |
| `task_plan.md` | 阶段计划 + 上线阻断条件 |
| `progress.md` | 进度记录 |
| 旧仓库 `docs/newapi-local-change-log.md` | 旧定制提交历史与行为说明（只读参考） |
| 旧仓库 `docs/superpowers/specs/` | 两份设计文档（永久封禁/IP、多分组路由） |

## 8. 当前 git 状态快照

- 分支：`rebuild/customizations-20260812`
- 产品提交：`bd8b8746`（`feat: restore admin observability UX`）；前序完整重建提交为 `da678d51`。
- 部署：2026-08-15 04:25 +08:00，`ssh2` 的 `newapi` 容器正在运行镜像 `newapi-custom:20260815-observability-bd8b8746`，镜像 ID 为 `sha256:bc3a6c51743b88aca6909bbb9d66a063eda666e2331752b7ca4dfd7cf786a794`；容器运行、重启次数 0，`/api/status` 和首页均为 HTTP 200。
- 部署前 SQLite 备份完整性为 `ok`，备份目录：`/opt/newapi/backups/deploy-20260815-042500-bd8b8746`。
- 分支已推送并跟踪 `origin/rebuild/customizations-20260812`。未纳入发布的本地 Agent/Trellis 运行时、任务元数据及 `nul` 仍保留在工作区，具体清单以 `git status` 为准。
