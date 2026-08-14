# 定制功能迁移清单

## 1. 文档目的

本文以最新上游基线 `ccd535ef8e50cf6e5846a59278c40b7ff59d1b7d` 为准，按功能目标盘点旧仓库 `E:\myCode\myapi` 中的定制，并决定每项应当：

- 直接采用最新版上游实现；
- 在最新版架构上重建；
- 修复安全或数据一致性问题后重建；
- 仅保留迁移兼容读取；
- 不迁移。

旧仓库首个可见提交是整仓快照，因此本清单不以“旧文件是否不同”作为唯一依据，而是综合使用：

- 旧仓库实际代码；
- 旧仓库本地变更记录和设计文档；
- 旧提交是否存在于官方上游历史；
- 最新上游当前实现；
- 外部审查指出的问题及其代码核验结果。

## 2. 状态说明

| 状态 | 含义 |
|---|---|
| 上游保留 | 最新上游已经提供，应直接使用，不复制旧实现 |
| 重建 | 最新上游没有，需要按当前架构重新实现 |
| 安全重写 | 功能有保留价值，但旧实现存在安全、事务或性能风险，禁止原样迁移 |
| 兼容迁移 | 不恢复旧运行时模型，只提供受控数据读取、对账和一次性迁移 |
| 不迁移 | 过期实现、过程文件、部署记录或已被新架构淘汰的内容 |

优先级定义：

- P0：权限、认证、数据迁移或生产正确性阻断项；
- P1：核心业务定制；
- P2：可独立交付的增强；
- P3：低价值兼容或可延后事项。

## 3. 来源核验结论

旧变更记录列出的以下提交存在于官方上游历史，因此对应功能不是需要重新移植的本地独有实现：

```text
4aee5f7d5  better admin permissions
df44a75d5  ClickHouse log LIKE compatibility
966af88ec  Playground and Markdown experience
df5ba9fa5  password validation copy
0b48ad86d  HTML and Markdown rendering
626dadb55  rich-content security
3a506f50f  Chat-to-Responses compatibility
2d5a04163  Responses-to-Chat compatibility
1d166532f  channel component layout
25f998595  channel management UI
43591fba7  advanced custom route editor
```

从 Vision 开始的以下提交不在当前官方上游历史，是本地定制的重要来源：

```text
73567538d  Vision interception
2866647c9  Vision pHash deduplication
07efa5983  request body download
6a741f935  RPM ban evolution
fbd63180a  IP blacklist and related controls
cc9bd7af2  OpenCode Go
8eaa0d0d7  attempted Vision plugin extraction
3d07afdfc  OpenCode Go history-system handling
```

注意：旧记录声称 Vision 已迁移到 `plugin/`，但旧仓库当前工作区并不存在该目录，实际代码仍位于 `middleware/vision_intercept.go` 和 `service/vision/vision.go`。迁移时以实际代码行为为准，不以未落地的历史说明为准。

## 4. 直接采用最新版上游

### 4.1 管理员细粒度权限

- 状态：上游保留
- 优先级：P0
- 最新接入点：`service/authz/`、`router/channel-router.go`、`router/authz-router.go`
- 决策：不复制旧 `authz` 文件。
- 迁移要求：新增渠道模型策略、请求体日志、IP 黑名单和 LLM Review 管理 API 时，必须接入最新版权限目录。
- 特别要求：渠道模型分组策略属于 `authz.ChannelSensitiveWrite`，不能仅使用普通渠道写权限。

### 4.2 Playground、Markdown 和富文本安全

- 状态：上游保留
- 优先级：P2
- 最新接入点：`web/src/features/playground/`、`web/src/components/ai-elements/`
- 决策：不迁移旧 `web/default` 或 `web/classic` 组件。

### 4.3 OpenAI Responses、Chat 和 Claude 协议转换

- 状态：上游保留
- 优先级：P0
- 最新接入点：`relaykit/relayconvert/`、`relaykit/dto/`、最新版 relay handlers
- 决策：不迁移旧 `service/relayconvert`、旧 Claude helper 或旧 DTO 快照。
- 迁移要求：OpenCode Go、Vision 和 usage 归一化必须适配最新版 `relaykit`，且 `relaykit` 保持可使用 `GOWORK=off` 独立构建。

