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

func setupRequestSnapshotRouteTest(t *testing.T) {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	previousRedis := common.RedisEnabled
	previousSecret := common.SessionSecret

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.RequestSnapshot{}, &model.RequestSnapshotAccess{}))
	model.DB = db
	model.LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.SessionSecret = "request-snapshot-route-test-secret"

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousType, previousLogType)
		common.RedisEnabled = previousRedis
		common.SessionSecret = previousSecret
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func createRequestSnapshotRouteUser(t *testing.T, username, accessToken string, role int) *model.User {
	t.Helper()
	user := &model.User{
		Username:    username,
		Password:    "password-placeholder",
		Role:        role,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AccessToken: &accessToken,
		AuthVersion: 1,
		AffCode:     "snapshot-route-" + username,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func TestRequestSnapshotRouteRequiresRootWithoutSecondaryProof(t *testing.T) {
	setupRequestSnapshotRouteTest(t)
	createRequestSnapshotRouteUser(t, "snapshot-admin", "snapshot-admin-pat", common.RoleAdminUser)
	root := createRequestSnapshotRouteUser(t, "snapshot-root", "snapshot-root-pat", common.RoleRootUser)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	adminRequest := httptest.NewRequest(http.MethodGet, "/api/log/req-admin/snapshot", nil)
	adminRequest.Header.Set("Authorization", "Bearer snapshot-admin-pat")
	adminResponse := httptest.NewRecorder()
	engine.ServeHTTP(adminResponse, adminRequest)

	assert.Equal(t, http.StatusForbidden, adminResponse.Code)
	assert.Contains(t, adminResponse.Body.String(), "auth.insufficient_privilege")
	var adminAccessCount int64
	require.NoError(t, model.DB.Model(&model.RequestSnapshotAccess{}).
		Where("request_id = ?", "req-admin").Count(&adminAccessCount).Error)
	assert.Zero(t, adminAccessCount, "non-root requests must be rejected before the snapshot handler")

	rootRequest := httptest.NewRequest(http.MethodGet, "/api/log/req-root/snapshot", nil)
	rootRequest.Header.Set("Authorization", "Bearer snapshot-root-pat")
	rootResponse := httptest.NewRecorder()
	engine.ServeHTTP(rootResponse, rootRequest)

	assert.Equal(t, http.StatusNotFound, rootResponse.Code)
	assert.Contains(t, rootResponse.Body.String(), "SNAPSHOT_NOT_FOUND")
	assert.Equal(t, "no-store, no-cache, must-revalidate, private, max-age=0", rootResponse.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", rootResponse.Header().Get("Pragma"))
	assert.Equal(t, "0", rootResponse.Header().Get("Expires"))
	var access model.RequestSnapshotAccess
	require.NoError(t, model.DB.Where("request_id = ?", "req-root").First(&access).Error)
	assert.Equal(t, root.Id, access.OperatorId)
	assert.Equal(t, "snapshot-root", access.Operator)
	assert.False(t, access.Success)
	assert.Equal(t, model.SnapshotResultNotFound, access.Result)
}
