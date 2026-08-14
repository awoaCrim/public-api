package model

import (
	"github.com/QuantumNous/new-api/common"
)

// llmReviewRecentChannelWindowSeconds bounds the reference lookback for
// preflight (pre-channel-assignment) RPM triggers.
const llmReviewRecentChannelWindowSeconds int64 = 15 * 60

// HydrateLLMReviewTaskMetadata fills display metadata for a review task.
// Username/display name come from the current user profile; the real channel
// name is only filled when the task already carries a channel id. RPM
// preflight triggers happen before channel assignment, so their recent
// channel is kept in a separate reference field and never presented as the
// channel of this task.
func HydrateLLMReviewTaskMetadata(task *LLMReviewTask) {
	if task == nil {
		return
	}
	if user, err := GetUserById(task.UserId, false); err == nil && user != nil {
		if task.Username == "" {
			task.Username = user.Username
		}
		task.DisplayName = user.DisplayName
	}
	if task.ChannelId > 0 {
		task.ChannelAssignment = "assigned"
		if task.ChannelName == "" {
			if channel, err := GetChannelById(task.ChannelId, false); err == nil && channel != nil {
				task.ChannelName = channel.Name
			}
		}
		return
	}
	if task.TriggerType == LLMReviewTriggerRPM && task.Stage == LLMReviewStagePreflight {
		task.ChannelAssignment = "unassigned_preflight"
		if channelID, createdAt, ok := nearestPriorReviewLogChannel(task.UserId, task.ModelName, task.CreatedAt); ok {
			task.RecentChannelId = channelID
			task.RecentChannelAt = createdAt
			if channel, err := GetChannelById(channelID, false); err == nil && channel != nil {
				task.RecentChannelName = channel.Name
			}
		}
		return
	}
	task.ChannelAssignment = "unassigned"
}

// nearestPriorReviewLogChannel finds the most recent consume-log channel for
// the user/model shortly before the trigger.
func nearestPriorReviewLogChannel(userID int, modelName string, before int64) (channelID int, createdAt int64, ok bool) {
	if LOG_DB == nil || userID <= 0 || before <= 0 {
		return 0, 0, false
	}
	query := LOG_DB.Model(&Log{}).
		Select("channel_id", "created_at").
		Where("user_id = ? AND type = ? AND channel_id > 0 AND created_at <= ? AND created_at >= ?", userID, LogTypeConsume, before, before-llmReviewRecentChannelWindowSeconds)
	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	order := "created_at DESC, id DESC"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = "created_at DESC, request_id DESC"
	}
	var log Log
	if err := query.Order(order).First(&log).Error; err != nil {
		return 0, 0, false
	}
	return log.ChannelId, log.CreatedAt, log.ChannelId > 0
}
