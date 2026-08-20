package service

import (
	"context"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// enqueueTokenPostflightReviews asynchronously enqueues one or more
// postflight token over-limit triggers. gin.Context is only read
// synchronously; the goroutine uses the copied, bounded request context only.
// Multiple triggers enqueue sequentially in one goroutine so the first creates
// the active task and the rest merge into it.
func enqueueTokenPostflightReviews(ctx *gin.Context, triggers []LLMReviewTrigger) {
	requestContext := common.CaptureLLMReviewRequestContext(ctx)
	clientIP := ctx.ClientIP()
	endpoint := ctx.Request.URL.Path
	for i := range triggers {
		triggers[i].RequestSnippet = requestContext.Summary
		triggers[i].RequestBody = requestContext.Body
		triggers[i].RequestHeaders = requestContext.Headers
		triggers[i].ClientIP = clientIP
		triggers[i].Endpoint = endpoint
	}
	common.RelayCtxGo(context.Background(), func() {
		for _, trigger := range triggers {
			_ = EnqueueLLMReview(context.Background(), trigger)
		}
	})
}
