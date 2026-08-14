package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRoutingMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf(
		"file:routing_group_migration_%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		_ = sqlDB.Close()
	})

	require.NoError(t, db.AutoMigrate(&model.UserGroupGrant{}, &model.Option{}))
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previousMigrationVersion, hadPreviousMigrationVersion := common.OptionMap[RoutingGroupMigrationVersionKey]
	delete(common.OptionMap, RoutingGroupMigrationVersionKey)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		if hadPreviousMigrationVersion {
			common.OptionMap[RoutingGroupMigrationVersionKey] = previousMigrationVersion
		} else {
			delete(common.OptionMap, RoutingGroupMigrationVersionKey)
		}
		common.OptionMapRWMutex.Unlock()
	})
	require.NoError(t, db.Exec(`
		CREATE TABLE routing_groups (
			id INTEGER PRIMARY KEY,
			key TEXT NOT NULL
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE user_routing_group_grants (
			user_id INTEGER NOT NULL,
			routing_group_id INTEGER NOT NULL,
			expires_at INTEGER NOT NULL DEFAULT 0
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE tokens (
			id INTEGER PRIMARY KEY,
			"group" TEXT NOT NULL DEFAULT '',
			routing_mode TEXT NOT NULL DEFAULT 'fixed',
			routing_group_id INTEGER
		)
	`).Error)
	return db
}

func TestMigrateRoutingGroupCompatibilityData(t *testing.T) {
	db := setupRoutingMigrationTestDB(t)

	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP'), (2, 'not-in-catalog')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO user_routing_group_grants (user_id, routing_group_id, expires_at)
		VALUES (1001, 1, 100), (1001, 1, 200), (1002, 1, 0), (1003, 2, 0)
	`).Error)
	require.NoError(t, db.Create(&model.UserGroupGrant{
		UserId:    1001,
		GroupKey:  "vip",
		Source:    UserGroupGrantSourceManual,
		ExpiresAt: 50,
	}).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tokens (id, "group", routing_mode, routing_group_id)
		VALUES (1, '', 'fixed', 1),
		       (2, '', 'legacy_auto', NULL),
		       (3, 'AUTO', 'fixed', NULL),
		       (4, '', 'fixed', 2)
	`).Error)

	// Dry-run must never mutate and must report the unmappable references.
	report, err := PreviewRoutingGroupMigration()
	require.NoError(t, err)
	require.True(t, len(report.UnmappableGroups) >= 1, "unknown catalog keys must be reported")
	require.Contains(t, report.UnmappableTokens, 4)
	var countBefore int64
	require.NoError(t, db.Model(&model.UserGroupGrant{}).Count(&countBefore).Error)
	require.EqualValues(t, 1, countBefore, "dry-run must not write")

	require.NoError(t, MigrateRoutingGroupCompatibilityData())

	var grants []model.UserGroupGrant
	require.NoError(t, db.Order("user_id ASC").Find(&grants).Error)
	require.Len(t, grants, 2)
	require.Equal(t, 1001, grants[0].UserId)
	require.Equal(t, "vip", grants[0].GroupKey)
	require.Equal(t, UserGroupGrantSourceManual, grants[0].Source)
	require.EqualValues(t, 200, grants[0].ExpiresAt)
	require.Equal(t, 1002, grants[1].UserId)
	require.EqualValues(t, 0, grants[1].ExpiresAt)

	require.Equal(t, "vip", migratedTokenGroup(t, db, 1))
	require.Equal(t, "auto", migratedTokenGroup(t, db, 2))
	require.Equal(t, "auto", migratedTokenGroup(t, db, 3))
	require.Equal(t, "", migratedTokenGroup(t, db, 4), "unmappable token references must remain unchanged")

	require.NoError(t, MigrateRoutingGroupCompatibilityData())
	var grantCount int64
	require.NoError(t, db.Model(&model.UserGroupGrant{}).Count(&grantCount).Error)
	require.EqualValues(t, 2, grantCount, "migration must be idempotent")
	require.Equal(t, "vip", migratedTokenGroup(t, db, 1))
	require.EqualValues(t, 200, grantExpiry(t, db, 1001, "vip"))
}

func TestPreviewRoutingGroupMigrationReportsOnlyPendingGrantChanges(t *testing.T) {
	db := setupRoutingMigrationTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO user_routing_group_grants (user_id, routing_group_id, expires_at)
		VALUES (2001, 1, 1000),
		       (2002, 1, 2000),
		       (2003, 1, 3000),
		       (2004, 1, 4000),
		       (2005, 1, 5000)
	`).Error)
	require.NoError(t, db.Create(&[]model.UserGroupGrant{
		{UserId: 2002, GroupKey: "vip", Source: UserGroupGrantSourceManual, ExpiresAt: 2000},
		{UserId: 2003, GroupKey: "vip", Source: UserGroupGrantSourceManual, ExpiresAt: 0},
		{UserId: 2004, GroupKey: "vip", Source: UserGroupGrantSourceManual, ExpiresAt: 3500},
		{UserId: 2005, GroupKey: "vip", Source: UserGroupGrantSourceRoutingCompat, ExpiresAt: 5000},
	}).Error)

	report, err := PreviewRoutingGroupMigration()
	require.NoError(t, err)
	assert.ElementsMatch(t, []RoutingGrantImportPreview{
		{UserId: 2001, GroupKey: "vip", ExpiresAt: 1000},
		{UserId: 2004, GroupKey: "vip", ExpiresAt: 4000},
		{UserId: 2005, GroupKey: "vip", ExpiresAt: 5000},
	}, report.GrantImports)

	require.NoError(t, MigrateRoutingGroupCompatibilityData())
	status, err := GetRoutingGroupMigrationStatus()
	require.NoError(t, err)
	assert.Zero(t, status.PendingGrants)
	assert.True(t, status.InSync)

	var converted model.UserGroupGrant
	require.NoError(t, db.Where("user_id = ? AND group_key = ?", 2005, "vip").First(&converted).Error)
	assert.Equal(t, UserGroupGrantSourceManual, converted.Source)
}

func TestPreviewRoutingGroupMigrationNoLegacyTables(t *testing.T) {
	db := setupRoutingMigrationTestDB(t)
	require.NoError(t, db.Exec(`DROP TABLE routing_groups`).Error)
	require.NoError(t, db.Exec(`DROP TABLE user_routing_group_grants`).Error)
	require.NoError(t, db.Exec(`DROP TABLE tokens`).Error)

	report, err := PreviewRoutingGroupMigration()
	require.NoError(t, err)
	require.NotNil(t, report)
	require.NoError(t, MigrateRoutingGroupCompatibilityData())
}

func migratedTokenGroup(t *testing.T, db *gorm.DB, tokenID int) string {
	t.Helper()
	var group string
	require.NoError(t, db.Raw(`SELECT "group" FROM tokens WHERE id = ?`, tokenID).Scan(&group).Error)
	return group
}

func grantExpiry(t *testing.T, db *gorm.DB, userID int, groupKey string) int64 {
	t.Helper()
	var grant model.UserGroupGrant
	require.NoError(t, db.Where("user_id = ? AND group_key = ?", userID, groupKey).First(&grant).Error)
	return grant.ExpiresAt
}
