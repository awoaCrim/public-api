# New API 定制重建进度

## 2026-08-12

- 已确认用户的新 fork：`https://github.com/awoaCrim/public-api.git`。
- 已核验 fork main 与官方上游 main 当前均指向 `ccd535ef8e50cf6e5846a59278c40b7ff59d1b7d`。
- 已将新 fork 克隆到 `E:\myCode\public-api`；旧仓库 `E:\myCode\myapi` 保持不动。
- 已配置 `origin=awoaCrim/public-api`、`upstream=QuantumNous/new-api`。
- 已创建工作分支 `rebuild/customizations-20260812`，未提交、未推送。
- 已读取并整理外部 ChatGPT 审查全文；确认其列出的固定 Token、用户更新、迁移、前端保存、缓存聚合和模型覆盖问题与旧代码一致。
- 用户确认 Auto Token 保留最新版上游的每 Token 候选组限制，并与用户当前有效权限取交集。
- 用户确认渠道模型分组采用 `inherit`、`custom`、`disabled` 三态。
- 已完成新旧仓库功能级盘点，并生成 `docs/customization-migration-inventory.md`。
- 已确认旧记录中 11 个提交属于官方上游历史，对应权限、Playground、协议转换和渠道 UI 不重复迁移。
- 已确认 Vision、请求体日志、RPM/IP、OpenCode Go 等本地提交不在上游历史。
- 已补充识别“输入 Token 前置硬限制与估算器校准验收”这一旧日志未单列的本地功能。
- 已确认旧 OpenCode Go type 59 与最新版 Sub2API type 59 冲突，必须使用新编号并提供 dry-run 数据识别。
- 已确认旧 RPM 最终代码语义为原子滑动窗口、429 和 LLM Review 触发，不是早期文档描述的自动短期/长期封禁。
- 前端依赖安装成功；首次下载完整性失败后使用 `--force` 重新下载，`bun.lock` 未改写。
- 前端基线：typecheck 通过，production build 通过；全量 lint 和 format check 因最新版上游既有问题失败。
- 已在仓库外安装便携 Go `1.26.1`，未修改系统 PATH 或 Git 配置。
- `relaykit` 使用 `GOWORK=off` 独立构建通过。
- 完整 `go test ./...` 只有一个上游测试共享状态污染失败；该测试单独复跑通过。
- `git diff --check` 通过。
- 阶段 1 基线验证完成时尚未修改业务代码；当前批次 A 改动均未提交、未推送、未部署。
- 用户已确认按正式清单迁移全部建议功能。
- 用户已确认请求快照保存完整请求体；实现仍增加默认关闭、敏感权限、审计、受控存储、容量和保留期保护，但不脱敏、不截断内容。
- 用户已确认 RPM 使用最终代码语义，不恢复早期自动封禁。
- HANDOFF 4.2.1 OpenAI-compatible usage 行为差异测试与归一化已完成（未提交）：新增流式任意 chunk usage 捕获与跨 chunk 缓存合并、input/output 别名归一化（流式+非流式 seam）、负值拒绝与缓存超 prompt 的计费安全测试；`relay/channel/openai`、`relay/channel/opencodego` 测试通过，`relaykit` GOWORK=off 构建通过，`git diff --check` 通过；未改 service/relaykit，无 staged 文件。
- HANDOFF 4.1（OpenCode Go 第三轮复核 3 项缺口）已完成（未提交）：billing header 清理收窄为仅剥离 header 区域（同行正文保留）、CCH token 按合法字符界定边界（标点后正文保留）、`CHANNEL_TYPE_CONFIGS[45]` 补齐 VolcEngine 默认 base URL 并新增 `getBaseUrlForChannelTypeChange`（默认值替换/自定义保留）；均补回归测试（adaptor_test.go 的 header/CCH 用例 + `channel-base-url-switch.test.ts`）。定向 Go 测试、前端 focused test、`bun run typecheck`、`relaykit` GOWORK=off 构建、`git diff --check` 均通过。
- 安全请求快照已完成（未提交）：默认关闭；完整字节保存、不脱敏不截断；显式稳定 secret 才可运行；HKDF-SHA256 + AES-256-GCM、AAD、原子本地文件、节点隔离；主库 metadata/audit 表；内容读取改为仅 Root 超级管理员直接访问，不可委派且不再要求 2FA/Passkey proof；成功返回正文前 audit fail-closed；no-store + critical rate limit；节点容量/retention/orphan/missing/terminal/access/tombstone 有界清理；Operations 设置与 Usage Log 按需查看/复制/下载；七 locale i18n 同步完成。独立 security/lifecycle/authz/frontend/oracle review 的 blocker/high 缺口均已修复；剩余仅为 cleanup 全节点 metadata 扫描、setting lock-free 与 secret 轮换等非阻断运行风险。Focused Go/Bun、`go build ./...`、typecheck、changed-file oxlint、relaykit 独立 build、`git diff --check` 均通过，无 staged 文件。
- 结构化缓存指标 + SQL Usage Analysis 已完成（未提交）：consume log 新增 cache read/write（含 5m/1h）和 total input 列，文本、Audio、Realtime/WSS、渠道测试写入 normalized/effective billing usage，并对负数/Int32 上界做饱和；ClickHouse 建表及幂等增列已覆盖。Root-only API 使用 SQL summary/grouped pagination/hourly trend，不加载原始日志、不解析 `other` JSON；default 24h、max 90d、15s timeout、page size max 100；summary/count/rows/trend、options 和渠道名解析均绑定 deadline，options 有结果上限且模型只扫最近 90 天；legacy rows 单独计数并从 cache-rate 计算排除。前端新增 root 路由/侧栏、日期与四维筛选、summary、趋势图和分页明细，Refresh 显式重试、分页保留旧数据、错误统一页内显示，七 locale 同步。两轮独立 SQL/auth/frontend/regression review 的 actionable findings 均已处理。Focused Go/Bun、typecheck、changed-file lint、frontend build、root build、relaykit standalone build、`git diff --check` 通过；无 staged 文件。全量 Go 仍有既有 channel-affinity 共享状态污染，HTTP/2 retry 测试在全量并发下于 Windows 偶发 socket abort、单独复跑通过。

