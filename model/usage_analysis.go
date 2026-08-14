package model

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const usageAnalysisBucketSeconds int64 = 3600

// UsageAnalysisQuery describes a bounded, paginated aggregation over consume logs.
type UsageAnalysisQuery struct {
	Context        context.Context
	StartTimestamp int64
	EndTimestamp   int64
	UserID         int
	TokenID        int
	ModelName      string
	ChannelID      int
	Page           int
	PageSize       int
}

type UsageAnalysisMetrics struct {
	RequestCount        int64   `json:"request_count" gorm:"column:request_count"`
	PromptTokens        int64   `json:"prompt_tokens" gorm:"column:prompt_tokens"`
	CompletionTokens    int64   `json:"completion_tokens" gorm:"column:completion_tokens"`
	TotalTokens         int64   `json:"total_tokens" gorm:"column:total_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens" gorm:"column:cache_read_tokens"`
	CacheWriteTokens    int64   `json:"cache_write_tokens" gorm:"column:cache_write_tokens"`
	CacheWriteTokens5m  int64   `json:"cache_write_tokens_5m" gorm:"column:cache_write_tokens_5m"`
	CacheWriteTokens1h  int64   `json:"cache_write_tokens_1h" gorm:"column:cache_write_tokens_1h"`
	InputTokensTotal    int64   `json:"input_tokens_total" gorm:"column:input_tokens_total"`
	Quota               int64   `json:"quota" gorm:"column:quota"`
	CacheRate           float64 `json:"cache_rate" gorm:"-"`
	LegacyRequestCount  int64   `json:"legacy_request_count" gorm:"column:legacy_request_count"`
	CacheRateReadTokens int64   `json:"-" gorm:"column:cache_rate_read_tokens"`
}

type UsageAnalysisRow struct {
	UserID      int    `json:"user_id" gorm:"column:user_id"`
	Username    string `json:"username" gorm:"column:username"`
	TokenID     int    `json:"token_id" gorm:"column:token_id"`
	TokenName   string `json:"token_name" gorm:"column:token_name"`
	ModelName   string `json:"model_name" gorm:"column:model_name"`
	ChannelID   int    `json:"channel_id" gorm:"column:channel_id"`
	ChannelName string `json:"channel_name" gorm:"-"`
	UsageAnalysisMetrics
}

type UsageAnalysisTrendPoint struct {
	Timestamp int64 `json:"timestamp" gorm:"column:timestamp"`
	UsageAnalysisMetrics
}

type UsageAnalysisResult struct {
	StartTimestamp int64                     `json:"start_timestamp"`
	EndTimestamp   int64                     `json:"end_timestamp"`
	BucketSeconds  int64                     `json:"bucket_seconds"`
	Page           int                       `json:"page"`
	PageSize       int                       `json:"page_size"`
	Total          int64                     `json:"total"`
	Summary        UsageAnalysisMetrics      `json:"summary"`
	Rows           []UsageAnalysisRow        `json:"rows"`
	Trend          []UsageAnalysisTrendPoint `json:"trend"`
}

func applyUsageAnalysisFilters(query *gorm.DB, filters UsageAnalysisQuery) *gorm.DB {
	if filters.UserID > 0 {
		query = query.Where("user_id = ?", filters.UserID)
	}
	if filters.TokenID > 0 {
		query = query.Where("token_id = ?", filters.TokenID)
	}
	if filters.ModelName != "" {
		query = query.Where("model_name = ?", filters.ModelName)
	}
	if filters.ChannelID > 0 {
		query = query.Where("channel_id = ?", filters.ChannelID)
	}
	return query
}

func usageAnalysisAggregateSelect() string {
	return `COUNT(*) AS request_count,
		COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
		COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS total_tokens,
		COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens,
		COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens,
		COALESCE(SUM(cache_write_tokens_5m), 0) AS cache_write_tokens_5m,
		COALESCE(SUM(cache_write_tokens_1h), 0) AS cache_write_tokens_1h,
		COALESCE(SUM(CASE WHEN input_tokens_total > 0 THEN input_tokens_total ELSE 0 END), 0) AS input_tokens_total,
		COALESCE(SUM(quota), 0) AS quota,
		COALESCE(SUM(CASE WHEN input_tokens_total > 0 THEN cache_read_tokens ELSE 0 END), 0) AS cache_rate_read_tokens,
		COALESCE(SUM(CASE WHEN input_tokens_total <= 0 THEN 1 ELSE 0 END), 0) AS legacy_request_count`
}

