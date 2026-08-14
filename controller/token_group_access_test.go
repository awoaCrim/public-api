package controller

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTokenGroupAccessTestDB(t *testing.T) (*gorm.DB, int) {
	t.Helper()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	userID := 920001
	dsn := fmt.Sprintf("file:token_group_access_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserGroupGrant{}))
	require.NoError(t, db.Create(&model.User{
		Id:       userID,
		Username: "token-group-access-user",
		AffCode:  "token-group-access-aff",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		db.Unscoped().Where("id = ?", userID).Delete(&model.User{})
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db, userID
}

func TestResolveTokenRoutingMutationAcceptsGrantedFixedGroup(t *testing.T) {
	db, userID := setupTokenGroupAccessTestDB(t)

	require.NoError(t, db.Create(&model.UserGroupGrant{
		UserId:   userID,
		GroupKey: "svip",
		Source:   service.UserGroupGrantSourceManual,
	}).Error)

	token := &model.Token{Group: "svip"}
	require.NoError(t, resolveTokenRoutingMutation(userID, token))
	require.Equal(t, "svip", token.Group)

	require.NoError(t, db.Where("user_id = ? AND group_key = ?", userID, "svip").Delete(&model.UserGroupGrant{}).Error)
	deniedToken := &model.Token{Group: "svip"}
	require.ErrorIs(t, resolveTokenRoutingMutation(userID, deniedToken), service.ErrRoutingGroupNotGranted)
}

func TestResolveTokenRoutingMutationRejectsInvalidCombinations(t *testing.T) {
	_, userID := setupTokenGroupAccessTestDB(t)

	require.Error(t, resolveTokenRoutingMutation(userID, &model.Token{Group: ""}))

	autoToken := &model.Token{Group: "auto"}
	require.NoError(t, resolveTokenRoutingMutation(userID, autoToken))
	require.Equal(t, "auto", autoToken.Group)

	fixedRetry := &model.Token{Group: "default", CrossGroupRetry: true}
	require.ErrorIs(t, resolveTokenRoutingMutation(userID, fixedRetry), service.ErrRoutingAutoNotAllowed)
}

func TestUpdateUserGroupGrantsPresenceSemantics(t *testing.T) {
	db, userID := setupTokenGroupAccessTestDB(t)

	// Non-nil replaces: inherited default skipped, svip granted.
	require.NoError(t, updateUserGroupGrants(db, userID, []string{"default", "svip"}))
	var grants []model.UserGroupGrant
	require.NoError(t, db.Where("user_id = ?", userID).Find(&grants).Error)
	require.Len(t, grants, 1)
	require.Equal(t, "svip", grants[0].GroupKey)

	// Explicit empty slice clears the manual set.
	txClear := db.Begin()
	require.NoError(t, updateUserGroupGrants(txClear, userID, []string{}))
	require.NoError(t, txClear.Commit().Error)
	require.NoError(t, db.Where("user_id = ?", userID).Find(&grants).Error)
	require.Empty(t, grants)

	// Inherited groups are skipped (vip belongs to the default tier's usable
	// groups); svip is a pure grant. Keys outside the catalog are rejected
	// atomically: the previous set survives because the caller's transaction
	// rolls back.
	txGrant := db.Begin()
	require.NoError(t, updateUserGroupGrants(txGrant, userID, []string{"vip", "svip"}))
	require.NoError(t, txGrant.Commit().Error)
	require.NoError(t, db.Where("user_id = ?", userID).Find(&grants).Error)
	require.Len(t, grants, 1)
	require.Equal(t, "svip", grants[0].GroupKey)

	txReject := db.Begin()
	require.Error(t, updateUserGroupGrants(txReject, userID, []string{"ghost-group"}))
	require.NoError(t, txReject.Rollback().Error)
	require.NoError(t, db.Where("user_id = ?", userID).Find(&grants).Error)
	require.Len(t, grants, 1)
	require.Equal(t, "svip", grants[0].GroupKey)
}