## 2026-08-14

- 阶段 3 RPM/IP security chain 已完成实现（未提交）：Redis Lua atomic sliding-window + memory fallback、OpenAI-compatible 429/headers、downstream failure slot release；RPM 两条超限路径异步触发 review，默认持久化 `rate_limit_review_events`（enabled=`pending`、disabled=`skipped_disabled`、root 跳过），不恢复历史 auto-ban 语义。
- 新增默认关闭的 exact IPv4 blacklist setting/model/cache/API；只接受 canonical IPv4，root 豁免；登录、注册、dashboard/session、API Token 与 relay Token auth 链均接入。
- 永久禁用递增 auth version、失效缓存/Token、撤销 session、写 system security audit，并在开关启用时从 `LOG_DB` 收集 distinct historical IPv4 和 triggering IP；add/remove/collection 使用幂等写入，本地 cache 配合 Redis per-IP positive/negative cache，miss 时直接 exact 查询主库后短 TTL 回填。
- users 搜索框识别 exact IPv4 后跨 `LOG_DB` 解析 user IDs，再在主库复用过滤、排序、分页和 sensitive omit；root-only blacklist mutations 复用 manage audit；用户重新 enable 时清理 RPM/rate-limit keys。
- 增补 model/common/middleware focused regression coverage（exact match、Redis miss DB exact lookup/negative cache、历史 IP 收集、root exemption、IPv4 search、review-event persistence、bounded request snippet、multipart preservation、stream failure release/stream flag），以及 service RPM error contract tests；focused Go 验证通过。
- 完成独立 security/correctness review；其 high/medium 与相关 low findings均已修复。完整验证：frontend typecheck/build、root build、relaykit standalone build、focused Go 全绿；`go test ./...` 仅既有 service channel-affinity shared-state 与 Windows HTTP/2 GOAWAY 两类失败，逐个单测均通过；`git diff --check` 仅已知 `web/scripts/sync-i18n.mjs` LF→CRLF warning，cached diff 为空，临时 `.pi/.review/add-missing-keys.mjs` 已清理。

## 2026-08-15（阶段 4 进行中，全部未提交）

