package controller

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// checkInputTokenPreflight runs the input-token hard limit before forwarding.
// It only blocks when: the limit setting is enabled, the user is not root,
// the model is not whitelisted, AND the estimator passed acceptance for the
// model. Everything else fails open — an unaccepted estimator never rejects a
// request. On a hit it asynchronously enqueues a review trigger and writes
// the OpenAI-compatible 429; the caller must stop forwarding and must not
// pre-consume quota.
func checkInputTokenPreflight(c *gin.Context, relayInfo *relaycommon.RelayInfo, estimate int) bool {
	banSetting := operation_setting.GetRateLimitBanSetting()
	if !banSetting.Enabled || banSetting.MaxInputTokens <= 0 {
		return false
	}
	// Root exemption.
	if model.IsRootUser(relayInfo.UserId) {
		return false
	}
	if operation_setting.IsModelRateLimitWhitelisted(relayInfo.OriginModelName) {
		return false
	}
	// Fail open: an estimator without accepted calibration must never reject.
	if !service.IsModelPreflightEligible(relayInfo.OriginModelName) {
		return false
	}
	if estimate <= banSetting.MaxInputTokens {
		return false
	}

	enqueueInputPreflightReview(c, relayInfo, estimate, banSetting.MaxInputTokens)

	logger.LogWarn(c.Request.Context(), fmt.Sprintf(
		"input token preflight exceeded: model=%s estimate=%d actual=0 limit=%d stage=preflight estimator=%s",
		relayInfo.OriginModelName, estimate, banSetting.MaxInputTokens, service.InputTokenEstimatorVersion))

	service.WriteRateLimitError(c, service.RateLimitExceededMessage, 60, 0, 0)
	return true
}

// enqueueInputPreflightReview asynchronously enqueues one input-token
// preflight trigger. gin.Context is only read synchronously (body snippet,
// endpoint, stream flag); the goroutine uses the extracted values only.
func enqueueInputPreflightReview(c *gin.Context, relayInfo *relaycommon.RelayInfo, estimate, limit int) {
	snippet := common.ExtractLLMReviewSnippetFromContext(c)
	endpoint := c.Request.URL.Path
	isStream := relayInfo.IsStream
	userId := relayInfo.UserId
	modelName := relayInfo.OriginModelName
	// ChannelMeta is only initialized inside the relay helpers, which run
	// AFTER the preflight check. Read the channel id through the embedded
	// pointer defensively.
	channelId := 0
	if relayInfo.ChannelMeta != nil {
		channelId = relayInfo.ChannelMeta.ChannelId
	}
	clientIP := c.ClientIP()
	common.RelayCtxGo(context.Background(), func() {
		_ = service.EnqueueLLMReview(context.Background(), service.LLMReviewTrigger{
			UserId:         userId,
			ModelName:      modelName,
			ChannelId:      channelId,
			Endpoint:       endpoint,
			IsStream:       isStream,
			TriggerType:    service.LLMReviewTriggerInputToken,
			Stage:          service.LLMReviewStagePreflight,
			CurrentValue:   estimate,
			LimitValue:     limit,
			EstimateInput:  estimate,
			RequestSnippet: snippet,
			ClientIP:       clientIP,
		})
	})
}

// relayModeIsTextTokenLimitEligible wraps the relay-constant text-mode gate
// for the controller call sites.
func relayModeIsTextTokenLimitEligible(mode int) bool {
	return relayconstant.IsTextTokenLimitMode(mode)
}
