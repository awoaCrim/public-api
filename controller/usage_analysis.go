package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	usageAnalysisDefaultRange      = 24 * time.Hour
	usageAnalysisMaxRange          = 90 * 24 * time.Hour
	usageAnalysisQueryTimeout      = 15 * time.Second
	usageAnalysisDefaultPageSize   = 20
	usageAnalysisMaxPageSize       = 100
	usageAnalysisMaxUserOptions    = 10_000
	usageAnalysisMaxTokenOptions   = 10_000
	usageAnalysisMaxModelOptions   = 5_000
	usageAnalysisMaxChannelOptions = 10_000
)

var (
	errUsageAnalysisInvalidRange  = errors.New("end timestamp must be after start timestamp")
	errUsageAnalysisRangeTooLarge = errors.New("usage analysis range exceeds 90 days")
	errUsageAnalysisInvalidPage   = errors.New("usage analysis page is too large")
)

type UsageAnalysisNamedOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type UsageAnalysisTokenOption struct {
	ID     int    `json:"id"`
	UserID int    `json:"user_id"`
	Name   string `json:"name"`
}

func parseUsageAnalysisQuery(c *gin.Context) (model.UsageAnalysisQuery, error) {
	now := time.Now()
	startTimestamp, err := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	if err != nil || startTimestamp <= 0 {
		startTimestamp = now.Add(-usageAnalysisDefaultRange).Unix()
	}
	endTimestamp, err := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if err != nil || endTimestamp <= 0 {
		endTimestamp = now.Unix()
	}
	if endTimestamp < startTimestamp {
		return model.UsageAnalysisQuery{}, errUsageAnalysisInvalidRange
	}
	if endTimestamp-startTimestamp > int64(usageAnalysisMaxRange/time.Second) {
		return model.UsageAnalysisQuery{}, errUsageAnalysisRangeTooLarge
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.Query("page_size"))
	if err != nil || pageSize < 1 {
		pageSize = usageAnalysisDefaultPageSize
	}
	if pageSize > usageAnalysisMaxPageSize {
		pageSize = usageAnalysisMaxPageSize
	}
	if page-1 > int(^uint(0)>>1)/pageSize {
		return model.UsageAnalysisQuery{}, errUsageAnalysisInvalidPage
	}

	query := model.UsageAnalysisQuery{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		Page:           page,
		PageSize:       pageSize,
		ModelName:      c.Query("model_name"),
	}
	query.UserID, _ = strconv.Atoi(c.Query("user_id"))
	query.TokenID, _ = strconv.Atoi(c.Query("token_id"))
	query.ChannelID, _ = strconv.Atoi(c.Query("channel_id"))
	return query, nil
}

// GetUsageAnalysisOptions returns root-only filter options without API key secrets.
func GetUsageAnalysisOptions(c *gin.Context) {
	queryContext, cancel := context.WithTimeout(c.Request.Context(), usageAnalysisQueryTimeout)
	defer cancel()

	users := make([]UsageAnalysisNamedOption, 0)
	if err := model.DB.WithContext(queryContext).Model(&model.User{}).
		Select("id, username AS name").
		Where("status = ?", common.UserStatusEnabled).
		Order("username ASC").
		Limit(usageAnalysisMaxUserOptions).
		Find(&users).Error; err != nil {
		writeUsageAnalysisQueryError(c, queryContext, err)
		return
	}

	tokens := make([]UsageAnalysisTokenOption, 0)
	if err := model.DB.WithContext(queryContext).Model(&model.Token{}).
		Select("id, user_id, name").
		Where("status = ?", common.TokenStatusEnabled).
		Order("user_id ASC, name ASC").
		Limit(usageAnalysisMaxTokenOptions).
		Find(&tokens).Error; err != nil {
		writeUsageAnalysisQueryError(c, queryContext, err)
		return
	}

	models := make([]string, 0)
	if err := model.LOG_DB.WithContext(queryContext).Model(&model.Log{}).
		Distinct("model_name").
		Where("type = ? AND created_at >= ? AND model_name <> ''", model.LogTypeConsume, time.Now().Add(-usageAnalysisMaxRange).Unix()).
		Order("model_name ASC").
		Limit(usageAnalysisMaxModelOptions).
		Pluck("model_name", &models).Error; err != nil {
		writeUsageAnalysisQueryError(c, queryContext, err)
		return
	}

	channels := make([]UsageAnalysisNamedOption, 0)
	if err := model.DB.WithContext(queryContext).Model(&model.Channel{}).
		Select("id, name").
		Order("name ASC").
		Limit(usageAnalysisMaxChannelOptions).
		Find(&channels).Error; err != nil {
		writeUsageAnalysisQueryError(c, queryContext, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"users":    users,
		"tokens":   tokens,
		"models":   models,
		"channels": channels,
	})
}

func writeUsageAnalysisQueryError(c *gin.Context, queryContext context.Context, err error) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(queryContext.Err(), context.DeadlineExceeded) {
		c.JSON(http.StatusGatewayTimeout, gin.H{"success": false, "message": "usage analysis query timed out"})
		return
	}
	common.ApiError(c, err)
}

func GetUsageAnalysis(c *gin.Context) {
	query, err := parseUsageAnalysisQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	queryContext, cancel := context.WithTimeout(c.Request.Context(), usageAnalysisQueryTimeout)
	defer cancel()
	query.Context = queryContext
	result, err := model.GetUsageAnalysis(query)
	if err != nil {
		writeUsageAnalysisQueryError(c, queryContext, err)
		return
	}

	channelIDs := make([]int, 0, len(result.Rows))
	seenChannels := make(map[int]struct{}, len(result.Rows))
	for _, row := range result.Rows {
		if row.ChannelID <= 0 {
			continue
		}
		if _, ok := seenChannels[row.ChannelID]; ok {
			continue
		}
		seenChannels[row.ChannelID] = struct{}{}
		channelIDs = append(channelIDs, row.ChannelID)
	}
	if len(channelIDs) > 0 {
		channels := make([]UsageAnalysisNamedOption, 0, len(channelIDs))
		if err := model.DB.WithContext(queryContext).Model(&model.Channel{}).Select("id, name").Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
			writeUsageAnalysisQueryError(c, queryContext, err)
			return
		}
		names := make(map[int]string, len(channels))
		for _, channel := range channels {
			names[channel.ID] = channel.Name
		}
		for index := range result.Rows {
			result.Rows[index].ChannelName = names[result.Rows[index].ChannelID]
		}
	}

	common.ApiSuccess(c, result)
}