### 4.4 每 Token Auto 候选组

- 状态：上游保留并扩展
- 优先级：P0
- 最新接入点：`model/token.go`、`service/group.go`、`middleware/auth.go`
- 已有行为：Token 可保存有序 Auto 候选组快照；没有快照时继承全局 Auto 列表。
- 决策：保留最新版行为，仅把“用户可用组”的判断扩展为主分组权限加额外授权的有效集合。

### 4.5 签到、渠道 UI、Advanced Custom、ClickHouse 基础兼容

- 状态：上游保留
- 优先级：P2
- 决策：不复制旧实现；仅在本地功能接入时补充必要扩展。

## 5. 已确认必须重建的权限与路由功能

### 5.1 用户额外原有分组授权

- 状态：安全重写
- 优先级：P0
- 用户决策：已确认
- 目标：用户保留唯一账户主分组，同时可以获得多个额外原有分组权限。
- 建议数据模型：

```text
user_group_grants(
  user_id,
  group_key,
  source,
  expires_at,
  sort_order
)
```

- 权威目录：继续使用原有 New API 分组配置，不恢复独立 `routing_groups` 作为运行时权威。
- 有效权限：主分组继承规则与当前未过期额外授权的并集；不存在于当前分组目录的 key 不生效。
- 缓存：使用版本化用户授权缓存；变更授权后必须立即失效用户缓存、Token 相关缓存和模型列表结果。
- 更新接口：使用专用 DTO，`extra_group_keys` 必须区分字段省略、显式空数组和显式替换。
- 测试重点：过期授权、授权撤销、目录删除、缓存失效、root 语义和跨数据库唯一约束。

### 5.2 固定 Token 权限边界

- 状态：安全重写
- 优先级：P0
- 用户决策：已确认
- 目标语义：固定 Token 只能使用其固定组。
- 允许：请求未指定组，或显式指定与 Token 固定组相同的组。
- 拒绝：请求指定 `auto` 或任何不同分组，即使该分组也属于用户。
- 错误协议：返回 OpenAI 兼容 403，不静默改写或回退。
- 实现要求：认证、模型列表、渠道选择、重试和计费使用同一个解析结果，避免各层自行推导。

### 5.3 Auto Token 与用户权限取交集

- 状态：重建扩展
- 优先级：P0
- 用户决策：已确认
- 目标语义：
  1. 无 Token 快照时使用全局 Auto 列表；
  2. 有 Token 快照时使用该有序子集；
  3. 最终候选始终与用户当前有效分组权限取交集；
  4. 授权撤销后立即从候选中消失；
  5. 空交集明确拒绝，不回退到用户主分组。

### 5.4 用户与 Token 编辑表单的加载安全

- 状态：安全重写
- 优先级：P0
- 最新前端位置：`web/src/features/users/`、`web/src/features/keys/`
- 旧问题：加载错误被吞掉，默认空数组或 fallback 组可能覆盖未知旧值。
- 目标行为：
  - 加载失败时显示错误和重试入口；
  - 数据未成功加载前禁止提交；
  - 当前值不在最新选项中时保留并标记为不可用，不自动替换；
  - Auto 能力和可选组由后端返回，不由前端硬编码；
  - 所有用户可见文案接入 i18n。

### 5.5 渠道模型分组三态

- 状态：安全重写（2026-08-15 已完成，未提交）
- 优先级：P0
- 用户决策：已确认
- 三态：
  - `inherit`：继承 `channels.group`；
  - `custom`：只发布到模型专属分组集合；
  - `disabled`：模型保留在渠道模型列表，但不发布任何 ability。
- 校验：
  - model 必须属于渠道公开模型；
  - group key 必须存在于当前分组目录；
  - `custom` 至少包含一个有效组；
  - `inherit` 和 `disabled` 不允许遗留 custom rows；
  - 重复 model/group 输入必须归一化或拒绝。
