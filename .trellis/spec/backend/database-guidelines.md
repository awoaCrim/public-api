# Backend database guidelines

The application uses GORM v2 and must preserve compatibility across the supported primary databases: SQLite, MySQL (>= 5.7.8), and PostgreSQL (>= 9.6). ClickHouse is supported only as an optional log database. A change that works only with the developer's default SQLite database is not sufficient.

## Database ownership and initialization

- `model/main.go` owns database selection, initialization, migration, and dialect-aware column constants.
- `common.DatabaseType` in `common/database.go` identifies SQLite, MySQL, PostgreSQL, and ClickHouse. Use `common.UsingMainDatabase(...)` for primary database branches and `common.UsingLogDatabase(...)` for log database branches; do not infer the dialect from the DSN at a call site.
- `chooseDB` in `model/main.go` selects SQLite when `SQL_DSN` is absent, PostgreSQL for `postgres://`/`postgresql://`, MySQL for the remaining SQL DSNs, and ClickHouse only for `LOG_SQL_DSN`. ClickHouse is explicitly rejected as the primary database.
- The primary connection is `model.DB`; the log connection is `model.LOG_DB`. When `LOG_SQL_DSN` is unset, `InitLogDB` aliases `LOG_DB` to `DB`.
- Root models are included in `migrateDB` (`model/main.go`), while `migrateLOGDB` migrates `Log` or creates the ClickHouse log table. New log persistence must not accidentally write to `DB`.

## Prefer GORM query APIs

Use GORM's model/query methods and parameter binding for ordinary persistence:

```go
err := DB.Model(&User{}).
    Where("id = ?", userID).
    Updates(map[string]any{
        "aff_count": gorm.Expr("aff_count + ?", 1),
        "aff_quota": gorm.Expr("aff_quota + ?", common.QuotaForInviter),
    }).Error
```

This pattern is used by `model/user.go` (`inviteUser`) and keeps values parameterized. Prefer `Create`, `First`, `Find`, `Where`, `Select`, `Updates`, `Delete`, `Transaction`, and `AutoMigrate` over hand-written SQL.

Let GORM generate primary keys. Do not add MySQL `AUTO_INCREMENT` or PostgreSQL `SERIAL` to models or migrations. Model tags and `AutoMigrate` are the normal schema source; explicit migration helpers are for compatibility cases that GORM cannot safely express.

## Transactions and concurrency

- Put a read-modify-write invariant in `DB.Transaction(func(tx *gorm.DB) error { ... })` or an explicitly started transaction that always rolls back on early return. `model/user.go:TransferAffQuotaToQuota` is an example of the explicit `Begin`/deferred `Rollback`/`Commit` pattern.
- For standard row locks, use `lockForUpdate(tx)` from `model/locking.go` inside the transaction:

```go
return DB.Transaction(func(tx *gorm.DB) error {
    var flow AuthFlow
    if err := lockForUpdate(tx).
        Where("token_hash = ?", tokenHash).
        First(&flow).Error; err != nil {
        return err
    }
    // validate and consume the flow while the transaction is open
    return nil
})
```

  `model/auth_flow.go` uses this transaction/lock pattern for one-time flow consumption; `model/redemption.go` applies the same helper while querying its reserved `key` column. The exact reserved-column quoting in the real redemption implementation is selected for PostgreSQL. `lockForUpdate` emits `clause.Locking{Strength: "UPDATE"}` for MySQL/PostgreSQL and intentionally skips `FOR UPDATE` for SQLite, where that syntax is unsupported. Never use the legacy GORM v1 `tx.Set("gorm:query_option", "FOR UPDATE")`, and do not duplicate `clause.Locking` at call sites.
- SQLite's skipped row-lock clause does not make a multi-step operation safe by itself. Use an atomic `UPDATE`/compare-and-swap where appropriate; `model/user_session.go` documents this for refresh-token rotation.
- Keep related updates in the same transaction. `model/subscription.go`, `model/topup.go`, and `model/auth_flow.go` show the pattern of locking the order/user/flow before applying an accounting or one-time-consumption transition.

