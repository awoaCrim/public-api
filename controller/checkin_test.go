package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCheckinControllerTest(t *testing.T) {
	t.Helper()

	previousDB := model.DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousQuotaPerUnit := common.QuotaPerUnit
	previousSetting := *operation_setting.GetCheckinSetting()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Checkin{}))

	model.DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.QuotaPerUnit = 500000
	require.NoError(t, i18n.Init())

	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		common.RedisEnabled = previousRedisEnabled
		common.QuotaPerUnit = previousQuotaPerUnit
		*operation_setting.GetCheckinSetting() = previousSetting
		_ = sqlDB.Close()
	})
}

func callDoCheckin(t *testing.T, userID int) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/checkin", nil)
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Set("id", userID)
	DoCheckin(context)
	return recorder
}

func TestDoCheckinLocalizesBalanceThresholdRejection(t *testing.T) {
	setupCheckinControllerTest(t)

	const userID = 98201
	setting := operation_setting.GetCheckinSetting()
	setting.Enabled = true
	setting.BalanceThresholdEnabled = true
	setting.BalanceThreshold = 1
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userID,
		Username: "checkin-controller-threshold",
		Password: "password123",
		Quota:    500000,
	}).Error)

	recorder := callDoCheckin(t, userID)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Check-in is unavailable because your current balance has reached the configured threshold")
	assert.NotContains(t, recorder.Body.String(), "check-in balance threshold reached")
}

func TestDoCheckinHidesBalanceReadFailureBehindGenericError(t *testing.T) {
	setupCheckinControllerTest(t)

	setting := operation_setting.GetCheckinSetting()
	setting.Enabled = true
	setting.BalanceThresholdEnabled = true
	setting.BalanceThreshold = 1

	recorder := callDoCheckin(t, 98202)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Database error, please contact the administrator")
	assert.NotContains(t, recorder.Body.String(), "record not found")
}
