package router

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelKeyRouteTest(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	previousRedis := common.RedisEnabled
	previousCriticalRateLimit := common.CriticalRateLimitEnable
	previousGlobalAPIRateLimit := common.GlobalApiRateLimitEnable

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Log{}))

	model.DB = db
	model.LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.CriticalRateLimitEnable = false
	common.GlobalApiRateLimitEnable = false

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedis
		common.CriticalRateLimitEnable = previousCriticalRateLimit
		common.GlobalApiRateLimitEnable = previousGlobalAPIRateLimit
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createChannelKeyRouteUser(t *testing.T, db *gorm.DB, username, accessToken string, role int) *model.User {
	t.Helper()
	user := &model.User{
		Username:    username,
		Password:    "password-placeholder",
		Role:        role,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AccessToken: &accessToken,
		AuthVersion: 1,
		AffCode:     "channel-key-route-" + username,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func TestChannelKeyRouteDoesNotRequireSecondaryProof(t *testing.T) {
	db := setupChannelKeyRouteTest(t)
	createChannelKeyRouteUser(t, db, "channel-admin", "channel-admin-pat", common.RoleAdminUser)
	root := createChannelKeyRouteUser(t, db, "channel-root", "channel-root-pat", common.RoleRootUser)
	channel := &model.Channel{Key: "channel-secret", Name: "test-channel"}
	require.NoError(t, db.Create(channel).Error)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	adminRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/channel/%d/key", channel.Id), nil)
	adminRequest.Header.Set("Authorization", "Bearer channel-admin-pat")
	adminResponse := httptest.NewRecorder()
	engine.ServeHTTP(adminResponse, adminRequest)
	assert.Equal(t, http.StatusForbidden, adminResponse.Code)

	rootRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/channel/%d/key", channel.Id), nil)
	rootRequest.Header.Set("Authorization", "Bearer channel-root-pat")
	rootResponse := httptest.NewRecorder()
	engine.ServeHTTP(rootResponse, rootRequest)

	require.Equal(t, http.StatusOK, rootResponse.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rootResponse.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, "channel-secret", body.Data.Key)
	assert.Equal(t, "no-store, no-cache, must-revalidate, private, max-age=0", rootResponse.Header().Get("Cache-Control"))

	var audit model.Log
	require.NoError(t, db.Where("user_id = ?", root.Id).Order("id DESC").First(&audit).Error)
	assert.Contains(t, audit.Other, "channel.key_view")
}