## SQL and dialect compatibility

Raw SQL is allowed only when GORM cannot express the operation, and every supported database needs a valid path.

- Reserved columns use the initialized identifiers in `model/main.go`: `commonGroupCol` and `commonKeyCol` are PostgreSQL `"group"`/`"key"` and MySQL/SQLite `` `group` ``/`` `key` ``. Use these variables rather than embedding one dialect's quote style.
- Boolean SQL values use `commonTrueVal` and `commonFalseVal` (`true`/`false` on PostgreSQL, `1`/`0` elsewhere).
- `model/channel.go:channelGroupFilterCondition` branches because MySQL uses `CONCAT(',', column, ',')`, while SQLite/PostgreSQL use `(',' || column || ',')`.
- `model/db_time.go` has explicit dialect expressions for PostgreSQL (`EXTRACT(EPOCH FROM NOW())`), SQLite (`strftime('%s','now')`), and MySQL (`UNIX_TIMESTAMP()`). Follow this style for unavoidable dialect functions.
- Use bind placeholders (`?`) for values. Do not concatenate user-controlled values into SQL. If an identifier must be assembled, derive it from a fixed, validated list as in the migration helpers.
- ClickHouse-specific SQL belongs behind `common.UsingLogDatabase(common.DatabaseTypeClickHouse)` and only for log operations. `model/main.go` uses ClickHouse `MergeTree`/TTL DDL, and `model/log.go` switches ordering and filtering semantics for ClickHouse.

## Migrations

Migrations must be idempotent and safe to rerun during startup.

- `model/main.go:migrateDB` runs targeted compatibility migrations before `AutoMigrate`, then calls `AutoMigrate` for all root entities.
- SQLite cannot use the same column-alteration syntax as MySQL/PostgreSQL. `ensureSubscriptionPlanTableSQLite` checks table/columns and uses `ALTER TABLE ... ADD COLUMN`; it skips unsupported `ALTER COLUMN` operations.
- `migrateTokenModelLimitsToText` and `migrateSubscriptionPlanPriceAmount` first check table/column/type and then branch: PostgreSQL uses `ALTER COLUMN ... TYPE`, MySQL uses `MODIFY COLUMN`, while SQLite returns early because its type affinity makes the change unnecessary. Preserve the existing type check before emitting DDL.
- Do not add a database-specific `ALTER TABLE` to the generic migration path. If a schema change cannot be represented by GORM, add a helper with a branch and a repeat-run check, and add/extend a migration regression test such as `model/user_session_migration_test.go`.
- Avoid boolean default tags such as `gorm:"default:true"` when the default is a business rule already enforced by normalization or service logic. Cross-dialect boolean normalization can cause repeated `AutoMigrate` changes. Prefer code defaults; do not replace with `default:1` without checking all three primary databases.

## JSON and database values

JSON stored in model fields is represented as text or `json.RawMessage` according to the existing model. Actual marshal/unmarshal calls in root business code go through `common/json.go` (`common.Marshal`, `common.Unmarshal`, `common.UnmarshalJsonStr`, `common.DecodeJson`). Examples include `model/task.go`, `model/user.go`, `model/token.go`, and `model/system_task.go`. `json.RawMessage` may be used as a type, but do not call `encoding/json` directly for business serialization. The independent RelayKit module uses its own dependency-free `relaykit/relayconvert/kitutil` wrapper so it does not import root `common`.

When adding or changing a JSON-backed field, preserve the existing empty/default representation (`""`, `{}`, `[]`, or `nil`) and test both persistence and decoding. `model/channel.go` and `model/task.go` are examples of getter/setter methods that own this normalization.

## Verification expectations

At minimum, run the focused model tests and the root module test/build checks relevant to the change. For migrations, exercise the SQLite path and inspect all explicit PostgreSQL/MySQL branches. If `relaykit/` is touched, root tests are not enough: run `cd relaykit && GOWORK=off go build ./...` (and focused RelayKit tests when relevant).
