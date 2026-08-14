package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStrictMigrationRefusesActiveUnmappableTokens(t *testing.T) {
	db := setupRoutingMigrationTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO user_routing_group_grants (user_id, routing_group_id, expires_at)
		VALUES (1001, 1, 0)
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tokens (id, "group", routing_mode, routing_group_id)
		VALUES (1, '', 'fixed', 1), (2, '', 'fixed', 999)
	`).Error)

	report, err := MigrateRoutingGroupCompatibilityDataStrict()
	require.Error(t, err)
	require.NotNil(t, report)
	require.Contains(t, report.UnmappableTokens, 2)

	// Fail-closed: nothing may be written.
	var count int64
	require.NoError(t, db.Model(&model.UserGroupGrant{}).Count(&count).Error)
	require.EqualValues(t, 0, count)
	require.Equal(t, "", migratedTokenGroup(t, db, 1))
}

func TestStrictMigrationSucceedsWhenFullyMappable(t *testing.T) {
	db := setupRoutingMigrationTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO user_routing_group_grants (user_id, routing_group_id, expires_at)
		VALUES (1001, 1, 0)
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tokens (id, "group", routing_mode, routing_group_id)
		VALUES (1, '', 'fixed', 1)
	`).Error)

	report, err := MigrateRoutingGroupCompatibilityDataStrict()
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Empty(t, report.UnmappableTokens)

	var grants []model.UserGroupGrant
	require.NoError(t, db.Order("user_id ASC").Find(&grants).Error)
	require.Len(t, grants, 1)
	require.Equal(t, "vip", grants[0].GroupKey)
	require.Equal(t, "vip", migratedTokenGroup(t, db, 1))

	// Marker persisted.
	status, err := GetRoutingGroupMigrationStatus()
	require.NoError(t, err)
	assert.True(t, status.Migrated)
	assert.Zero(t, status.PendingGrants)
	assert.True(t, status.InSync)

	// Idempotent: re-running strictly succeeds without adding rows or pending work.
	report, err = MigrateRoutingGroupCompatibilityDataStrict()
	require.NoError(t, err)
	assert.Empty(t, report.GrantImports)
	var grantCount int64
	require.NoError(t, db.Model(&model.UserGroupGrant{}).Count(&grantCount).Error)
	require.EqualValues(t, 1, grantCount)
}

func TestRoutingGroupMigrationGrantReadFailureAbortsWithoutWritesOrMarker(t *testing.T) {
	db := setupRoutingMigrationTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO user_routing_group_grants (user_id, routing_group_id, expires_at)
		VALUES (1001, 1, 0)
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tokens (id, "group", routing_mode, routing_group_id)
		VALUES (1, '', 'fixed', 1)
	`).Error)

	queryErr := errors.New("legacy grant query failed")
	callbackName := "test:fail_legacy_routing_grant_query"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		tableName := tx.Statement.Table
		if tableName == "" && tx.Statement.Schema != nil {
			tableName = tx.Statement.Schema.Table
		}
		if tableName == "user_routing_group_grants" {
			tx.AddError(queryErr)
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Query().Remove(callbackName))
	})

	_, err := PreviewRoutingGroupMigration()
	require.ErrorIs(t, err, queryErr)
	ready, blockers, err := RoutingGroupMigrationReadiness()
	require.ErrorIs(t, err, queryErr)
	require.False(t, ready)
	require.Empty(t, blockers)
	require.ErrorIs(t, MigrateRoutingGroupCompatibilityData(), queryErr)
	report, err := MigrateRoutingGroupCompatibilityDataStrict()
	require.ErrorIs(t, err, queryErr)
	require.Nil(t, report)

	var grantCount int64
	require.NoError(t, db.Model(&model.UserGroupGrant{}).Count(&grantCount).Error)
	require.Zero(t, grantCount)
	require.Equal(t, "", migratedTokenGroup(t, db, 1))
	var markerCount int64
	require.NoError(t, db.Model(&model.Option{}).Where("key = ?", RoutingGroupMigrationVersionKey).Count(&markerCount).Error)
	require.Zero(t, markerCount)
	common.OptionMapRWMutex.RLock()
	marker := common.OptionMap[RoutingGroupMigrationVersionKey]
	common.OptionMapRWMutex.RUnlock()
	require.Empty(t, marker)
}

func TestStrictMigrationRefusesActiveOrphanGrant(t *testing.T) {
	db := setupRoutingMigrationTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO user_routing_group_grants (user_id, routing_group_id, expires_at)
		VALUES (1001, 999, 0)
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tokens (id, "group", routing_mode, routing_group_id)
		VALUES (1, '', 'fixed', 1)
	`).Error)

	report, err := MigrateRoutingGroupCompatibilityDataStrict()
	require.Error(t, err)
	require.NotNil(t, report)
	assert.Equal(t, []RoutingUnmappableGrantPreview{{
		UserId:         1001,
		RoutingGroupId: 999,
		ExpiresAt:      0,
	}}, report.UnmappableGrants)

	var grantCount int64
	require.NoError(t, db.Model(&model.UserGroupGrant{}).Count(&grantCount).Error)
	assert.Zero(t, grantCount)
	assert.Equal(t, "", migratedTokenGroup(t, db, 1))
	var markerCount int64
	require.NoError(t, db.Model(&model.Option{}).Where("key = ?", RoutingGroupMigrationVersionKey).Count(&markerCount).Error)
	assert.Zero(t, markerCount)
	common.OptionMapRWMutex.RLock()
	marker := common.OptionMap[RoutingGroupMigrationVersionKey]
	common.OptionMapRWMutex.RUnlock()
	assert.Empty(t, marker)
}

func TestStrictMigrationIgnoresExpiredOrphanGrant(t *testing.T) {
	db := setupRoutingMigrationTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO user_routing_group_grants (user_id, routing_group_id, expires_at)
		VALUES (?, 999, ?)
	`, 1001, common.GetTimestamp()-1).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tokens (id, "group", routing_mode, routing_group_id)
		VALUES (1, '', 'fixed', 1)
	`).Error)

	report, err := MigrateRoutingGroupCompatibilityDataStrict()
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Empty(t, report.UnmappableGrants)
	assert.Equal(t, "vip", migratedTokenGroup(t, db, 1))

	status, err := GetRoutingGroupMigrationStatus()
	require.NoError(t, err)
	assert.True(t, status.Migrated)
	assert.Zero(t, status.PendingGrants)
	assert.True(t, status.InSync)
}

func TestRoutingGroupMigrationReadiness(t *testing.T) {
	db := setupRoutingMigrationTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP'), (2, 'unknown-orphan')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tokens (id, "group", routing_mode, routing_group_id)
		VALUES (1, '', 'fixed', 2)
	`).Error)

	ready, blockers, err := RoutingGroupMigrationReadiness()
	require.NoError(t, err)
	require.False(t, ready)
	require.NotEmpty(t, blockers)
}
