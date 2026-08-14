# New API 定制重建计划

## 目标

以 `awoaCrim/public-api` 的最新上游同步状态为新基线，重新识别并应用旧仓库 `E:\myCode\myapi` 中仍有价值的定制功能，同时修复外部审查指出的权限、迁移、事务、前端错误处理和性能问题。

## 固定基线

- 旧仓库：`E:\myCode\myapi`，只读保留，不覆盖、不重写历史。
- 新仓库：`E:\myCode\public-api`。
- 新仓库基线：`ccd535ef8e50cf6e5846a59278c40b7ff59d1b7d`。
- 工作分支：`rebuild/customizations-20260812`。
- `origin`：`https://github.com/awoaCrim/public-api.git`。
- `upstream`：`https://github.com/QuantumNous/new-api.git`。
- 本轮不操作生产环境，不提交、不推送，除非用户另行确认。

## 已确认产品决策

1. 先生成并确认功能级定制清单，再迁移业务实现。
2. 上游已经具备的功能直接采用上游版本，不重复移植旧实现。
3. 用户可以在账户主分组之外获得额外原有分组权限。
4. 固定分组 Token 只能使用其固定分组；请求其他分组必须拒绝。
5. Auto Token 保留最新版上游的每 Token 候选组能力，实际候选组必须与用户当前有效权限取交集。
6. 渠道模型分组使用 `inherit`、`custom`、`disabled` 三态。
7. 旧 `routing_groups` 平行模型不恢复为运行时权威，只允许受控、可审计的兼容迁移读取。
8. 兼容迁移必须先 dry-run，存在活跃未映射 Token、授权或 canonical key 冲突时 fail-closed。
9. 按正式清单迁移全部建议功能。
10. 请求体快照仍保存完整内容，不做脱敏或截断；同时必须默认关闭，并具备专用敏感权限、访问审计、加密/受控存储、容量限制和保留期清理。
11. RPM 采用旧仓库最终代码语义：原子滑动窗口、OpenAI 兼容 429、失败请求释放槽位并触发 LLM Review；不恢复早期自动短期/长期封禁。

## 阶段

### 阶段 1：建立新基线并盘点定制

状态：`completed_pending_user_confirmation`

- 核验新 fork、远程、基线提交和工作分支。
- 对照旧仓库实际代码、变更记录和最新版上游实现。
- 生成正式定制清单，区分：上游已有、部分重叠、本地独有、需重写、仅文档/运维记录。
- 记录高冲突路径、外部依赖、数据库模型、前端入口和测试资产。
- 在修改业务代码前建立后端和前端基线验证结果。
- 正式清单：`docs/customization-migration-inventory.md`。

### 阶段 2：低冲突独立功能

状态：`completed`

- [x] 实现默认关闭、完整内容、带敏感权限与访问审计、受控存储和保留期限制的请求快照。
- [x] OpenCode Go 渠道，重新分配不冲突的渠道类型并适配最新版 relaykit。
- [x] OpenAI-compatible 流式/非流式 usage/cache 归一化。
- [x] 缓存指标改为结构化日志字段和 SQL 聚合，并实现 Usage Analysis。

### 阶段 3：安全与封禁链路

状态：`completed`

- [x] RPM 原子滑动窗口、OpenAI 兼容 429、失败请求释放槽位，以及异步审查触发；`rate_limit_logs` 保持 historical-only，不恢复历史短期/长期自动封禁语义。
- [x] 当前上游没有 LLM Review worker，因此默认触发器持久化 `rate_limit_review_events`：enabled 用户为 `pending`，disabled 用户为 `skipped_disabled`，root 跳过；同时保留可替换 hook。
- [x] 默认关闭的精确 IPv4 黑名单、永久禁用时历史 IP 与触发 IP 收集、users exact IPv4 搜索。
- [x] 登录、注册、dashboard/session、API Token 和 relay Token 认证链接入，root 豁免，缓存/Token/session 失效，重新启用时清理 rate-limit keys。
- [x] root-only audited blacklist API、本地 + Redis per-IP positive/negative cache（miss exact 查主库后短 TTL 回填），以及 focused regression tests。
- [x] 完整验证矩阵与独立 review；review 的 high/medium 与相关 low findings（multi-node cache poisoning、multipart body preservation、stream failure release、register denial、audit、snippet allocation、actual memory count/stream flag）均已处理。

### 阶段 4：Vision 与 LLM Review

状态：`completed`（4.1–4.9 全部完成，未提交；剩余收尾验证与可选后续阶段见 HANDOFF）