- 4.1 审查通用原语已完成：`common/review_redact.go`（凭据正则脱敏 + 递归白名单 JSON 脱敏）、`common/review_util.go`（MaskIP 部分掩码 / HashClientIP 不可逆 HMAC）、`common/review_snippet.go`（rune 安全截断、仅文本多模态排除、最近用户消息优先、最终凭据掩码，另加 gin 上下文便捷入口）；focused tests 全绿。
- 4.2 审查任务模型层已完成：`model/llm_review_task.go`（task/attempt/grace/calibration 状态机与 payload 类型）、`model/llm_review_task_db.go`（CAS 领取、活动槽位、CASE WHEN 原子合并、stale 恢复、重试/完成/跳过/取代、queue summary、calibration 验收统计与保留清理）、`model/llm_review_enqueue.go`（draft marker + 槽位 CAS 的跨实例原子入队）、`model/llm_review_metadata.go`（display/最近渠道参考元数据）；在 migrateDB 与 migrateDBFast 注册 4 张新表；新增 skip reason `skipped_disabled` 保留阶段 3 禁用用户审计语义；`RateLimitReviewEvent` 模型与注册已移除（由真实入队取代）；模型层并发/CAS/验收/保留 focused tests 全绿。
- 4.3 审查配置与客户端已完成：`setting/operation_setting/llm_review_setting.go`（默认关闭，注册 GlobalConfig，含 policy_text 与 Schema 能力状态机）；`service/llm_review_client.go`（拨号期每 IP SSRF 校验、禁重定向、2MB 响应上限、严格 JSON Schema、凭据脱敏、connection/schema 测试）、`llm_review_payload.go`（裁决校验/ShouldAutoBan/响应解析/脱敏载荷快照）、`llm_review_policy.go`（政策文本清洗与载荷回填）、`llm_review_credential.go`（HKDF+AES-256-GCM 域分离密钥加密 API Key）、`llm_review_settings.go`（SaveReviewSetting 经 UpdateOptionsBulk 落库）。SSRF 配置校验不做配置期域名解析（本机 DNS 污染不可确定），拨号期仍是强制边界；字面云元数据地址配置期即拒绝。focused tests（含 httptest 端到端、重定向拒绝、凭据脱敏）全绿。
- 4.4 Worker 已完成：`service/llm_review_worker.go`（5s 轮询、master-only、并发信号量、领取/重试/stale 恢复、人工 ban/unban 覆盖、compliant 免审、保留清理、缺政策转 uncertain）；自动封禁经可替换 `reviewAutoBanUser` seam 调用 `model.DisableUserPermanently(userID, "")`，消息通用化（不含旧站专属联系方式），root 豁免；`main.go` 启动/优雅停止；`DisableUserPermanently` 与用户 enable/disable 管理路径记录 manual ban/unban 时间戳并取消 pending 任务（best-effort，不回滚主结果）。worker 全链路 httptest 测试（compliant/violation/auto-ban seam/重试耗尽/人工覆盖/缺政策）全绿。
- 4.5 入队替换已完成：`service/llm_review_enqueue.go`（root 跳过、关闭→skipped、永久禁用→skipped_disabled 审计、免审→skipped、活动任务合并、失败不阻塞）；`service/rate_limit_error.go` 默认 hook 改为真实入队；阶段 3 对应测试迁移为新的 pending/skipped_disabled/root/grace/merge 合约。
- 4.6 管理 API 与权限已完成：`service/authz/resources_llmreview.go`（`llm_review.read`，无内置默认角色）；config/测试/清理 root-only + manage audit；任务列表/详情/重试/摘要经 AdminAuth + 权限 + 2FA/passkey proof（scope `llm_review.read`，详情读取写 manage audit）；`router/api-router.go` 接入；focused controller 测试（Key 掩码、启用门禁、schema 失效持久化、proof 拒绝矩阵）全绿。
- 4.7 前端已完成：`web/src/features/llm-review/`（api/types/lib+测试/stats/columns/table/detail drawer）、`llm-review-logs` 路由（admin+）、侧栏入口、`system-settings/security/llm-review-section.tsx`（配置/连接与严格 Schema 测试/API Key 掩码与清除/队列概览）+ registry 接入；七 locale i18n 经 add-missing-keys 脚本 + i18n:sync 同步（658 条写入），临时脚本已删除；typecheck/build/focused test 全绿。
- 4.8 输入 Token 前置限制与校准已完成：`relay/constant.IsTextTokenLimitMode`（chat/completions/responses/compact）；`rate_limit_ban_setting` 增加 `max_input_tokens`/`max_output_tokens`（默认 200000/10000，总开关仍默认关闭）；`service/token_input_calibration.go`（60s TTL 验收缓存 + fail-open，异步落样本）；`controller/token_preflight.go` 接入 Relay 管线（仅文本模式、root/白名单豁免、未过验收 fail-open，命中 429 + 异步入队 preflight 审查，不预扣费不调上游）；`service/text_quota.go` 后置校准样本 + 输入/输出超限 postflight 审查入队（白名单跳过）；focused 测试（模式表、fail-open、白名单、root、已校准模型拦截 + 异步任务落地）全绿。注意：RelayInfo 的 `*ChannelMeta` 为嵌入指针，preflight 阶段尚未初始化，读取渠道 ID 必须判空（已修复并留注释）。
- 4.9 视觉拦截（Vision Interception）已完成：
  - `relaykit/dto/user_settings.go` 新增 `UserSetting.Vision`（`UserVisionSetting`：enabled/vision_model/vision_suffix/prompt_template/phash_threshold），relaykit 独立构建通过。
  - `service/vision/vision.go`：安全边界（base64 编码上限 20MB、解码 15MB、单边 8192、总像素 20MP、URL 下载 15MB/30s、每请求最多 16 张）；`DecodeBase64Image` 先解头部再全量解码；`DownloadImageForPhash` 经 `ValidateSSRFProtectedFetchURL` + SSRF 保护 HTTP 客户端（禁重定向、限长流式读取）；`ComputePhash`/`HammingDistance`（goimagehash，新增依赖）；四级缓存：请求级 LRU（TTL 5m，写锁并发安全）→ 跨请求 LRU（TTL 10m，key 含 user/model/prompt 哈希身份，绝不跨用户泄漏）→ singleflight 合并 → pHash 环形缓存（500 条、TTL 10m、user/model/prompt 隔离）。
  - 子调用 `analyzeImageWithRelay` 走真实 Relay 管线：`CacheGetRandomSatisfiedChannel` 选渠道 → 隔离子 context（继承 id/token/group/auto-group 用户键 + 完整渠道 context，经本地 `setupVisionChannelContext` 避免 middleware↔vision 导入环）→ GenRelayInfo/InitChannelMeta/ModelMappedHelper → `ModelPriceHelper` 估价 → `PreConsumeBilling` 预扣（尊重订阅/钱包偏好，失败自动 `Refund`）→ adaptor Convert/DoRequest → 4MB 限读响应 → `PostTextConsumeQuota` 结算（配额、日志、审计全走标准路径）；上游 429/非 200 原样失败并回滚预扣；请求体关闭经 `NewOutboundJSONBody` closer。
  - `service/vision/intercept.go`：`ExtractImages` 有界递归（深度 12、总数 16）提取 OpenAI `image_url`/`input_image` 与 Claude `image`（url/base64 source）条目并生成就地替换 hook（OpenAI 变 text part、Claude 整块替换）；`clusterImages` 按 pHash 汉明距离聚类（阈值 0 关闭）；`InterceptImages` 逐簇查缓存或子调用后统一替换。
  - `middleware/vision_intercept.go`：按路径白名单（/v1/chat/completions、/v1/messages、/chat/completions、/messages）拦截；用户配置 Enabled+Suffix+VisionModel 齐全才生效；后缀匹配请求模型；重写请求体并同步刷新 `KeyBodyStorage`/`KeyRequestBody` 缓存（下游 distributor 读到新 body）；任何失败 fail-open 不改写请求；标记 `vision_intercepted`。挂载于 httpRouter `Distribute()` 之前。
  - `controller/user.go`：`UpdateUserSettingRequest.Vision` 缺省保留现有配置、携带则整体覆盖（该接口其余字段是通知类，不能顺带清空 vision 配置）。
  - 前端：`web/src/features/profile/components/vision-interception-card.tsx`（开关 + 模型/后缀/提示词模板/pHash 阈值 + 保存），注册进 profile 卡片区；`UserSettings`/`UpdateUserSettingsRequest` 增加 vision 类型；7 个 locale 新增 12 条 key（经 add-missing-keys 脚本 + i18n:sync，脚本已删）。
  - focused 测试：vision 包（base64 解码与尺寸拒绝、pHash/汉明距离、OpenAI+Claude 提取、16 张上限、就地替换、聚类、SSRF 私有地址/云元数据/file:// 拒绝、缓存隔离、失败 fail-open）与 middleware（默认关闭、缺后缀、后缀不匹配、非目标路径、分析不可用时 fail-open 保持原 body）全绿；修复 `TestRPMMiddlewareReturnsOpenAICompatible429` 的异步 review 入队与 nil DB 竞争（显式 sqlite fixture + Eventually 等待任务落库）；修复 `new-api-channel.test.ts` 因新增渠道配置导致的相邻位置断言（改为“New API 在 58 之前”的不变量）。
  - 新增依赖：`github.com/corona10/goimagehash v1.1.0`（含 nfnt/resize）。
