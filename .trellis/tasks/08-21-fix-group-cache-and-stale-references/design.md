# Design: 分组缓存失效与活动引用迁移

## Root cause

- 分组列表使用的 React Query `['groups']` 缓存默认可存活 5 分钟。
- `useUpdateOption` 的成功写入路径没有集中失效共享 `['groups']`，因此 GroupRatio 已成功保存时，其他页面仍可能展示旧分组列表。
- ssh2 生产数据中仍存在活动记录引用 `test`，导致运行时继续读取已不再作为目标分组的旧引用；历史日志和指标数据则属于审计/分析记录，不应被当作活动配置一起改名。

## Frontend design

- 在 `useUpdateOption` 的成功回调中按 option 名称识别 `GroupRatio`。
- 仅当 GroupRatio mutation 成功完成后调用共享 QueryClient 的 `invalidateQueries({ queryKey: ['groups'] })`。
- mutation 失败直接进入既有错误处理，不触发失效；其他 option 成功也不触发失效。
- 保持现有 option 保存、错误提示和 query key 约定，不新增通用 rename API，也不在各个列表组件中重复补丁式刷新。
- 使用 deterministic hook test 验证 mutation 回调与 invalidate 行为；通过可控的 QueryClient/mutation mock 观察外部行为，不测试私有实现细节。

## Production migration design

- 迁移目标是 `test` → `svip`，但只更新已确认的活动引用：`channels` 的 channel 53、`abilities` 两条记录、`tokens` 的 token 32、`user_group_grants` 的 grant 237。
- 执行顺序：读取并核对目标记录 → 创建 SQLite 数据库备份 → 开启事务 → 对授权记录执行带原值条件的更新 → 校验更新数量和目标值 → 提交事务。
- 使用显式表/主键范围，不提供通用 rename API；每次更新都记录表名、主键、旧值、新值和受影响行数。
- 事务内任一校验失败即回滚；事务外的恢复方案使用迁移前 SQLite 备份，不覆盖备份文件。
- `logs`、`perf_metrics`、`quota_data` 保持原样，即使其中存在历史 `test` 文本或关联值也不参与活动引用迁移。
- 迁移脚本/操作说明必须避免输出 DSN、凭据和完整敏感记录，并在执行前确认目标数据库与 ssh2 环境。

## Rollout and rollback

1. 先完成前端回归测试与构建，再按 deployment guideline 从 committed ref 打包部署。
2. 部署后确认容器运行、`/api/status` 健康、变更前端资源可访问，再执行生产数据迁移；若团队运行手册要求先迁移，则必须保持同样的备份、事务和健康门禁。
3. 迁移失败：事务回滚，保留 SQLite 备份并核对活动引用未产生部分提交。
4. 部署失败：恢复 Compose/image 备份并重建旧服务，直到 `/api/status` 恢复健康。
5. 若需要撤销已提交迁移，使用备份恢复或同样受限的反向运维步骤；不得借此引入通用 rename API 或修改历史表。