- 将 Vision 图片转文字、pHash 去重和用户配置接入最新版请求管线。
- 迁移 LLM Review 的任务模型、worker、管理 API 和前端入口，并接入输入 Token 前置限制与估算器校准。
- 避免复制上游已存在的 Playground、富文本和协议转换实现。

子批次与关键决策（2026-08-15 侦察确认）：

- 4.1 审查通用原语（common）：`MaskReviewCredentialText`、`RedactReviewJSON`、`MaskIP`、`HashClientIP`。
  沿用旧语义（凭据正则脱敏、递归白名单 JSON 脱敏、HMAC-SHA256(CryptoSecret) 不可逆 IP 哈希），
  但使用当前 `common.Marshal/Unmarshal`、`HmacSha256Raw`、`NormalizeIPv4`；MaskIP 的 IPv6 输出改用 8 组展开后前 4 组掩码，避免旧压缩形式产生 `:::` 拼接。
- 4.2 审查任务模型层：`model/llm_review_task.go` + `llm_review_task_db.go` + `llm_review_enqueue.go`，
  在 `model/main.go` 的 migrateDB / migrateDBFast 注册 4 张表（task/attempt/grace/calibration）。
  沿用旧状态机与跨库原子语义（draft marker + 活动槽位 CAS、CASE WHEN 合并、stale 恢复、重试），
  并新增 skip reason `skipped_disabled` 以保留阶段 3 的禁用用户审计语义；移除 `RateLimitReviewEvent` 持久化模型（被 4.5 的真实入队取代）。
- 4.3 审查配置与客户端：`setting/operation_setting/llm_review_setting.go`（默认关闭，注册 GlobalConfig），
  `service/llm_review_client.go`（SSRF 拨号校验 + 禁重定向 + 严格 JSON Schema + 2MB 响应上限 + 凭据脱敏），
  `service/llm_review_payload.go`（裁决校验/ShouldAutoBan/响应解析），
  `service/llm_review_credential.go`（HKDF-AES-256-GCM 域分离密钥派生自 CryptoSecret），
  `service/llm_review_policy.go`（政策文本来自 `llm_review_setting.policy_text`，空政策→uncertain；不做站点专属文案）。
- 4.4 Worker：`service/llm_review_worker.go`（5s 轮询、master-only、并发信号量、CAS 领取、固定重试间隔、stale 恢复、
  人工封禁/解封覆盖、compliant 免审、保留期清理）；自动封禁用当前 `model.DisableUserPermanently(userID, "")` 与通用文案，root 豁免。
  `main.go` 启动/优雅停止；`DisableUserPermanently`/用户重新启用路径记录 manual ban/unban 时间戳。
- 4.5 入队替换：`service/llm_review_enqueue.go` 实现 `EnqueueLLMReview`（root 跳过、永久禁用→skipped_disabled、免审→skipped、
  活动任务合并、失败不阻塞 429）；替换 `service/rate_limit_error.go` 默认 hook；更新阶段 3 相应测试。
- 4.6 管理 API 与权限：`service/authz/resources_llmreview.go`（`llm_review.read`，无内置默认角色），
  配置/测试/清理接口 root-only + manage audit，任务列表/详情/重试/摘要要求 AdminAuth + 权限 + 2FA/passkey proof（scope `llm_review.read`），
  访问审计复用 manage audit 记录。
- 4.7 前端：`web/src/features/llm-review/`（api/types/列表/详情/统计/格式化）、路由与侧栏入口、
  `system-settings/security` 增加 LLM Review 配置分区（含 connection/schema 测试与 API Key 掩码/清除）、七 locale i18n。
- 4.8 输入 Token 前置限制与校准：扩展 `rate_limit_ban_setting` 增加 `max_input_tokens`/`max_output_tokens`（沿用旧默认 200000/10000，开关仍默认关闭），
  `service/token_input_calibration.go`（60s TTL 验收缓存 + fail-open），前置检查接入当前文本 relay 管线（仅文本模式、root 豁免、白名单豁免、
  未过验收 fail-open），命中时异步入队审查并返回 429；校准样本在 usage 已知的结算路径异步落库（estimate 来自 `EstimateTokenByModel`，actual 取 prompt tokens）。
- 4.9 Vision：`relaykit/dto/user_settings.go` 增加 Vision 用户配置（relaykit 独立模块约束）；
  `middleware/vision_intercept.go` + `service/vision/vision.go` 安全重写：默认关闭、仅 OpenAI/Claude 文本端点、
  typed JSON（common JSON wrapper）替换 gjson/sjson 路径改写、SSRF 安全下载（复用 GetSSRFProtectedHTTPClient + 尺寸/像素/下载上限）、
  pHash 去重（goimagehash，缓存 key 隔离用户/模型/prompt）、子调用沿用原 Token/分组与计费会话、失败 502、旧图无缓存占位符不调 API；
  用户配置 UI 迁入 `web/src/features/profile/` 并 i18n。