- 原子性：渠道创建、更新、复制、删除、策略替换、ability 重建和缓存刷新必须形成统一事务/提交后刷新流程。
- 最新上游注意事项：`Channel.Insert()`、`Channel.Update()` 和 `Channel.Delete()` 当前并非覆盖所有附属配置的单一事务，不能在其后追加零散写入。
- 缓存注意事项：最新版 `InitChannelCache()` 当前根据 `channels.group × channels.models` 构建内存索引；三态策略上线后必须改为与 ability 投影一致的来源，避免数据库 ability 与内存缓存分叉。
- 当前实现（2026-08-15）：
  - 存储：`channel_model_group_overrides(channel_id, model, group_key, sort_order)`（custom 行）+ `channel_model_group_disabled(channel_id, model)`（禁用标记）；inherit 不落任何行；
  - 投影：`ResolveChannelModelGroups`（disabled→空集、custom→覆盖行、无行→渠道默认组）驱动 `AddAbilities`/`UpdateAbilities`，`disabled` 模型保留在渠道模型列表但发布零 ability；
  - 校验：model 必须属于渠道公开模型；group key 必须在当前分组目录（ratio_setting ∪ usable groups）；custom≥1 组；inherit/disabled 不带组；重复 model 拒绝；去重去 auto；
  - 原子性：`Channel.Insert/Update/Delete` 与 `BatchInsertChannels/BatchDeleteChannels` 全部单事务（策略替换 + ability 重建 + 删除时清理两表），删除能力无需建表守卫（旧库安全）；
  - 权限：`model_group_modes` 已归入 `channelSensitiveFields`（精确 old-vs-new 比较，`TestChannelFieldsAreClassified` 守卫）；
  - 回读：`GetChannelById` 回填 `model_group_modes` 三态 DTO；前端渠道抽屉提供 per-model 三态编辑与序列化测试；
  - 用户额外分组与解析器（5.1–5.4）：`user_group_grants` 授权、`ResolveGroupSelection` 统一解析器、固定 Token 创建/更新校验、UpdateUser `extra_group_keys` presence 语义、Auto 候选与有效组交集、旧 `routing_groups` 只读 dry-run + 幂等 fail-closed 迁移（见 progress.md 阶段 5）。

## 6. 本地独有、建议重建的业务功能

### 6.1 Usage Analysis 与缓存指标

- 状态：安全重写（已完成，未提交）
- 优先级：P1
- 推荐：迁移
- 旧入口：`controller/usage_analysis.go`、`web/default/src/features/usage-analysis/`
- 上游重叠：最新版已经能识别多种 provider cache usage，并在计费日志 metadata 中保存部分缓存信息，但没有旧版 Usage Analysis 管理页面和聚合 API。
- 必须修复：旧实现会读取时间范围内全部原始日志，在 Go 内存逐条解析 `other`。
- 新设计：
  - 日志写入时保存结构化缓存指标，例如 cache read、cache write、5m/1h creation；
  - 查询使用 SQL `SUM()` / `GROUP BY`；
  - 支持独立日志库和现有 ClickHouse 能力；
  - 设置最大查询区间、分页/结果上限和查询超时；
  - 保留现有 API 所需的渠道名称、模型、用户和 Token 维度；
  - 对历史仅存在于 metadata 的记录提供受限兼容统计或一次性回填工具，不在在线查询中全量解析。