- 验证：`go build ./...`、relaykit GOWORK=off 构建、frontend typecheck/build、`git diff --check` 通过；focused Go 测试（common/model/service/controller/middleware/service/vision/setting）与 Bun 测试全绿；`format:check` 仅剩 3 个既有上游文件（channel-form.ts、api-key-group-cell.tsx、redemption-form.ts，不在本次改动内）；`api-key-group-cell.test.tsx` 4 个失败为上游 HEAD 自带（组件中 AutoGroupBadge 帧被注释、测试仍期待 2 帧，两文件均未改动）；全量 service 包仍仅既有 channel-affinity 共享状态污染失败（单独复跑通过）。

## 2026-08-15（阶段 5 用户额外分组与渠道模型三态，全部未提交）

- 5.1 Model 层已完成：`model/user_group_grants.go`（`UserGroupGrant`，唯一 (user,group,source)，`ReplaceUserGroupGrantsWithSource` 原子替换、`ListUserGroupGrants`）；`model/channel_model_group.go`（`channel_model_group_overrides` + `channel_model_group_disabled` 两表；`ResolveChannelModelGroups`：disabled→空集、custom→覆盖行、无行→渠道默认组；`ReplaceChannelModelGroupPolicies` 全量校验替换——model 必须属于渠道公开模型、group 必须在目录（ratio_setting ∪ usable groups）、custom≥1、inherit/disabled 不带组、去重去 auto、重复 model 拒绝；`LoadChannelModelGroupModes` 回读三态）；`model/ability.go` AddAbilities/UpdateAbilities 改为 effective groups 投影（buildAbilities + 一次性建表检查）；`model/channel.go` 新增 `ModelGroupModes *[]ChannelModelGroupModeInput`（gorm:"-:all"，nil 保留、显式空替换），Insert/Update/Delete/BatchInsert/BatchDelete 全部事务化（策略替换+ability 重建+删除清理两表，均带建表守卫）；`model/main.go` migrateDB/migrateDBFast 注册 3 张新表（user_group_grants + 2 张渠道策略表）。
- 5.2 Service 已完成：`service/group_access.go`（`UserGroupAccess`、`GetLegacyGroupCatalog`、`ResolveUserGroupAccess` 事务内账户档位、`IsUserGroupAllowed`、`GetUserAutoGroupByID`/`GetAutoGroupsForUser` 保序并集）；`service/group_resolver.go`（`ResolveGroupSelection`：空 token→账户组、固定组必须在有效组（root 豁免）、requested 必须在有效组且不得 auto、`ErrRoutingGroupNotGranted/Invalid/AutoNotAllowed`）；`service/routing_group_migration.go`（只读 dry-run 报告 + 幂等迁移：可映射才导入、未知 key/token 保持不动并报告、grant 按 (user,group) 合并取最晚到期、routing-compat→manual、legacy_auto/auto 归一化；修复首条 grant 与 map 零值混淆的永久化 bug）。
- 5.3 管线已完成：`middleware/auth.go` 固定 Token 检查改 `ResolveGroupSelection`（未授权 403 + `access_denied` 稳定码，保留弃用组检查）；`middleware/distributor.go` playground 显式请求组经统一解析器；`service/group.go` `GetRequestAutoGroups`/`FilterUserTokenAutoGroups` 用有效组交集（无 id 时回退旧语义，显式空快照不回退全局）；`controller/model.go`/`pricing.go`/`token.go`(GetTokenAutoGroups)/`user.go`(GetUserModels)/`group.go`(GetUserGroups) 全部改用有效组（含授权），GetUserGroups 保留 auto 伪条目检查原 usable 表。
- 5.4 Controller 已完成：`model.User.ExtraGroupKeys *[]string`（gorm:"-:all"）+ `UpdateUser` 事务内 presence 语义（nil 保留、显式空清空、非空替换；继承组跳过、目录外 key 拒绝、事务回滚保护）+ `GetUser` 回填 manual 授权；`controller/token.go` `resolveTokenRoutingMutation`（空必填、auto 放行、固定组必须有效、固定+CrossGroupRetry 拒绝）接入 AddToken/UpdateToken（UpdateToken 先按既有语义强制 fixed 组关闭跨组重试再校验）；`channel_authz.go` 将 `model_group_modes` 归入 `channelSensitiveFields` + 精确 old-vs-new 比较（guard test 通过）；`GetChannelById` 回填 `model_group_modes`。
- 5.5 前端已完成：渠道抽屉新增 `ChannelModelGroupPolicies`（每模型 inherit/custom/disabled 三态 Select + custom 组 MultiSelect，空 custom 回落 inherit），`channel-form.ts` schema/defaults/transform/双 payload 序列化 + `pruneChannelModelGroupModes`（模型移除时清理策略）；用户抽屉 Group & Quota 区新增“模型访问分组”MultiSelect，`user-form.ts` 更新总是显式提交 `extra_group_keys`（空数组清空授权）、创建不带该字段；7 locale 新增 9 条 key（经脚本 + i18n:sync，脚本已删）。
- 5.6 回归：model 三态投影/校验/更新重建/批量生命周期/roundtrip、service resolver/access/migration 幂等与 dry-run、controller token 越权拒绝/用户 grants presence 原子性/channel 敏感比较、前端 tri-state 序列化与用户 presence 测试全绿；修复 `LoadChannelModelGroupModes` 无建表守卫导致旧测试库 `CacheGetChannel` 报 no such table 的回归、token 测试 fixture 缺 users 表与 aff_code 冲突、GetUserModels auto 分支对有效组（不含 auto）的成员检查回归。
- 验证：`go build ./...`、relaykit GOWORK=off、前端 typecheck/build、`git diff --check` 通过；model/controller/middleware/setting 全量测试绿；service 全量仅既有 channel-affinity 共享状态污染失败（单独复跑通过）；前端 features 测试仅既有 api-key-group-cell 3 个失败与 Bun node:test 嵌套 describe 噪音（均未改动文件）。
- 剩余：阶段 6 的兼容迁移管理入口/readiness 阻断（dry-run 服务已就绪）、阶段 7 完整验证与交付报告。

