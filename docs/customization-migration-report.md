# 定制重建交付报告（阶段 7）

> 生成日期：2026-08-15
> 分支：`rebuild/customizations-20260812`
> 产品提交：`9d828d74`（`fix: relax security proofs and restore data assets`）；前序可观测性 UX 提交为 `bd8b8746`，完整重建提交为 `da678d51`
> 基线：`ccd535ef8e50cf6e5846a59278c40b7ff59d1b7d`（上游 `QuantumNous/new-api`）
> 部署：2026-08-15 14:56 +08:00 已部署到 `ssh2`，镜像 `newapi-custom:20260815-security-avatar-9d828d74`，分支已 push 至 `origin/rebuild/customizations-20260812`

## 1. 交付范围

以最新上游为新基线，重建旧 fork（`E:\myCode\myapi`，只读）中仍有价值的定制功能。上游已具备的功能（Playground、协议转换、订阅/钱包页面等）直接采用上游，未重复移植。

| 阶段 | 内容 | 状态 |
|---|---|---|
| 1 | 盘点、基线验证、正式定制清单 | ✅ |
| 2 | OpenCode Go 渠道、OpenAI usage 归一化、安全请求快照、结构化缓存指标 + SQL Usage Analysis | ✅ |
| 3 | RPM 原子滑动窗口 + OpenAI 兼容 429、审查触发、精确 IPv4 黑名单、永久禁用、认证拦截、IP 搜索 | ✅ |
| 4 | LLM Review 全链路（4.1–4.7）、输入 Token 前置限制与校准（4.8）、Vision 拦截（4.9） | ✅ |
| 5 | 用户额外分组授权、渠道模型三态、统一分组解析器、固定 Token 校验 | ✅ |
| 6 | 旧 routing_groups 兼容迁移（dry-run/readiness/fail-closed/对账/回滚手册） | ✅ |
| 7 | 完整验证与交付报告（本文件） | ✅ |

关键文档：`docs/customization-migration-inventory.md`（功能清单）、`findings.md`（盘点结论）、`task_plan.md`（阶段计划与决策）、`progress.md`（实施记录）、`HANDOFF.md`（交接）、`docs/routing-group-migration-manual.md`（迁移/回滚手册）。

## 2. 验证矩阵

| 项目 | 结果 |
|---|---|
| `go build ./...`（便携 Go 1.26.1） | ✅ |
| `relaykit` 独立构建（`GOWORK=off`） | ✅ |
| 定向 Go 测试（model/service/controller/middleware/setting/relay/common/authz 等） | ✅ 全绿 |
| 完整 `go test ./...` | ✅ 仅 2 个基线预存失败（见 §4） |
| 前端 `bun run typecheck` | ✅ |
| 前端 `bun run build` | ✅ |
| 前端 `bun test src/features` | ✅ 仅基线预存失败（见 §4） |
| 本次可观测性 focused Bun tests | ✅ 23/23 |
| `git diff --check` | ✅（仅既有 `sync-i18n.mjs` LF/CRLF 提示） |
| 七 locale i18n 同步 | ✅（`0 missing / 0 extras / 0 untranslated`） |
| 临时脚本/`.pi`/`.review` 残留 | ✅ 无 |
| 密钥、依赖、许可证 | 未引入新密钥；新增依赖仅 `goimagehash`(MIT) + `nfnt/resize`；前端无新依赖 |

## 3. 默认关闭与权限边界（安全摘要）

- **请求快照**：默认关闭；内容仅 Root 超级管理员可直接读取，不可委派且不要求 2FA/Passkey proof；仍保留读取审计、HKDF/AES-256-GCM、容量与保留期清理；更换 `CRYPTO_SECRET` 会使旧快照不可读——密钥轮换必须与保留期清理配套（见 inventory §3.5）。
- **IP 黑名单**：默认关闭（`ip_blacklist_setting.Enabled`）；exact IPv4；root 豁免；Redis + 主库精确回源。
- **LLM Review**：默认关闭；`llm_review.read` 无默认授权，管理路由仍要求 AdminAuth、精确权限、关键限流与审计；配置/测试/清理 root-only；API Key 加密存储。管理路由不再强制额外 2FA/Passkey proof。
- **输入 Token 前置限制**：总开关默认关闭（`max_input_tokens=200000`、`max_output_tokens=10000`）；未过校准验收 fail-open。
- **Vision 拦截**：用户级默认关闭；SSRF 校验 + 硬边界；失败 fail-open。
- **渠道模型三态**：`model_group_modes` 归入 `ChannelSensitiveWrite`（fail-closed 字段分类守卫）。
- **用户额外分组**：`extra_group_keys` presence 语义（nil 保留/空清空/替换）；固定 Token 创建/更新服务端校验；Auto 候选与有效权限交集。
- **旧表只读**：`routing_groups` 等旧表在任何迁移路径中永不写入或删除。