- 验收：使用大数据量 fixture 证明内存占用不随原始日志行数线性增长。
- 当前实现（2026-08-13）：
  - consume log 新增 `cache_read_tokens`、`cache_write_tokens`、`cache_write_tokens_5m`、`cache_write_tokens_1h`、`input_tokens_total` 结构化列；文本、音频、Realtime/WSS 和渠道测试日志在写入时从 normalized/effective billing usage 填充，并将负值和超过 Int32 的指标饱和到持久化边界；Midjourney、任务计费与违规费等无 Token usage 的日志保留为 legacy rows；
  - SQLite/MySQL/PostgreSQL 使用 GORM migration；ClickHouse 建表 SQL 和幂等 `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` 同步支持；
  - root-only `/api/usage-analysis` 与 `/options` 使用 `RootAuth + CriticalRateLimit + DisableCache`；默认 24h、最大 90 天、15s 查询超时、每页最多 100；summary/count/rows/trend、options 查询和渠道名称解析都绑定 deadline；options 有结果上限，模型选项只扫描最近 90 天；
  - summary、分组分页 rows、小时 trend 均由 SQL `SUM/COUNT/GROUP BY` 完成，不读取原始日志到 Go 内存、不解析历史 `other` JSON；legacy rows 只计数并从 cache-rate 分子/分母排除；
  - 新版 `web/src/features/usage-analysis/` 提供日期及用户/Token/模型/渠道筛选、summary、趋势图、分页明细、root 路由和侧栏入口；Refresh 支持同筛选显式重试，分页切换保留上一页结果，业务/HTTP 错误统一在页面内展示；七 locale 已同步；
  - focused backend/frontend tests、frontend typecheck/lint/build、root build、relaykit standalone build 与 `git diff --check` 通过。全量 Go 测试仍受既有 channel-affinity 共享状态污染影响；HTTP/2 retry 用例在全量并发下偶发 Windows socket 失败，单独复跑通过。

### 6.2 OpenAI-compatible 流式 usage 归一化

- 状态：部分重叠后重建
- 优先级：P1
- 推荐：迁移差异，不复制旧文件
- 旧能力：支持 `input_tokens/output_tokens`、任意 SSE usage chunk、缓存 token 合并和 `prompt_cache_hit_tokens`。
- 最新上游：已包含 DeepSeek、Zhipu、Moonshot、OpenAI/llama.cpp 等 post-processing，并使用 `relaykit/dto.Usage`。
- 迁移方式：建立行为对照测试，只补最新版缺失的 provider/protocol 情况；不得覆盖最新版计费表达式、usage DTO 或流处理框架。

### 6.3 OpenCode Go 渠道

- 状态：安全重写
- 优先级：P1
- 推荐：迁移
- 旧能力：
  - 保留流式 `stream_options.include_usage`；
  - Claude 请求转换后移除 `cache_control`；
  - 移除易变 billing header / `cch=`；
  - 将历史消息中的 system reminder 改写为 user，稳定 prompt cache 前缀。
- 编号冲突：旧实现使用渠道类型和 API 类型 `59`；最新版上游已将 `59` 分配给 Sub2API，并新增 `60` New API。
- 新注册方式：分配新的非冲突类型，更新后端、前端类型目录、stream support 和迁移代码。
- 兼容迁移：旧数据库中的 type 59 不能盲目改写。必须在 dry-run 中结合明确的旧版本标记或渠道特征生成待确认列表，避免把真正的 Sub2API 渠道误判为 OpenCode Go。
- relaykit：Claude/OpenAI 转换使用最新版 relaykit 扩展点，不调用旧 service 转换函数。

### 6.4 Vision Interception

- 状态：安全重写
- 优先级：P1
- 推荐：迁移，但作为可关闭功能
- 旧能力：模型后缀触发、OpenAI/Claude 图片提取、图片转文字、URL/pHash 去重、缓存、用户级配置和计费。
- 旧实现风险：
  - 直接使用 `encoding/json` 编解码，违反当前项目 JSON wrapper 约束；
  - 使用虚拟 Gin context 递归调用 relay，和最新版 relaykit/计费会话边界耦合；
  - URL 下载、DNS 变化、重定向和私网地址需要重新做完整 SSRF 威胁模型；
  - 图片解码、pHash 和并发 API 调用可能放大 CPU、内存和额度消耗；
  - 当前工作区并未真正完成历史文档声称的插件化。
- 新设计要求：
  - 功能默认可关闭；
  - 明确支持的端点和协议；
  - 使用统一请求 body storage 和 `common.*` JSON wrapper；
  - 限制编码体积、解码体积、像素、尺寸、下载时间、重定向和并发数；
  - 视觉子调用沿用原 Token、最终分组和计费会话，不绕过配额、权限和审计；
  - 缓存 key 隔离用户、模型、prompt 和配置版本；
  - 用户配置前端迁移到 `web/src/features/profile/` 并支持 i18n。