## 2026-08-15（阶段 6 兼容迁移、对账与回滚，全部未提交）

- 服务层：`service/routing_group_migration.go` 扩展 —— `RoutingGroupMigrationReadiness`（活跃未映射 Token/孤儿组 key 阻塞诊断，只读）；`MigrateRoutingGroupCompatibilityDataStrict`（存在阻断项时整体 fail-closed 零写入并返回报告；全部可映射时单事务幂等导入 + 写入 `routing_group_migration_version=1` 标记）；`GetRoutingGroupMigrationStatus`（标记 + 实时 dry-run 对账：pending/in_sync）。
- 启动诊断：`main.go` 在 `InitOptionMap` 后执行只读 readiness 检查，存在阻断项时输出结构化告警日志；不阻塞启动、不自动迁移（替代旧仓库的启动自动迁移行为，符合“先 dry-run、fail-closed”决策）。
- 管理 API：`controller/routing_group_migration.go` + router 挂载 —— root-only（RootAuth + CriticalRateLimit + DisableCache）`GET /preview`（只读报告）、`GET /status`（标记+readiness+blockers）、`POST /run`（严格迁移，阻断时 success:false + 报告，成功写 manage audit）。
- 文档：`docs/routing-group-migration-manual.md`（dry-run→readiness→run→对账→回滚全流程，旧表永不写入/删除，回滚仅需回退代码，SQLite/MySQL/PostgreSQL 兼容说明）。
- 测试：strict 阻断零写入、全量可映射成功+标记+幂等、readiness、controller preview 只读/run fail-closed/status 阻断报告，全绿（修复 `RoutingGroupMigrationStatus` 函数/类型同名、测试 fixture 缺 options 表与 OptionMap 未初始化的两个问题）。

