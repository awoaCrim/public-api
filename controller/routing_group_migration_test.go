package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRoutingMigrationControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:routing_migration_controller_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
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

	require.NoError(t, db.AutoMigrate(&model.UserGroupGrant{}, &model.Option{}))
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMapRWMutex.Unlock()
	require.NoError(t, db.Exec(`
		CREATE TABLE routing_groups (id INTEGER PRIMARY KEY, key TEXT NOT NULL)
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

func TestRoutingGroupMigrationRunIsFailClosed(t *testing.T) {
	db := setupRoutingMigrationControllerTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tokens (id, "group", routing_mode, routing_group_id)
		VALUES (1, '', 'fixed', 999)
	`).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/routing-group-migration/run", nil)
	c.Set("id", 1)
	c.Set("role", common.RoleRootUser)

	RoutingGroupMigrationRun(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.False(t, body.Success, "blocked runs must fail closed")

	var count int64
	require.NoError(t, db.Model(&model.UserGroupGrant{}).Count(&count).Error)
	require.EqualValues(t, 0, count, "nothing may be written when blocked")
}

func TestRoutingGroupMigrationPreviewIsReadOnly(t *testing.T) {
	db := setupRoutingMigrationControllerTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO user_routing_group_grants (user_id, routing_group_id, expires_at)
		VALUES (1001, 1, 0)
	`).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/routing-group-migration/preview", nil)
	c.Set("id", 1)
	c.Set("role", common.RoleRootUser)

	RoutingGroupMigrationPreview(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			GrantImports []map[string]any `json:"grant_imports"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Len(t, body.Data.GrantImports, 1)

	var count int64
	require.NoError(t, db.Model(&model.UserGroupGrant{}).Count(&count).Error)
	require.EqualValues(t, 0, count, "preview must never write")
}

func TestRoutingGroupMigrationStatusReportsReadiness(t *testing.T) {
	db := setupRoutingMigrationControllerTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO routing_groups (id, key) VALUES (1, 'VIP')`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tokens (id, "group", routing_mode, routing_group_id)
		VALUES (1, '', 'fixed', 999)
	`).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/routing-group-migration/status", nil)
	c.Set("id", 1)
	c.Set("role", common.RoleRootUser)

	RoutingGroupMigrationStatus(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Ready    bool     `json:"ready"`
			Blockers []string `json:"blockers"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.False(t, body.Data.Ready)
	require.NotEmpty(t, body.Data.Blockers)
}