### 6.5 请求体日志查看与清理

- 状态：安全重写
- 优先级：P1
- 推荐：仅在采用安全模式后迁移
- 旧能力：把完整请求体异步保存到 `log_bodies/{logId}.json`，管理员可查看、下载和按时间清理。
- 旧实现风险：
  - 默认明文保存完整 prompt、图片 data URI、工具参数和潜在密钥；
  - 与日志数据库不在同一事务，可能产生孤儿文件或日志有记录但文件缺失；
  - 相对本地目录不适合多实例和容器滚动部署；
  - 异步写入没有持久任务、容量上限或失败状态；
  - 文件枚举会随文件数量增长；
  - 管理 API 只有 root 路由保护，缺少独立敏感数据权限和访问审计。
- 建议安全模式：
  - 默认关闭，显式按环境启用；
  - 按用户确认保存完整请求体，不做脱敏或截断；
  - 使用不可猜测对象键，并在日志记录状态、大小、哈希和过期时间；
  - 支持本地持久卷或对象存储抽象；
  - 下载需要敏感日志权限并记录审计事件；
  - 配置最大单体、总容量、保留期和后台清理；
  - 删除日志时处理关联对象，清理孤儿对象时提供对账。

### 6.6 IP 黑名单、永久封禁历史 IPv4 和用户 IP 搜索

- 状态：安全重写
- 优先级：P1
- 推荐：迁移当前最终语义
- 旧能力：
  - 精确 IPv4 黑名单；
  - 登录、注册、session、Token 和只读 Token 认证链拦截；
  - 永久封禁时收集日志历史 IPv4；
  - users 搜索框识别 IPv4并查询关联用户；
  - root 豁免和缓存失效。
- 当前最终规则：仅精确 IPv4；不自动处理 IPv6/CIDR；命中黑名单的已认证非 root 用户被永久禁用。
- 一致性约束：主库封禁与日志库历史 IP 查询无法组成跨库事务。账户封禁是主结果，附加拉黑失败应记录可重试任务和审计，不应回滚封禁。
- 多节点约束：旧实现只刷新当前实例缓存；新实现必须复用 Redis/版本广播或缩短缓存一致性窗口，避免其他实例继续放行。
- 隐私约束：IP 搜索和历史查看属于敏感权限；默认前端不保留独立“IP 查用户”入口，复用 users 搜索框。

- 当前实现（2026-08-14）：
  - 新增默认关闭的 `ip_blacklist_setting.Enabled`；规范化层只接受 canonical exact IPv4，拒绝 IPv6、CIDR 和部分匹配；
  - 永久禁用在主库事务中锁定非 root 用户、递增 `auth_version`、更新 disabled 状态，然后失效用户/Token 缓存、撤销全部会话并写 system security audit；已 disabled 用户再次进入该流程仍会补收集历史 IP；
  - 开关启用时，从 `LOG_DB.logs` 收集 distinct historical IPv4，并合并 triggering request IP，使用 `ON CONFLICT DO NOTHING` 幂等写入；主库禁用成功后，附加 IP 收集/缓存操作失败只记录错误，不回滚账户主结果；
  - 登录、注册、dashboard/session auth、API Token auth 和 relay Token auth 均接入；relay 风格链路返回 OpenAI-compatible access-denied envelope；root 始终豁免；
  - 本地全量缓存配合 Redis per-IP positive/negative cache；Redis miss 直接查询主库 exact IP 后短 TTL 回填，读路径不使用 stale local cache 推断结果；增删 IP 会立即写入正/负 Redis 值；移除条目会失效该 IP 历史用户缓存；
  - `SearchUsers` 仅在 keyword 是 exact IPv4 时从 `LOG_DB` 解析 distinct user IDs，再在主库应用现有过滤、排序、分页与敏感字段 omit；
  - root-only `/api/ip_blacklist` 提供 list/add/remove，并复用 manage audit；用户被重新启用时清理 RPM 与关联 rate-limit keys；
  - focused regression tests 覆盖 exact IPv4、Redis miss 主库 exact lookup/negative cache、历史 IP 收集、root 豁免、IP 搜索和 blacklist persistence。