## 2026-08-15（阶段 7 完整验证与交付报告，全部未提交）

- 完整 `go test ./...` 全量复跑：33 包 ok，仅 2 个基线预存失败（service channel-affinity 共享状态污染、relay/channel HTTP/2 GOAWAY Windows 偶发），均单独 `-count=1` 复跑通过。
- 修复全量运行暴露的阶段 4 测试债：`service/authz.TestSetUserPermissionsStoresOnlyOverrides` 期望快照未包含阶段 4 新增的 `llm_review` 资源（新增资源归一化为全 false），已更新两处期望。
- 交付报告：`docs/customization-migration-report.md`（范围/验证矩阵/安全摘要/基线预存问题/部署启用指引/回滚/待确认事项）。
- 收尾检查：`git diff --check` 0 错误；无临时脚本与 `.pi`/`.review` 残留；工作树 184 个文件全部未提交、未推送、未部署。

## 2026-08-14 20:48 +08:00（提交与 ssh2 部署）

- 产品代码、测试、依赖、前端与 i18n 已提交：`da678d51`（`feat: rebuild customizations on latest upstream`），未 push。
- 从该提交生成源码归档并上传 `ssh2:/opt/newapi`，构建镜像 `newapi-custom:20260814-remediated-da678d51`。
- 部署前通过 SQLite 在线 `.backup` 生成一致性备份，`PRAGMA integrity_check` 返回 `ok`；compose、`.env`、旧源码和旧镜像信息同时保存在 `/opt/newapi/backups/deploy-20260814-204512-da678d51`。
- `newapi` 容器已切换到新镜像，运行状态正常、重启次数 0，`GET /api/status` 返回 HTTP 200。
- 启动 migration 成功；routing-group readiness 检测到旧分组 key `渠道1` 无法映射，因此严格迁移未自动执行，旧表保持只读。
- 启动日志另提示 `SESSION_COOKIE_SECURE` 未开启、`TRUSTED_PROXIES` 使用兼容默认值；待结合现有 Caddy 配置确认。

