# New API 定制重建发现

## 仓库与来源

- `E:\myCode\public-api` 是 `awoaCrim/public-api` 的完整 fork，克隆时 HEAD 与官方上游 main 一致：`ccd535ef8e50cf6e5846a59278c40b7ff59d1b7d`。
- `E:\myCode\myapi` 的首个提交 `e747e19` 是无父提交的整仓快照，无法仅靠 Git 历史精确区分快照中的上游代码和此前定制。
- 旧仓库快照后的提交可以明确识别缓存分析和统一分组路由改造，但更早的 Vision、封禁、IP、日志和审查功能只能通过代码、设计文档和变更记录共同盘点。
- 外部分享对话指出的问题已经通过当前旧代码抽查得到印证，不能只把它们当作推测。

## 已确认的外部审查问题

1. 固定 Token 的请求分组边界不完整：旧解析器允许请求体分组覆盖固定 Token，只要该分组属于用户。
2. 用户更新使用模型结构直接解码，无法区分 `extra_group_keys` 省略和显式空数组。
3. 用户和 Token 编辑表单吞掉加载错误，且会用空数组或默认组覆盖未知旧值。
4. 旧路由兼容迁移会跳过不可映射记录、转小写 key，并在部分迁移后继续启动。
5. 模型分组覆盖缺少完整 group key 校验，替换和渠道删除的事务边界不完整。
6. 使用分析缓存指标会读取查询区间内全部原始日志，在 Go 内存中逐条解析和聚合。

## 最新上游重要变化

- 最新上游已经支持每个 Token 的 Auto 候选分组列表，不应退回旧定制的“Auto 永远使用全部用户组”实现。
- 最新上游已经包含管理员细粒度权限、Playground/Markdown、Responses/Chat 转换、签到和基础 Auto 分组等能力，应优先复用。
- 最新上游后端已经引入独立 `relaykit` 模块，旧代码的 DTO、协议转换和 relay 接入不能整文件覆盖。
- 最新上游前端已从旧仓库的 `web/default` 结构演进为单一 `web/src` 结构，旧 TS/TSX 文件必须按功能重写接入。
- 最新上游已占用渠道类型 `59` 作为 Sub2API、`60` 作为 New API；旧 OpenCode Go 的 type 59 存在数据迁移歧义，必须分配新编号并先 dry-run。
- 最新上游 `Channel.Insert()`、`Channel.Update()`、`Channel.Delete()` 与 ability/缓存生命周期仍不是可直接承载模型三态策略的统一事务，需要在重建时收口。
- 最新上游内存渠道缓存仍按 `channels.group × channels.models` 构建，模型三态策略必须同步改为与 ability 投影一致的数据来源。

## 正式盘点结论

正式功能清单已生成：`docs/customization-migration-inventory.md`。

来源核验显示，旧变更记录中管理员权限、ClickHouse LIKE、Playground/Markdown、认证文案、Responses/Chat 转换和渠道 UI 的 11 个提交存在于官方上游历史，应直接采用最新版。Vision、请求体日志、RPM/IP、OpenCode Go 等后续本地提交不在上游历史，属于重建候选。

旧变更日志之外还确认了一项高置信度定制：输入 Token 前置硬限制与估算器校准验收。它依赖 LLM Review 和实际 usage，建议作为可独立关闭的功能一并迁移。

最新阶段 2 结果：安全请求快照已按完整内容语义完成安全重写。实现默认关闭；只有显式稳定 secret 才运行；内容读取仅允许 Root 超级管理员直接访问，不可委派且不再要求 2FA/Passkey proof；仍保留主库 metadata/access audit、HKDF-SHA256 + AES-256-GCM 节点本地原子文件、容量/retention/orphan/missing/terminal 有界清理、no-store/rate limit 与前端按需查看/下载。成功审计 fail-closed、terminal row 有界、通用错误路径安全码及设置溢出保护保持不变。

结构化缓存指标与 SQL Usage Analysis 也已完成：日志在写入时持久化 cache read/write（含 Claude 5m/1h）和 total input，并对负值及 ClickHouse Int32 上界做饱和保护；在线查询仅执行 SQL aggregate/group/pagination，不再加载原始日志或解析 `other` JSON。Root-only API 具备 24h 默认、90d 上限、15s timeout、最大 page size 100；summary/count/rows/trend、options 和渠道名解析均绑定 deadline，options 有结果上限且模型只扫描最近 90 天；legacy rows 被计数但从 cache-rate 分子和分母排除。前端提供 root 路由、筛选、summary、trend、分页明细与七 locale，Refresh 可显式重试，分页保留旧数据，错误统一页内展示。

旧仓库 RPM 功能的最终代码语义已经从早期“短期/长期自动封禁”演进为“Redis ZSET/Lua 原子滑动窗口、429、失败释放槽位并触发 LLM Review”。迁移应以最终代码为准，不恢复过期文档语义，除非用户重新确认。

### 建议迁移

- Vision 图片转文字、pHash 去重和用户级配置。
- RPM 原子滑动窗口、429、审查触发和限流日志。
- 精确 IPv4 黑名单、永久封禁历史 IP 收集和 users IP 搜索。
- 默认关闭、脱敏截断、带敏感权限和审计的安全请求快照；不建议恢复旧版明文请求体日志。
- LLM Review 持久化任务、worker 和管理界面。
- 输入 Token 前置硬限制和估算器校准验收。
- OpenCode Go 渠道和 prompt cache/stream usage 兼容。
- OpenAI-compatible usage/cache 归一化和结构化缓存指标。
- 用户额外原有分组授权。
- 渠道模型 `inherit`、`custom`、`disabled` 三态分组策略。

### 不应重复迁移

- 管理员细粒度权限的旧副本。
- Playground 和普通 Markdown/富文本组件的旧副本。
- Responses 与 Chat 的旧转换包。
- 签到基础实现。
- 已淘汰的 `routing_groups`、`user_routing_group_grants`、`user_routing_preferences` 和 Token 旁路字段运行时模型。

### 不进入产品代码

- ssh2 目录、镜像、容器和备份路径盘点。
- 旧任务过程文件和过期设计结论。
- 旧快照提交本身。

## 基线验证

- 前端依赖：`bun install --frozen-lockfile --force` 成功，锁文件未改写。首次普通安装因两个 tarball 完整性校验失败，强制重新下载后恢复。
- 前端 typecheck：通过。
- 前端 production build：通过。
- 前端 lint：最新版上游基线已有多处 error/warning，失败。
- 前端 format check：最新版上游基线已有 7 个文件不符合格式，失败。
- `relaykit` 独立构建：使用便携 Go `1.26.1` 和 `GOWORK=off` 通过。
- 完整 `go test ./...`：绝大多数包通过；`service` 的 channel affinity 统计测试存在既有全局状态污染（单独运行也可能因进程内其他测试残留而失败）；`relay/channel` 的两个 HTTP/2 GOAWAY retry 用例在全量并发下于 Windows 偶发 socket abort，定向单独复跑通过。
- `git diff --check`：通过。

## 等待用户确认

- 已确认迁移全部建议功能。
- 已确认请求快照保存完整请求体，不做脱敏或截断；仍需默认关闭、专用敏感权限、访问审计、受控存储、容量限制和保留期清理。
- 已确认 RPM 按最终代码语义迁移：原子滑动窗口、429、失败释放槽位并触发 LLM Review。