### 6.7 RPM 滑动窗口、审查触发和限流日志

- 状态：部分重叠后重建
- 优先级：P1
- 推荐：迁移当前代码语义，不恢复过期文档语义
- 最新上游重叠：已有基础模型请求限流。
- 旧仓库最终代码语义：Redis ZSET/Lua 原子滑动窗口，超限返回 OpenAI 兼容 429，并异步触发 LLM Review；请求失败时释放预留槽位。
- 历史文档语义：早期版本曾描述短封禁、长期封禁和累计自动永久封禁，但当前代码明确不再执行这些动作。
- 决策要求：迁移当前代码行为，除非用户重新明确要求恢复自动封禁策略。
- 可靠性：Redis 故障时旧代码 fail-open；新实现需要记录指标和告警，并明确是否保留 fail-open。
- 内存模式：必须与 Redis 模式保持成功/失败计数语义一致，不能出现不同的窗口或释放行为。

- 当前实现（2026-08-14）：
  - Redis 使用 Lua + ZSET 原子 reserve/count/retry-after，内存 fallback 使用相同 sliding-window 计数语义；
  - 超限返回 OpenAI-compatible 429，并设置 `Retry-After` 与 `X-RateLimit-*` headers；下游转发失败时释放当前 request ID 预留槽位；
  - RPM 两条超限路径都异步构建 review trigger，包含 user/model/endpoint/current/limit、最多 2 KiB request snippet、client IP 与 stream 标记；触发器错误只写日志，不阻塞 429；
  - 当前 upstream baseline 没有 LLM Review worker。默认 `service.EnqueueRateLimitReview` 持久化 `rate_limit_review_events`：enabled 用户为 `pending`，已永久禁用用户为 `skipped_disabled`（保留审计），root 跳过；该函数变量可在后续 worker 模块中替换；
  - `rate_limit_logs` 保持 historical-only；未恢复短期/长期自动封禁或累计自动永久封禁；
  - focused tests 覆盖 atomic parallel limit、window rollover、Retry-After、request failure/explicit stream failure release、multipart request preservation、bounded snippet、stream flag 与 non-blocking review trigger；独立 security/correctness review findings 已处理。

### 6.8 LLM Review

- 状态：安全重写
- 优先级：P1
- 推荐：迁移，但默认关闭并显式配置
- 旧能力：持久任务、worker、RPM/Token 触发、请求摘要脱敏、统计、详情、管理 API 和前端页面。
- 外部依赖：审查模型渠道、系统凭据/Token、worker 生命周期。
- 安全要求：
  - 默认关闭；
  - 请求摘要先脱敏和截断，再持久化或发送上游；
  - 不保存认证 header、完整 Token、Cookie、支付字段或图片 data URI；
  - 任务幂等、租约/抢占、重试上限和终态明确；
  - 管理查看使用独立敏感审计权限；
  - worker 失败不影响正常模型请求和 429 响应；
  - 多实例运行时防止重复消费。

### 6.9 输入 Token 前置硬限制与估算器校准

- 状态：安全重写
- 优先级：P2
- 推荐：与 LLM Review 一并迁移，可独立关闭
- 来源：旧变更日志未单独列出，但旧代码存在完整实现和测试。
- 旧行为：模型至少积累 1000 个实际样本、95% 样本误差不超过 5%、阈值附近无误拒后，才允许使用估算值在转发前执行硬 429；未通过验收时 fail-open。
- 依赖：LLM Review calibration 表、实际 usage、模型级 estimator version。
- 新设计：
  - 估算算法变更必须增加版本；
  - 验收统计使用 SQL 聚合和缓存；
  - 配置变更使已有资格失效时可观察；
  - 只对明确支持的文本端点执行；
  - root 和白名单模型语义一致；
  - 预检拒绝不得预扣费或调用上游。

