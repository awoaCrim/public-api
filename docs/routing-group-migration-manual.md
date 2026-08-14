# 旧 routing_groups 兼容迁移手册

> 适用：从旧 fork（含 `routing_groups` / `user_routing_group_grants` / Token 的
> `routing_mode`、`routing_group_id` 列）升级到本重建版的部署。
> 原则：**只读 dry-run 先行、存在活跃未映射引用时 fail-closed、旧表永不写入或删除。**

## 1. 背景

本重建版把分组路由统一回原有分组模型：

- 用户额外权限：`user_group_grants(user_id, group_key, source, expires_at, sort_order)`；
- 渠道模型三态：`channel_model_group_overrides` + `channel_model_group_disabled`；
- Token 继续用 `tokens.group`（单固定组或 `auto`）。

旧 fork 曾短暂使用 `routing_groups` 平行模型。迁移只把**能映射到当前分组目录**
的数据导入新表；映射依据是「原有分组目录」（`UserUsableGroups` ∪ `GroupRatio`）。

## 2. 迁移前检查（dry-run）

```bash
curl -s -H "Authorization: Bearer <root-session>" \
  http://<host>/api/routing-group-migration/preview
```

返回的只读报告字段：

| 字段 | 含义 |
|---|---|
| `mapped_groups` | 旧 `routing_groups.id` → 目录 key 的映射 |
| `unmappable_groups` | 不在目录中的旧分组 key |
| `grant_imports` | 将要导入/合并的授权（user, group_key, expires_at） |
| `token_updates` | 将要规范化的 Token 组（auto/legacy_auto/空组+路由ID） |
| `unmappable_tokens` | 引用不可映射分组的 Token ID（**阻断项**） |

preview 永不写数据，可反复执行。

## 3. Readiness 与阻断

```bash
curl -s -H "Authorization: Bearer <root-session>" \
  http://<host>/api/routing-group-migration/status
```

- `ready=false` 表示存在活跃未映射旧引用（`blockers` 列出明细）；
- 服务启动时也会输出同样的阻断告警日志，但**不阻塞启动、不自动迁移**；
- 上线门禁要求 `ready=true`（或阻断项已由管理员确认处理）。

处理阻断项（二选一）：

1. 手工修正旧数据（例如把孤儿 Token 改为 `group='auto'` 或清空 `routing_group_id`），再重跑 preview；
2. 确认该引用属于历史遗留、不需要迁移：在旧表中将其置为非活跃（停用 Token），重新检查。

## 4. 执行迁移

```bash
curl -s -X POST -H "Authorization: Bearer <root-session>" \
  http://<host>/api/routing-group-migration/run
```

行为：

- **fail-closed**：仍存在阻断项时返回 `success:false` 与报告，不写任何数据；
- 全部可映射时，在单事务内：
  1. 授权按 `(user_id, group_key)` 合并（多来源取最晚到期，永久优先），导入为 `manual` 来源；
  2. Token 规范化：`auto`/`legacy_auto` → `group='auto'`；空组 + 可映射 `routing_group_id` → 目录 key；
  3. 写入迁移版本标记 `routing_group_migration_version=1`。
- 幂等：重复执行不产生重复行；旧表只读。

## 5. 迁移后对账

再次请求 `/status`：

- `in_sync=true` 表示 dry-run 已无待导入授权、待更新 Token 与未映射引用；
- 手动抽样核对：`user_group_grants` 中目标用户行数 == preview 的 `grant_imports` 行数；
- 旧表（`routing_groups` / `user_routing_group_grants`）保持原样，可随时比对。

## 6. 回滚

迁移**从不修改旧表**，且新代码只在下列点读取新表：

- `user_group_grants`：用户有效组解析（auth、distributor、auto 候选、模型/定价列表）；
- `channel_model_group_overrides` / `channel_model_group_disabled`：渠道 ability 投影。

回滚步骤：

1. 回退到不含阶段 5 的镜像/代码版本；
2. 新表保留即可（additive schema，无 destructive down-migration）；
3. 旧表数据未被改动，旧读路径立即恢复；
4. 若已通过渠道三态修改过 ability 投影，旧版启动时会按 `channels.group × models` 重建旧语义，无需手工操作。

## 7. 数据库兼容性

- 迁移逻辑全部走 GORM 方言无关查询，覆盖 SQLite / MySQL 5.7+ / PostgreSQL 9.6+；
- 表存在性检查使用 `Migrator().HasTable`，旧库缺新表时全部操作安全跳过。
