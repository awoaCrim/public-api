# 优化限流 LLM 审查去重与请求上下文

## Goal

让 RPM 限流的一次阈值窗口最多选择第一个超限请求进行 LLM 审查，并让所有自动限流审查都向 reviewer 提供有界、脱敏的请求摘要、请求记录和请求头。

## Requirements

### RPM 窗口去重

- 同一用户、同一个 RPM 滑动窗口内，最多创建一个审查任务并调用一次 reviewer。
- 第一个超限请求选中后，窗口内后续超限请求仍按现有协议返回 HTTP 429，但不得再创建、合并或调用新的 RPM 审查。
- 第一个任务无论最终为 `compliant`、`violation`、`uncertain`、`failed`、`skipped` 或 `superseded`，该窗口内都不自动选择第二个请求；失败后的再次处理使用现有人工 retry，而不是依靠后续 429 创建替代任务。
- 窗口结束后，新的 RPM 超限事件可以选择新的第一个请求。
- RPM 窗口去重必须独立于 `ActiveTaskId`；完成任务后可以释放 active slot，但窗口标记保留至到期。
- 一个 RPM 窗口最多为 `CompliantCount` 贡献一次 compliant 结果，避免同一窗口快速触发长期免审。
- 同窗口后续事件静默去重，不生成重复 `skipped` 审计记录，避免任务列表被同一次限流事件淹没。
- Root、永久禁用用户、review disabled/unavailable、长期 grace、manual ban/unban、retry 和自动封禁安全边界保持现有语义；第一条被选中的事件仍可记录其真实 skipped 原因。

### 请求上下文

- RPM、input-token preflight、input-token postflight、output-token postflight 自动触发路径都必须使用同一个请求上下文捕获合同。
- reviewer payload 在现有 `request_snippet` 摘要之外增加：
  - 有界、脱敏的请求体记录；
  - 有界、脱敏的请求头。
- 请求体和请求头必须在启动异步 goroutine 前同步复制，异步代码不得继续读取 `gin.Context` 或请求 body storage。
- 不得向数据库或 reviewer 暴露 Authorization、Cookie、API key、access/refresh token、password、secret、signature、JWT、原始 base64/media 内容等敏感值。
- JSON、multipart 和其他文本请求都应得到安全、有界的记录；multipart 只记录文本字段和文件元数据，不记录文件字节。
- 请求上下文捕获失败必须 fail-safe：允许创建审查任务但使用空或截断上下文，不得阻塞原有 429/relay 响应。
- 手动创建审查任务没有原始被审查 HTTP 请求，不得误把管理员创建任务接口自身的 body/header 当作审查上下文。

### Compatibility and scope

- SQLite、MySQL 5.7.8+、PostgreSQL 9.6+ 必须同时支持。
- 使用 GORM 和条件更新/CAS；不得增加只适用于单一数据库的锁或索引语义。
- 保持现有 OpenAI-compatible 429 响应和 `Retry-After` / `X-RateLimit-*` headers。
- 保持 reviewer readiness、strict-only auto-ban、credential masking、retention 和管理权限合同。
- 历史 `LLMReviewPayload` 没有新字段时仍可正常读取。
- Request Snapshot 功能不作为 reviewer 上下文依赖。

## Acceptance Criteria

- [ ] 确定性回归测试证明：同一 RPM 窗口连续/并发触发时只有第一个事件创建任务，后续事件仍返回 429 但不新增任务或 reviewer 调用。
- [ ] 第一个任务完成并释放 active slot 后，同一窗口的后续 RPM 事件仍被静默去重。
- [ ] 窗口到期后可以创建新的 RPM 审查任务。
- [ ] 一个 RPM 窗口最多增加一次 compliant count；现有长期 grace 只按独立任务/窗口累计。
- [ ] Redis 和内存 RPM 实现都向审查触发器提供明确、可测试的窗口边界。
- [ ] RPM、input-token preflight、input/output-token postflight 的最终 reviewer payload 都包含 `request_snippet`、脱敏请求体记录和脱敏请求头。
- [ ] 测试证明 Authorization、Cookie、API key、JWT、password/secret、base64/media bytes 不会进入 task payload 或 reviewer 请求。
- [ ] 请求体/header 的读取量、输出量、嵌套、集合和单值长度都有确定上限，且 body storage 仍可被后续 relay 读取。
- [ ] 第一条 disabled/unavailable/grace/skipped RPM 事件保留审计原因，同窗口后续事件不产生重复记录。
- [ ] Root、permanent ban、manual override、retry、auto-ban trust 和 429 协议测试继续通过。
- [ ] `go test ./common ./middleware ./model ./service ./controller`、`go build ./...` 和 `git diff --check` 通过；若改动前端，则额外通过 `web` 的 Bun typecheck/build/i18n 验证。

## Out of Scope

- 改变 RPM 阈值、限流时长或 429 协议。
- 删除现有 configurable compliant count / long grace 机制。
- 为每一个被 merge 的跨请求 trigger 创建独立 reviewer 调用；本次任务保证每个实际创建的 task 携带其首个触发请求上下文。
- 将完整未脱敏请求快照发送给 reviewer。
- 改变 Request Snapshot 的 Root-only、加密、保留或节点归属合同。
