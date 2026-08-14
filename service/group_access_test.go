package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupGroupResolverTestDB(t *testing.T, group string) (*gorm.DB, int) {
	t.Helper()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	userID := 930000 + len(t.Name())
	dsn := fmt.Sprintf("file:group_resolver_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserGroupGrant{}))
	require.NoError(t, db.Create(&model.User{
		Id:       userID,
		Username: "group-resolver-user",
		AffCode:  "group-resolver-aff",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    group,
	}).Error)

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db, userID
}

func TestResolveGroupSelectionFixedAutoRequestedAndRevoked(t *testing.T) {
	db, userID := setupGroupResolverTestDB(t, "default")

	require.NoError(t, db.Create(&model.UserGroupGrant{
		UserId:   userID,
		GroupKey: "svip",
		Source:   UserGroupGrantSourceManual,
	}).Error)

	fixed, err := ResolveGroupSelection(userID, " svip ", "")
	require.NoError(t, err)
	require.Equal(t, "svip", fixed.TokenGroup)
	require.Equal(t, "svip", fixed.UsingGroup)
	require.Contains(t, fixed.EffectiveGroups, "default")
	require.Contains(t, fixed.EffectiveGroups, "svip")

	auto, err := ResolveGroupSelection(userID, "auto", "")
	require.NoError(t, err)
	require.Equal(t, "auto", auto.TokenGroup)
	require.Equal(t, "auto", auto.UsingGroup)
	require.Contains(t, auto.AutoGroups, "default")

	requested, err := ResolveGroupSelection(userID, "auto", "svip")
	require.NoError(t, err)
	require.Equal(t, "svip", requested.UsingGroup)
	require.Equal(t, "svip", requested.RequestedGroup)

	fixedSameGroup, err := ResolveGroupSelection(userID, "svip", "svip")
	require.NoError(t, err)
	require.Equal(t, "svip", fixedSameGroup.UsingGroup)

	_, err = ResolveGroupSelection(userID, "default", "svip")
	require.ErrorIs(t, err, ErrRoutingGroupNotGranted,
		"a fixed token must not switch to another group even when the user is granted that group")

	require.ErrorIs(t, ValidateRequestedGroup(userID, "auto"), ErrRoutingGroupInvalid)
	require.ErrorIs(t, ValidateRequestedGroup(userID, "unknown"), ErrRoutingGroupNotGranted)
	require.ErrorIs(t, ValidateRequestedGroup(userID, ""), ErrRoutingGroupInvalid)

	require.NoError(t, db.Where("user_id = ? AND group_key = ?", userID, "svip").Delete(&model.UserGroupGrant{}).Error)
	_, err = ResolveGroupSelection(userID, "svip", "")
	require.ErrorIs(t, err, ErrRoutingGroupNotGranted)
	_, err = ResolveGroupSelection(userID, "auto", "svip")
	require.ErrorIs(t, err, ErrRoutingGroupNotGranted)
}

func TestResolveGroupSelectionEmptyTokenUsesUserGroup(t *testing.T) {
	_, userID := setupGroupResolverTestDB(t, "vip")

	selection, err := ResolveGroupSelection(userID, "", "")
	require.NoError(t, err)
	require.Equal(t, "vip", selection.TokenGroup)
	require.Equal(t, "vip", selection.UsingGroup)
}

func TestResolveGroupSelectionRootCanUseUnknownFixedGroup(t *testing.T) {
	db, userID := setupGroupResolverTestDB(t, "default")
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
		"role": common.RoleRootUser,
	}).Error)

	selection, err := ResolveGroupSelection(userID, "root-only-group", "")
	require.NoError(t, err)
	require.Equal(t, "root-only-group", selection.UsingGroup)
}

func TestResolveUserGroupAccessUsesTransactionUserGroup(t *testing.T) {
	db, userID := setupGroupResolverTestDB(t, "default")

	tx := db.Begin()
	require.NoError(t, tx.Model(&model.User{}).Where("id = ?", userID).Update("group", "vip").Error)
	access, err := ResolveUserGroupAccess(tx, userID, "vip")
	require.NoError(t, err)
	require.True(t, access.Inherited["vip"])
	require.True(t, access.Inherited["default"], "the account-tier catalog inherits default even after changing the user tier")
	require.NoError(t, tx.Rollback().Error)
}

func TestGetAutoGroupsForUserUsesConfiguredOrder(t *testing.T) {
	original := setting.AutoGroups2JsonString()
	t.Cleanup(func() { require.NoError(t, setting.UpdateAutoGroupsByJsonString(original)) })
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["svip","default"]`))

	groups := map[string]string{"default": "default", "svip": "svip"}
	require.Equal(t, []string{"svip", "default"}, GetAutoGroupsForUser(groups))
}

func TestResolveUserGroupAccessIgnoresExpiredGrant(t *testing.T) {
	db, userID := setupGroupResolverTestDB(t, "default")
	require.NoError(t, db.Create(&model.UserGroupGrant{
		UserId:    userID,
		GroupKey:  "svip",
		Source:    UserGroupGrantSourceManual,
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	}).Error)

	access, err := ResolveUserGroupAccess(db, userID, "default")
	require.NoError(t, err)
	require.NotContains(t, access.Groups, "svip")
}

func TestGetUserGroupAccessMergesInheritedAndActiveGrants(t *testing.T) {
	userID := 910001
	db, _ := setupGroupResolverTestDB(t, "default")
	require.NoError(t, db.Create(&model.User{
		Id:       userID,
		Username: "group-access-user",
		AffCode:  "group-access-aff",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	t.Cleanup(func() {
		db.Where("user_id = ?", userID).Delete(&model.UserGroupGrant{})
		db.Unscoped().Where("id = ?", userID).Delete(&model.User{})
	})
	require.NoError(t, db.Create(&model.UserGroupGrant{
		UserId:    userID,
		GroupKey:  "svip",
		Source:    UserGroupGrantSourceManual,
		ExpiresAt: 0,
		SortOrder: 1,
	}).Error)
	require.NoError(t, db.Create(&model.UserGroupGrant{
		UserId:    userID,
		GroupKey:  "vip",
		Source:    UserGroupGrantSourceManual,
		ExpiresAt: time.Now().Unix() - 1,
		SortOrder: 2,
	}).Error)

	access, err := GetUserGroupAccess(userID)
	require.NoError(t, err)
	require.True(t, access.Inherited["default"])
	require.True(t, access.Granted["svip"])
	require.Contains(t, access.Groups, "svip")
	require.NotContains(t, access.Granted, "vip", "expired grants must not be reported as active grants")

	allowed, err := IsUserGroupAllowed(userID, "svip")
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = IsUserGroupAllowed(userID, "unknown-group")
	require.NoError(t, err)
	require.False(t, allowed)
}