### 6.10 Vision 拦截（图片转文字）

- 状态：安全重写（2026-08-15 完成）
- 优先级：P2
- 推荐：默认关闭，用户级配置
- 来源：旧仓库 `middleware/vision_intercept.go` + `service/vision/vision.go`。
- 当前实现：
  - 用户配置 `UserSetting.Vision`（relaykit dto）：enabled / vision_model / vision_suffix / prompt_template / phash_threshold，默认关闭；
  - 仅 `/v1/chat/completions`、`/v1/messages`（及无版本前缀路径）拦截，模型名需匹配后缀；
  - typed JSON（common wrapper）递归提取 OpenAI `image_url`/`input_image` 与 Claude `image`（url/base64）条目，深度 12、每请求最多 16 张；OpenAI 条目就地变 text part，Claude 条目整块替换；
  - 安全边界：base64 编码 ≤20MB、解码 ≤15MB、单边 ≤8192、总像素 ≤20MP、URL 下载 ≤15MB/30s；URL 下载经 SSRF 校验 + 保护客户端；
  - pHash（goimagehash）+ 汉明距离聚类；四级缓存（请求级/跨请求 LRU/ singleflight/ pHash 环形）全部按用户+模型+prompt 哈希隔离；
  - 子调用走真实 relay 管线（选渠道、模型映射、估价、预扣、上游调用、`PostTextConsumeQuota` 结算），继承原 Token/分组身份；失败自动回滚预扣；
  - 任何失败 fail-open：不改写请求体、原样放行并记录日志；不引入旧版 502/占位符语义；
  - 前端：`web/src/features/profile/components/vision-interception-card.tsx`（7 locale i18n）。
- focused tests：base64/尺寸拒绝、pHash 距离、OpenAI+Claude 提取与替换、16 张上限、聚类、SSRF 拒绝（私有/云元数据/file://）、缓存隔离、middleware 门禁与 fail-open。

## 7. 仅做兼容迁移的旧数据

### 7.1 旧 routing group 平行模型

- 状态：兼容迁移
- 优先级：P0
- 不恢复为运行时权威：

```text
routing_groups
user_routing_group_grants
user_routing_preferences
tokens.routing_group_id
tokens.routing_mode
```

- 迁移目标：映射到原有分组目录、`user_group_grants`、`tokens.group` 和最新版 Auto Token 语义。
- 禁止复用旧迁移器的行为：
  - 不把 key 强制转小写后直接映射；
  - 不跳过活跃不可映射 Token 或授权后继续启动；
  - 不在部分成功时提交；
  - 不因映射失败静默回退到用户主分组。
- 正确流程：
  1. 只读 dry-run；
  2. canonical key 精确匹配；
  3. 精确失败后仅允许不区分大小写的唯一候选；
  4. 零候选和多候选均形成结构化阻断；
  5. 完整可迁移时才在单事务写入；
  6. 保留旧表，不自动 DROP；
  7. 写入迁移版本和摘要；
  8. readiness 在活跃阻断项存在时失败；
  9. 支持幂等重跑、回滚和三数据库验证。

### 7.2 OpenCode Go 旧 type 59

- 状态：兼容迁移
- 优先级：P0
- 风险：最新版 type 59 是 Sub2API。
- 处理：dry-run 输出所有 type 59 渠道及其名称、base URL、配置摘要和活跃状态，由明确规则或管理员确认后迁移到新编号。存在歧义时 readiness 失败。

### 7.3 历史缓存指标

- 状态：兼容迁移
- 优先级：P2
- 处理：提供离线回填/对账工具，把可可靠解析的历史 metadata 写入结构化列；无法解析的记录进入报告，不在在线请求中无限回扫。

## 8. 不迁移内容

### 8.1 旧前端和临时构建修复

- `web/default` 和 `web/classic` 文件不直接迁移；只在最新版 `web/src` 重建所选功能。
- classic 前端依赖安装修复不迁移，最新版已经使用单一前端结构。

### 8.2 旧运行时架构和普通上游代码