## 2026-08-15 04:25 +08:00（可观测性 UX 提交、推送与 ssh2 部署）

- 已将 Root-only 请求体直读、独立 Request Snapshots 设置入口、恢复版 Usage Analysis、回归测试和七语言翻译提交为 `bd8b8746`（`feat: restore admin observability UX`）。
- 已推送并建立跟踪：`origin/rebuild/customizations-20260812`。
- 从精确提交构建镜像 `newapi-custom:20260815-observability-bd8b8746`，镜像 ID：`sha256:bc3a6c51743b88aca6909bbb9d66a063eda666e2331752b7ca4dfd7cf786a794`。
- 部署前备份：`/opt/newapi/backups/deploy-20260815-042500-bd8b8746`；SQLite 在线备份 `PRAGMA integrity_check` 返回 `ok`，同时保留 Compose、`.env`、旧源码和旧镜像元数据。
- `newapi` 容器切换后运行正常、重启次数 0，`/api/status` 与首页均返回 HTTP 200；未修改环境变量，未执行严格 routing-group 迁移。

## 2026-08-15 14:56 +08:00（安全证明放宽与 data-assets 恢复提交、推送与 ssh2 部署）

- 已将渠道 Key、LLM Review 管理操作、Passkey 管理中的强制二次验证/Passkey proof 移除；保留既有认证、权限、Root/Admin 边界、限流、审计、WebAuthn 流程绑定与会话身份校验。产品提交为 `9d828d74`（`fix: relax security proofs and restore data assets`）。
- 已推送至 `origin/rebuild/customizations-20260812`；从该精确提交构建镜像 `newapi-custom:20260815-security-avatar-9d828d74`，镜像 ID 为 `sha256:d2e56d70f7bf4c8921b2c69a87b776e4d03c062422753b87825984214accd891`。
- 部署前备份：`/opt/newapi/backups/deploy-20260815-145129-9d828d74`；SQLite 在线备份 `PRAGMA integrity_check` 返回 `ok`，并保留 Compose、`.env`、旧源码和旧镜像元数据。
- `newapi` 容器切换后运行正常、重启次数 0，`/api/status` 与首页均返回 HTTP 200；未修改环境变量，未执行严格 routing-group 迁移。
- 已恢复 `https://newapi.uwoacrimson.com/data-assets/anon-removebg-preview.png`：HTTP 200、`image/png`、344251 bytes，响应与 `/opt/newapi/data/anon-removebg-preview.png` SHA-256 一致；HEAD 为 200，缺失文件、POST、路径穿越均为 404。

