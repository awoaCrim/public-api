package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

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
