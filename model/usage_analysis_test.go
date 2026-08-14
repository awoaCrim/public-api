package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useUsageAnalysisTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB, previousLogDB := DB, LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		_ = sqlDB.Close()
	})
	return db
}

func TestUsageAnalysisRejectsMissingLogDatabase(t *testing.T) {
	previousLogDB := LOG_DB
	LOG_DB = nil
	t.Cleanup(func() { LOG_DB = previousLogDB })

	_, err := GetUsageAnalysis(UsageAnalysisQuery{
		StartTimestamp: 1,
		EndTimestamp:   2,
		Page:           1,
		PageSize:       20,
	})

	require.ErrorContains(t, err, "log database is not initialized")
}

func TestUsageAnalysisAggregatesStructuredCacheMetrics(t *testing.T) {
	db := useUsageAnalysisTestDB(t)

	logs := []Log{
		{
			UserId:             1,
			Username:           "alice",
			TokenId:            10,
			TokenName:          "primary",
			ModelName:          "gpt-test",
			ChannelId:          7,
			CreatedAt:          3_600,
			Type:               LogTypeConsume,
			PromptTokens:       100,
			CompletionTokens:   20,
			CacheReadTokens:    40,
			CacheWriteTokens:   15,
			CacheWriteTokens5m: 10,
			CacheWriteTokens1h: 5,
			InputTokensTotal:   100,
			Quota:              12,
		},
		{
			UserId:             1,
			Username:           "alice",
			TokenId:            10,
			TokenName:          "primary",
			ModelName:          "gpt-test",
			ChannelId:          7,
			CreatedAt:          3_900,
			Type:               LogTypeConsume,
			PromptTokens:       80,
			CompletionTokens:   10,
			CacheReadTokens:    20,
			CacheWriteTokens:   4,
			CacheWriteTokens5m: 4,
			InputTokensTotal:   80,
			Quota:              8,
		},
		{
			UserId:           2,
			Username:         "bob",
			TokenId:          20,
			TokenName:        "secondary",
			ModelName:        "claude-test",
			ChannelId:        8,
			CreatedAt:        7_300,
			Type:             LogTypeConsume,
			PromptTokens:     30,
			CompletionTokens: 5,
			CacheReadTokens:  30,
			InputTokensTotal: 60,
			Quota:            6,
		},
	}
	require.NoError(t, db.Create(&logs).Error)

	result, err := GetUsageAnalysis(UsageAnalysisQuery{
		StartTimestamp: 3_000,
		EndTimestamp:   8_000,
		Page:           1,
		PageSize:       10,
	})
	require.NoError(t, err)
	require.Len(t, result.Rows, 2)
	assert.Equal(t, int64(2), result.Total)
	assert.Equal(t, int64(3), result.Summary.RequestCount)
	assert.Equal(t, int64(210), result.Summary.PromptTokens)
	assert.Equal(t, int64(35), result.Summary.CompletionTokens)
	assert.Equal(t, int64(90), result.Summary.CacheReadTokens)
	assert.Equal(t, int64(19), result.Summary.CacheWriteTokens)
	assert.Equal(t, int64(14), result.Summary.CacheWriteTokens5m)
	assert.Equal(t, int64(5), result.Summary.CacheWriteTokens1h)
	assert.Equal(t, int64(240), result.Summary.InputTokensTotal)
	assert.InDelta(t, 37.5, result.Summary.CacheRate, 0.001)

	assert.Equal(t, "alice", result.Rows[0].Username)
	assert.Equal(t, int64(2), result.Rows[0].RequestCount)
	assert.Equal(t, int64(60), result.Rows[0].CacheReadTokens)
	assert.Equal(t, int64(19), result.Rows[0].CacheWriteTokens)
	assert.InDelta(t, 33.333333, result.Rows[0].CacheRate, 0.001)

	require.Len(t, result.Trend, 2)
	assert.Equal(t, int64(3_600), result.Trend[0].Timestamp)
	assert.Equal(t, int64(60), result.Trend[0].CacheReadTokens)
	assert.Equal(t, int64(7_200), result.Trend[1].Timestamp)
	assert.Equal(t, int64(30), result.Trend[1].CacheReadTokens)
}

func TestUsageAnalysisPaginatesGroupedRowsWithoutChangingSummary(t *testing.T) {
	db := useUsageAnalysisTestDB(t)

	logs := []Log{
		{UserId: 1, Username: "alice", TokenId: 10, TokenName: "a", ModelName: "m1", ChannelId: 1, CreatedAt: 100, Type: LogTypeConsume, PromptTokens: 100, CompletionTokens: 10, InputTokensTotal: 100},
		{UserId: 2, Username: "bob", TokenId: 20, TokenName: "b", ModelName: "m2", ChannelId: 2, CreatedAt: 101, Type: LogTypeConsume, PromptTokens: 50, CompletionTokens: 5, InputTokensTotal: 50},
		{UserId: 3, Username: "carol", TokenId: 30, TokenName: "c", ModelName: "m3", ChannelId: 3, CreatedAt: 102, Type: LogTypeConsume, PromptTokens: 25, CompletionTokens: 3, InputTokensTotal: 25},
	}
	require.NoError(t, db.Create(&logs).Error)

	result, err := GetUsageAnalysis(UsageAnalysisQuery{
		StartTimestamp: 1,
		EndTimestamp:   200,
		Page:           2,
		PageSize:       1,
	})
	require.NoError(t, err)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, int64(3), result.Total)
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 1, result.PageSize)
	assert.Equal(t, "bob", result.Rows[0].Username)
	assert.Equal(t, int64(3), result.Summary.RequestCount)
	assert.Equal(t, int64(175), result.Summary.PromptTokens)
}

func TestUsageAnalysisCountsLegacyRowsWithoutParsingMetadata(t *testing.T) {
	db := useUsageAnalysisTestDB(t)

	logs := []Log{
		{
			UserId:           1,
			Username:         "alice",
			TokenId:          10,
			TokenName:        "legacy",
			ModelName:        "legacy-model",
			ChannelId:        1,
			CreatedAt:        100,
			Type:             LogTypeConsume,
			PromptTokens:     100,
			CompletionTokens: 20,
			Other:            `{"cache_tokens":75,"cache_write_tokens":25,"input_tokens_total":100}`,
		},
		{
			UserId:           1,
			Username:         "alice",
			TokenId:          10,
			TokenName:        "current",
			ModelName:        "current-model",
			ChannelId:        1,
			CreatedAt:        101,
			Type:             LogTypeConsume,
			PromptTokens:     100,
			CompletionTokens: 20,
			CacheReadTokens:  25,
			InputTokensTotal: 100,
		},
	}
	require.NoError(t, db.Create(&logs).Error)

	result, err := GetUsageAnalysis(UsageAnalysisQuery{
		StartTimestamp: 1,
		EndTimestamp:   200,
		Page:           1,
		PageSize:       10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Summary.LegacyRequestCount)
	assert.Equal(t, int64(25), result.Summary.CacheReadTokens)
	assert.Zero(t, result.Summary.CacheWriteTokens)
	assert.InDelta(t, 25, result.Summary.CacheRate, 0.001)
}
