# 修复分组列表缓存与 test 遗留引用

## Goal

修复 GroupRatio 配置保存后分组列表仍显示旧数据的问题，并清理 ssh2 生产数据库中仍指向 `test` 的活动引用；本阶段只记录计划与上下文，不修改生产代码或执行生产迁移。

## Requirements

- GroupRatio 保存成功后，统一失效共享 React Query query key `['groups']`，使分组列表重新获取最新数据。
- 保存失败时不得失效 `['groups']`，避免失败写入造成无意义刷新或掩盖错误。
- 与 GroupRatio 无关的 option 更新不得失效 `['groups']`。
- 将 ssh2 生产数据中的活动 `test` 引用迁移为 `svip`，范围限定为：
  - `channels`：channel 53；
  - `abilities`：两条活动引用；
  - `tokens`：token 32；
  - `user_group_grants`：grant 237。
- 生产迁移必须先创建 SQLite 备份，并在单个事务内完成；失败时回滚事务并保留备份以支持恢复。
- 保留历史数据：`logs` 189 条、`perf_metrics` 12 条、`quota_data` 13 条不迁移、不删除、不重写。
- 不新增通用的 rename API；迁移应是明确、可审计、范围受限的一次性运维操作。

## Acceptance Criteria

- [ ] GroupRatio 成功保存后，所有使用共享 `['groups']` query 的列表在下一次读取时获得最新分组数据。
- [ ] GroupRatio 保存失败时，`['groups']` 不被 invalidate。
- [ ] 非 GroupRatio option 成功或失败更新均不会触发 `['groups']` invalidate。
- [ ] 新增 deterministic hook test，覆盖上述成功、失败、无关更新分支，不依赖真实网络或固定 sleep。
- [ ] ssh2 迁移前已验证 SQLite 备份可读，迁移事务提交后四类活动引用全部从 `test` 指向 `svip`，且没有超出授权记录范围的更新。
- [ ] `logs`、`perf_metrics`、`quota_data` 的历史记录数量和内容保持不变。
- [ ] 通过受影响 frontend tests、`bun run typecheck`、相关 lint、`bun run build`。
- [ ] 按 deployment guideline 从已提交 ref 部署，完成远端容器、`GET /api/status`（HTTP 200 且 `success=true`）及变更前端资源健康检查。
- [ ] 失败部署或迁移均有可执行的 rollback 路径，并保留备份与验证记录。