- 4.9 实施偏差记录（已按当前架构收敛）：失败语义改为 **fail-open**（任何子调用/分析失败不改写请求体、原样放行），
  并新增每请求 16 张图上限；旧仓库“失败 502/占位符不调 API”在重建版不适用（当前管线无占位符概念）；
  子调用结算走标准 `PostTextConsumeQuota` 路径（配额/日志/审计与普通请求一致），失败自动回滚预扣；
  渠道 context 由本地 `setupVisionChannelContext` 设置（避免 middleware↔vision 导入环）。
- 每子批次：先写 failing test（testify），聚焦实现，更新 progress/HANDOFF，跑聚焦测试 + root/relaykit/前端构建 + `git diff --check`。
- 说明：审查政策文本不再依赖站点首页（当前仓库无 HomePageContent），改为 `llm_review_setting.policy_text` 显式配置；自动封禁文案通用化，不含旧站专属联系方式。

### 阶段 5：用户额外分组和渠道模型策略

状态：`completed`（2026-08-15 完成 5.1–5.6；参考旧仓库最终实现 `user_group_grants` + `channel_model_group_overrides` 统一原有分组模型，并补齐 disabled 三态；兼容迁移的管理端入口/readiness 归入阶段 6）

- 新增用户额外原有分组授权和版本化缓存。
- 固定 Token、Auto Token 候选子集和显式请求分组采用统一解析器。
- 用户更新 DTO 区分字段省略与显式空数组。
- 渠道模型分组实现 `inherit`、`custom`、`disabled` 三态。
- 创建、更新、复制、删除、ability 和缓存重建使用完整事务与统一验证。
- 将模型分组策略归类为敏感渠道写权限。

子批次（每批先写 failing test，再实现）：

- 5.1 Model 层：`model/user_group_grants.go`（`UserGroupGrant` + 迁移注册）；
  `model/channel_model_group.go`（`ChannelModelGroupOverride` + `ChannelModelGroupDisabled` 两张表、
  `ResolveChannelModelGroups`（disabled→空集、custom→覆盖行、无行→inherit 渠道默认组）、
  `ReplaceChannelModelGroupPolicies` 全量校验替换（model 必须属于渠道公开模型、group key 必须在目录、custom≥1、
  inherit/disabled 不遗留行、去重去 auto）、`LoadChannelModelGroupModes` 回读）；
  `model/channel.go` 增加 `ModelGroupModes *[]ChannelModelGroupModeInput`（`gorm:"-:all"`），
  Insert/Update/Delete/BatchInsert/BatchDelete 全部事务化（策略替换 + ability 重建 + 删除时清理两张表）；
  `model/ability.go` 的 AddAbilities/UpdateAbilities 改用 effective groups 投影；目录校验用 `ratio_setting` ∪ `setting`（无环）。
- 5.2 Service：`service/group_access.go`（`UserGroupAccess`、`GetUserEffectiveGroups`、`IsUserGroupAllowed`、
  `GetUserAutoGroupByID`/`GetAutoGroupsForUser` 保序并集、`GetLegacyGroupCatalog`）；
  `service/group_resolver.go`（`ResolveGroupSelection`：空 token→账户组、固定组必须在有效组（root 豁免）、
  requested 组必须在有效组且不得为 auto、`UsingGroup`=requested??token）；
  `service/routing_group_migration.go`（只读旧 `routing_groups`/`user_routing_group_grants`/`tokens` 表，
  fail-closed dry-run 报告 + 幂等迁移：可映射才导入、未知 key 不改动、legacy_auto/auto 归一化）。
- 5.3 管线：`middleware/auth.go` 固定 Token 检查改为 `ResolveGroupSelection`（未授权 403 稳定码，保留弃用组检查）；
  `middleware/distributor.go` 请求分组解析；`service/channel_select.go`/`GetRequestAutoGroups` 用有效组交集（显式空快照不回退全局）；
  `controller/model.go`/`controller/pricing.go` 的 auto 列表用 `GetUserAutoGroupByID`；`controller/group.go` `GetUserGroups` 返回有效组（含授权）。