func usageAnalysisBucketExpression() string {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return "intDiv(created_at, 3600) * 3600"
	}
	return "created_at - (created_at % 3600)"
}

func finalizeUsageAnalysisMetrics(metrics *UsageAnalysisMetrics) {
	if metrics == nil || metrics.InputTokensTotal <= 0 {
		return
	}
	metrics.CacheRate = float64(metrics.CacheRateReadTokens) * 100 / float64(metrics.InputTokensTotal)
}

func finalizeUsageAnalysisRows(rows []UsageAnalysisRow) {
	for index := range rows {
		finalizeUsageAnalysisMetrics(&rows[index].UsageAnalysisMetrics)
	}
}

func finalizeUsageAnalysisTrend(points []UsageAnalysisTrendPoint) {
	for index := range points {
		finalizeUsageAnalysisMetrics(&points[index].UsageAnalysisMetrics)
	}
}

func GetUsageAnalysis(query UsageAnalysisQuery) (UsageAnalysisResult, error) {
	if LOG_DB == nil {
		return UsageAnalysisResult{}, fmt.Errorf("log database is not initialized")
	}
	result := UsageAnalysisResult{
		StartTimestamp: query.StartTimestamp,
		EndTimestamp:   query.EndTimestamp,
		BucketSeconds:  usageAnalysisBucketSeconds,
		Page:           query.Page,
		PageSize:       query.PageSize,
		Rows:           make([]UsageAnalysisRow, 0),
		Trend:          make([]UsageAnalysisTrendPoint, 0),
	}
	if result.Page <= 0 {
		result.Page = 1
	}
	if result.PageSize <= 0 {
		result.PageSize = 20
	}

	queryContext := query.Context
	if queryContext == nil {
		queryContext = context.Background()
	}
	baseQuery := LOG_DB.WithContext(queryContext).Model(&Log{}).
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, query.StartTimestamp, query.EndTimestamp)
	baseQuery = applyUsageAnalysisFilters(baseQuery, query)

	if err := baseQuery.Select(usageAnalysisAggregateSelect()).Scan(&result.Summary).Error; err != nil {
		return UsageAnalysisResult{}, err
	}
	finalizeUsageAnalysisMetrics(&result.Summary)

	groupedQuery := baseQuery.Session(&gorm.Session{}).
		Select(`user_id, username, token_id, token_name, model_name, channel_id, ` + usageAnalysisAggregateSelect()).
		Group("user_id, username, token_id, token_name, model_name, channel_id")
	countQuery := LOG_DB.WithContext(query.Context).Table("(?) AS usage_groups", groupedQuery)
	if err := countQuery.Count(&result.Total).Error; err != nil {
		return UsageAnalysisResult{}, err
	}

	if err := groupedQuery.
		Order("total_tokens DESC").
		Order("user_id ASC").
		Order("token_id ASC").
		Order("model_name ASC").
		Order("channel_id ASC").
		Limit(result.PageSize).
		Offset((result.Page - 1) * result.PageSize).
		Scan(&result.Rows).Error; err != nil {
		return UsageAnalysisResult{}, err
	}
	finalizeUsageAnalysisRows(result.Rows)

	bucketExpression := usageAnalysisBucketExpression()
	trendSelect := fmt.Sprintf("%s AS timestamp, %s", bucketExpression, usageAnalysisAggregateSelect())
	if err := baseQuery.Session(&gorm.Session{}).
		Select(trendSelect).
		Group(bucketExpression).
		Order("timestamp ASC").
		Scan(&result.Trend).Error; err != nil {
		return UsageAnalysisResult{}, err
	}
	finalizeUsageAnalysisTrend(result.Trend)
	return result, nil
}
