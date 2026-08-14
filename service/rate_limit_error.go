package service

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const RateLimitExceededMessage = "请求频率超过当前限制，请稍后重试"

type RateLimitReviewTrigger struct {
	UserID         int
	ModelName      string
	Endpoint       string
	CurrentValue   int
	LimitValue     int
	RequestSnippet string
	ClientIP       string
	IsStream       bool
}

// EnqueueRateLimitReview is the replaceable review-trigger seam for RPM
// limit hits. The default forwards the trigger into the LLM review pipeline,
// which persists audit-only skipped records when the review master switch or
// the user's state does not allow a reviewer call.
var EnqueueRateLimitReview = func(_ context.Context, trigger RateLimitReviewTrigger) error {
	return EnqueueLLMReview(context.Background(), LLMReviewTrigger{
		UserId:         trigger.UserID,
		ModelName:      trigger.ModelName,
		Endpoint:       trigger.Endpoint,
		CurrentValue:   trigger.CurrentValue,
		LimitValue:     trigger.LimitValue,
		RequestSnippet: trigger.RequestSnippet,
		ClientIP:       trigger.ClientIP,
		IsStream:       trigger.IsStream,
		TriggerType:    LLMReviewTriggerRPM,
		Stage:          LLMReviewStagePreflight,
	})
}

// WriteRateLimitError writes the OpenAI-compatible error contract expected by relay clients.
func WriteRateLimitError(c *gin.Context, message string, retryAfterSeconds int, limit int, remaining int) {
	if retryAfterSeconds < 1 {
		retryAfterSeconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(retryAfterSeconds))
	if limit > 0 {
		c.Header("X-RateLimit-Limit-Requests", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining-Requests", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset-Requests", strconv.FormatInt(time.Now().Add(time.Duration(retryAfterSeconds)*time.Second).Unix(), 10))
	}
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "rate_limit_error",
			"param":   nil,
			"code":    "rate_limit_exceeded",
		},
	})
	c.Abort()
}
