package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetUsageAnalysisOptionsIncludesEnabledRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}))
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	root := model.User{Id: 101, Username: "root-admin", AffCode: "root-aff", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	member := model.User{Id: 102, Username: "member", AffCode: "member-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	disabledRoot := model.User{Id: 103, Username: "disabled-root", AffCode: "disabled-root-aff", Role: common.RoleRootUser, Status: common.UserStatusDisabled}
	require.NoError(t, db.Create(&[]model.User{root, member, disabledRoot}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 201, UserId: member.Id, Name: "member-key", Key: "sk-member", Status: common.TokenStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 301, Name: "test-channel", Key: "channel-key"}).Error)
	require.NoError(t, db.Create(&model.Log{
		UserId:    root.Id,
		CreatedAt: time.Now().Unix(),
		Type:      model.LogTypeConsume,
		ModelName: "gpt-test",
	}).Error)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/usage-analysis/options", nil)
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request

	GetUsageAnalysisOptions(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                         `json:"success"`
		Data    UsageAnalysisOptionsResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, root.Id, response.Data.RootUserID)
	assert.Contains(t, response.Data.Users, UsageAnalysisNamedOption{ID: root.Id, Name: root.Username})
	assert.NotContains(t, response.Data.Users, UsageAnalysisNamedOption{ID: disabledRoot.Id, Name: disabledRoot.Username})
	assert.Contains(t, response.Data.Tokens, UsageAnalysisTokenOption{ID: 201, UserID: member.Id, Name: "member-key"})
	assert.Contains(t, response.Data.Models, "gpt-test")
	assert.Contains(t, response.Data.Channels, UsageAnalysisNamedOption{ID: 301, Name: "test-channel"})
}

func TestParseUsageAnalysisQueryRejectsOversizedRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodGet, "/api/usage-analysis?start_timestamp=1&end_timestamp="+strconv.FormatInt(1+int64((usageAnalysisMaxRange+time.Second)/time.Second), 10), nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	_, err := parseUsageAnalysisQuery(context)

	assert.ErrorIs(t, err, errUsageAnalysisRangeTooLarge)
}

func TestParseUsageAnalysisQueryRejectsExtremeRangeWithoutDurationOverflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodGet, "/api/usage-analysis?start_timestamp=1&end_timestamp="+strconv.FormatInt(int64(^uint64(0)>>1), 10), nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	_, err := parseUsageAnalysisQuery(context)

	assert.ErrorIs(t, err, errUsageAnalysisRangeTooLarge)
}

func TestParseUsageAnalysisQueryAcceptsPageSizeOne(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().Unix()
	request := httptest.NewRequest(http.MethodGet, "/api/usage-analysis?start_timestamp="+strconv.FormatInt(now-60, 10)+"&end_timestamp="+strconv.FormatInt(now, 10)+"&page=1&page_size=1", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	query, err := parseUsageAnalysisQuery(context)

	assert.NoError(t, err)
	assert.Equal(t, 1, query.Page)
	assert.Equal(t, 1, query.PageSize)
}

func TestParseUsageAnalysisQueryAcceptsLargestSafeOffset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().Unix()
	maxInt := int(^uint(0) >> 1)
	largestSafePage := maxInt/2 + 1
	request := httptest.NewRequest(http.MethodGet, "/api/usage-analysis?start_timestamp="+strconv.FormatInt(now-60, 10)+"&end_timestamp="+strconv.FormatInt(now, 10)+"&page="+strconv.Itoa(largestSafePage)+"&page_size=2", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	query, err := parseUsageAnalysisQuery(context)

	assert.NoError(t, err)
	assert.Equal(t, largestSafePage, query.Page)
	assert.Equal(t, 2, query.PageSize)
}

func TestParseUsageAnalysisQueryRejectsOffsetOverflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().Unix()
	request := httptest.NewRequest(http.MethodGet, "/api/usage-analysis?start_timestamp="+strconv.FormatInt(now-60, 10)+"&end_timestamp="+strconv.FormatInt(now, 10)+"&page="+strconv.Itoa(int(^uint(0)>>1))+"&page_size=100", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	_, err := parseUsageAnalysisQuery(context)

	assert.ErrorIs(t, err, errUsageAnalysisInvalidPage)
}

func TestParseUsageAnalysisQueryRejectsReversedRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodGet, "/api/usage-analysis?start_timestamp=200&end_timestamp=100", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	_, err := parseUsageAnalysisQuery(context)

	assert.ErrorIs(t, err, errUsageAnalysisInvalidRange)
}

func TestParseUsageAnalysisQueryDefaultsInvalidPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().Unix()
	request := httptest.NewRequest(http.MethodGet, "/api/usage-analysis?start_timestamp="+strconv.FormatInt(now-60, 10)+"&end_timestamp="+strconv.FormatInt(now, 10)+"&page=invalid&page_size=0", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	query, err := parseUsageAnalysisQuery(context)

	assert.NoError(t, err)
	assert.Equal(t, 1, query.Page)
	assert.Equal(t, usageAnalysisDefaultPageSize, query.PageSize)
}

func TestParseUsageAnalysisQueryClampsPageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().Unix()
	request := httptest.NewRequest(http.MethodGet, "/api/usage-analysis?start_timestamp="+strconv.FormatInt(now-60, 10)+"&end_timestamp="+strconv.FormatInt(now, 10)+"&page=0&page_size=9999", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	query, err := parseUsageAnalysisQuery(context)

	assert.NoError(t, err)
	assert.Equal(t, 1, query.Page)
	assert.Equal(t, usageAnalysisMaxPageSize, query.PageSize)
}