- 不复制旧 `service/relayconvert`、旧 Claude handler、旧 DTO 整体快照。
- 不恢复 parallel routing group CRUD 管理页。
- 不迁移未被路由使用的 `RequestBodyLimitBytes` 预留 helper。
- 不迁移已被最新版重构吸收的 task video、tool billing、pre-consume quota 等普通代码。

### 8.3 过程与部署资产

- 不迁移旧 `task_plan.md`、`findings.md`、`progress.md` 的历史副本。
- 不迁移 ssh2 路径、镜像名、容器 inspect、数据库备份路径或生产部署状态。
- 不复制旧 README 的定制改写。
- 在用户明确确认前不提交、不推送、不部署；本次用户已确认后，产品提交 `bd8b8746` 已推送并部署，过程运行时与任务资产仍不纳入产品发布。

## 9. 建议迁移批次

### 批次 A：低耦合行为与数据基础

1. OpenAI-compatible usage 行为差异测试与归一化；
2. 结构化缓存指标和 SQL Usage Analysis；
3. OpenCode Go 新编号、最新版 adaptor 和 type 59 dry-run；
4. 安全模式请求快照（若用户确认）。

### 批次 B：安全与审查

1. IP 黑名单和认证链接入；
2. 永久封禁历史 IPv4 的可重试附加任务；
3. users IPv4 搜索；
4. RPM 原子滑动窗口；
5. LLM Review；
6. 输入 Token 前置限制与校准。

### 批次 C：Vision

1. 用户配置和 capability；
2. 安全图片读取与 pHash；
3. 视觉子调用、计费和缓存；
4. OpenAI/Claude 请求改写与前端设置。

### 批次 D：分组和渠道模型策略

1. 用户额外授权与缓存；
2. 统一组解析器和固定 Token 边界；
3. Auto Token 权限交集；
4. 用户/Token 前端加载安全；
5. 渠道模型三态策略；
6. ability/cache 原子投影；
7. 旧 routing group dry-run、迁移和 readiness。

说明：批次 D 复杂度最高，但其产品语义已经由用户确认。开发时可以先实现其纯模型和迁移测试，再切换请求热路径。

## 10. 建议确认结果

建议默认确认以下项目：

- Usage Analysis 与结构化缓存指标；
- OpenAI-compatible usage 差异；
- OpenCode Go；
- Vision Interception；
- IP 黑名单、永久封禁历史 IPv4 和 users IPv4 搜索；
- RPM 原子滑动窗口；
- LLM Review；
- 输入 Token 前置限制和校准；
- 已确认的用户额外分组、固定/Auto Token 和渠道模型三态；
- 所有 fail-closed 兼容迁移和对账工具。

用户已确认保留完整请求体。实现已完成：默认关闭，内容读取仅允许 Root 超级管理员直接访问（不可委派、不要求 2FA/Passkey proof），并保留主库 metadata/access audit、HKDF + AES-256-GCM 节点本地原子文件、容量与 retention/orphan/missing/terminal 清理、no-store/rate limit 及前端按需查看/复制/下载。成功返回前审计 fail-closed、terminal row 有界清理、通用错误安全码和设置溢出保护保持不变。

## 11. 全局验收门槛

- 固定 Token 无法通过任何请求字段、模型列表或重试路径切换到其他组；
- 用户更新字段省略不会清空额外授权；
- 前端加载失败或遇到未知旧值时不会自动覆盖；
- 迁移存在活跃阻断项时服务不进入 ready；
- 渠道和策略写入、ability 投影及附属记录具有明确事务边界；
- 内存渠道缓存与 ability 数据来源一致；
- Usage Analysis 不加载整个时间范围的原始日志到 Go 内存；
- 请求快照、LLM Review 和 IP 数据遵循最小化、脱敏、权限和审计要求；
- SQLite、MySQL、PostgreSQL 的模型与迁移行为一致；
- `go test ./...`、`relaykit` 独立构建、前端 typecheck、受影响文件 lint/format 和生产构建通过；
- 新代码不引入最新版上游基线之外的新 lint/format 错误。