## 4. 基线预存问题（非本次引入，不处理）

1. `service` 包全量运行时 `TestObserveChannelAffinityUsageCacheByRelayFormat_*` 共享状态污染失败；单独 `-count=1` 复跑通过。
2. `relay/channel` 的 HTTP/2 GOAWAY retry 用例在全量并发时偶发 Windows socket abort；单独复跑通过。
3. 前端 `api-key-group-cell.test.tsx` 3 个失败：上游 HEAD 中组件注释了 AutoGroupBadge 帧但测试仍期待 2 帧（相关文件均未改动）。
4. 前端 `format:check` 对 3 个上游既有文件（`channel-form.ts`、`api-key-group-cell.tsx`、`redemption-form.ts`）报告格式差异；本次新增/修改文件均已格式化。
5. Bun 运行 `node:test` 风格测试时的嵌套 `describe()` 噪音（14 条 "Unhandled error between tests"），非断言失败。

## 5. 部署与启用指引

1. **代码部署**：产品提交 `9d828d74` 已推送至 `origin/rebuild/customizations-20260812`，并于 2026-08-15 14:56 +08:00 构建部署到 `ssh2`；容器镜像为 `newapi-custom:20260815-security-avatar-9d828d74`，镜像 ID 为 `sha256:d2e56d70f7bf4c8921b2c69a87b776e4d03c062422753b87825984214accd891`，`/api/status` 和首页均返回 200，容器运行且重启次数为 0。部署前 SQLite 备份位于 `/opt/newapi/backups/deploy-20260815-145129-9d828d74`，`PRAGMA integrity_check` 返回 `ok`；备份同时保留 Compose、`.env`、旧源码和旧镜像元数据。`https://newapi.uwoacrimson.com/data-assets/anon-removebg-preview.png` 通过公网验证为 HTTP 200、`image/png`、344251 bytes，响应与源文件 SHA-256 一致；缺失文件、POST 和路径穿越请求均为 404。。
2. **schema**：新表在启动时经 `migrateDB`/`migrateDBFast` 自动创建（`user_group_grants`、`channel_model_group_overrides`、`channel_model_group_disabled`、LLM Review 4 表、IP 黑名单、快照 2 表、日志新列等）。
3. **旧 routing_groups 数据**：启动诊断发现未映射旧分组 key `渠道1`，因此严格迁移尚未执行，避免静默丢失授权。处理该 key 后按 `docs/routing-group-migration-manual.md` 执行 preview → readiness → run。
4. **可选功能**均默认关闭：按需在管理端开启（IP 黑名单、LLM Review、输入 Token 前置限制、用户级 Vision）。
5. **请求快照密钥**：部署时固定 `CRYPTO_SECRET`/`SESSION_SECRET`，密钥轮换与 retention 清理计划配套。

## 6. 回滚

- 阶段 5/6 为 additive schema，旧表只读；回退到基线代码即可恢复旧读路径，无需 destructive down-migration（详见 migration manual §6）。
- 其余阶段功能默认关闭；回退代码即可完全停用。

## 7. 部署后待办

- 核对旧分组 key `渠道1` 的 canonical 映射，处理 blocker 后再运行严格兼容迁移；当前旧表未被写入或删除。
- 生产日志提示 `SESSION_COOKIE_SECURE` 未开启，且 `TRUSTED_PROXIES` 使用兼容默认值；应结合现有 Caddy 反向代理配置单独确认。
- 当前产品与文档提交已 push 至 `origin/rebuild/customizations-20260812`；未创建 PR，未向 `upstream` 推送。
- 可选功能仍保持默认关闭，应按业务需要逐项启用。