## 当前阶段

- 阶段 1–7、独立 Review、Remediation、产品提交、推送和 `ssh2` 部署均已完成。
- 下一步：处理 routing-group blocker `渠道1` 后执行严格迁移；按需确认 cookie/proxy 生产配置。
- 当前分支已 push；未创建 PR，未向 `upstream` 推送。

## 2026-08-22 00:13 +08:00（渠道模型固定端点功能开发与 ssh2 部署）

- 新功能：渠道 -> 编辑 -> 模型分组策略 下方为每个模型新增「固定端点」输入。模型配置固定端点后，relay 请求该模型时若渠道有效 base URL 与固定端点不一致直接拒绝（403，skip-retry），覆盖普通分发、重试、渠道测试、任务重放、vision 子调用全部路径。
- 实现：新表 `channel_model_fixed_endpoints`（channel_id+model 主键、endpoint，三库兼容 AutoMigrate + migrateDBFast 双注册）；Channel 新增 `model_fixed_endpoints` 传输字段，保存校验（模型必须已发布、http/https URL），删除渠道联动清理；内存索引（InitChannelCache 全量 + CacheUpdateChannel 单渠道刷新，DB 查询兜底）；authz 归入 sensitive 字段；relaykit 新增 `fixed_endpoint_mismatch` 错误码（不带 channel: 前缀避免触发重试）；前端 7 locale + prune + 单测。
- 验证：受动包 go test 全绿（仅 2 个预存基线失败：HTTP/2 GOAWAY Windows 偶发、service affinity 共享状态，单跑均通过）；relaykit GOWORK=off 构建通过；前端 typecheck/lint/build/20 个测试通过；git diff --check 通过。
- 提交：`7f3e5f24`（`feat: pin channel models to fixed endpoints`），未 push。
- 部署：镜像 `newapi-custom:20260822-fixed-endpoint-7f3e5f24`（ID b4f93ef70360）已切换至 ssh2 生产 `newapi` 容器，Restarts=0，`channel_model_fixed_endpoints` 表已建，`/api/status` 与首页 200。部署前 SQLite 在线备份（`one-api.db`，integrity ok）位于 `/opt/newapi/backups/deploy-20260822-001025-fixed-endpoint-7f3e5f24`（含 compose、.env、旧镜像元数据）。

## 2026-08-22 00:20 +08:00（修复固定端点输入框无法输入并重新部署）

- 问题：固定端点输入框无法输入文字。原因：drawer 用 `form.getValues('model_fixed_endpoints')` 作为受控值，`getValues` 不触发订阅，`setValue` 后组件不重渲染，受控 Input 每次输入都被重置。
- 修复：改为组件顶层 `form.watch('model_fixed_endpoints')`（watch 订阅变化，setValue 会触发重渲染），编辑回填路径已有 `transformChannelToFormDefaults` 覆盖。
- 提交：`3c9d0579`（`fix: subscribe fixed endpoint field so the input stays editable`），未 push。
- 部署：镜像 `newapi-custom:20260822-fixed-endpoint-3c9d0579`（ID b6cc0db2f12f）已切换，Restarts=0，`/api/status` 与首页 200；部署前备份 `/opt/newapi/backups/deploy-20260822-0019xx-fixed-endpoint-3c9d0579`（one-api.db integrity ok）。