- 5.4 Controller：`UpdateUser` 增加 `ExtraGroupKeys *[]string`（指针区分省略/显式空数组：nil 保留、空清空、非空替换，
  事务内校验目录并跳过继承组）；`controller/token.go` `resolveTokenRoutingMutation`（空必填、auto 放行、固定组必须有效、
  固定+CrossGroupRetry 拒绝）接入新增/编辑；`channel_authz.go` 把 `model_group_modes` 归入 `channelSensitiveFields` 并加精确比较；
  `GetChannelById` 回填 `model_group_modes`。
- 5.5 前端：用户抽屉“模型访问分组”多选（继承只读展示 + 手动授权编辑，`extra_group_keys` 总是显式提交）；
  渠道抽屉模型分组三态（inherit/custom/disabled + custom 组多选，提交展开为 `model_group_modes`）；
  Token 组下拉自然受有效组限制（后端已拒）；七 locale i18n。
- 5.6 回归：model 三态投影/事务/生命周期、service group_access/resolver/migration、controller token 越权/用户 DTO presence、
  middleware 固定 Token 撤销即时拒绝、前端序列化；定向测试 + root/relaykit/前端构建 + `git diff --check`。

### 阶段 6：兼容迁移、对账和回滚

状态：`completed`（2026-08-15；管理入口/readiness/标记/手册完成）

- 只读 dry-run 报告与 canonical key 映射：`service.PreviewRoutingGroupMigration`（mapped/unmappable groups、grant_imports、token_updates、unmappable_tokens），旧表永不写入。
- 结构化阻断报告与 readiness：`service.RoutingGroupMigrationReadiness`（活跃未映射 Token/孤儿组 key → blockers）；服务启动时只读诊断告警（不阻塞启动、不自动迁移）。
- fail-closed 写入：`service.MigrateRoutingGroupCompatibilityDataStrict` —— 存在阻断项时整体拒绝并返回报告；全部可映射时单事务幂等导入 + 写入 `routing_group_migration_version` 标记；旧表只读。
- 对账：`service.GetRoutingGroupMigrationStatus`（标记 + 实时 dry-run：pending grants/tokens、unmappable、in_sync）。
- root-only API（RootAuth + CriticalRateLimit + DisableCache + manage audit）：`GET /api/routing-group-migration/preview`、`/status`、`POST /run`。
- 回滚手册与迁移手册：`docs/routing-group-migration-manual.md`（dry-run→readiness→run→对账→回滚，SQLite/MySQL/PostgreSQL 兼容说明）。
- 测试：strict 阻断零写入、全量可映射成功 + 标记、幂等重跑、readiness、preview 只读、status 阻断报告。

### 阶段 7：完整验证和交付

状态：`completed`（2026-08-15；验证矩阵与交付报告完成）

- 定向 Go 测试与完整 `go test ./...`：仅 2 个基线预存失败（channel-affinity 共享状态污染、Windows HTTP/2 GOAWAY 偶发），均单独复跑通过。
- `relaykit` 独立构建通过；前端 typecheck/build 通过；`git diff --check` 通过；七 locale i18n 同步 0 missing/extras/untranslated。
- 权限边界（敏感字段 fail-closed 守卫、专用权限、root-only API）、迁移失败（strict 阻断零写入）、事务回滚、缓存失效均有回归测试。
- 密钥/依赖/许可证：无新密钥；仅新增 MIT 许可依赖 goimagehash + nfnt/resize。
- 交付物：`docs/customization-migration-inventory.md`（清单）、`docs/routing-group-migration-manual.md`（迁移/回滚手册）、`docs/customization-migration-report.md`（本阶段交付报告）。
- 修复了全量运行暴露的 `service/authz` 测试债：阶段 4 新增的 `llm_review` 资源需反映在权限快照期望中。

## 上线阻断条件

- 固定 Token 可以切换到其他用户分组。
- 用户更新字段省略会清空额外授权。
- 分组数据加载失败时前端仍允许保存默认值。
- 存在活跃未映射旧 Token 或授权，但服务仍进入 ready 状态。
- 模型分组策略可写入不存在的模型或分组。
- 渠道删除、策略替换或 ability 重建不是原子操作。
- 缓存统计仍会把查询区间内全部日志加载到 Go 内存。
- 任一受影响数据库方言迁移失败。

## 环境约束

- 当前宿主机可用 Bun `1.3.10`。
- 当前宿主机 PATH 中没有系统 Go；已在仓库外准备便携 Go `1.26.1`：`E:\myCode\.tools\go1.26.1\go\bin\go.exe`。
- 最新上游前端全量 lint 和 format 检查在未修改业务代码时已有既存失败；后续要求受影响文件全部通过，并保证不增加新的全量失败。
- 最新上游完整 `go test ./...` 在未修改业务代码时存在一个可隔离复现为测试共享状态污染的失败；单独复跑该测试通过。
