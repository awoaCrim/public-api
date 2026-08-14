package service

import (
	"context"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// enqueueTokenPostflightReviews asynchronously enqueues one or more
// postflight token over-limit triggers. gin.Context is only read
// synchronously (snippet, client IP, endpoint); the goroutine uses the
// extracted values only. Multiple triggers enqueue sequentially in one
// goroutine so the first creates the active task and the rest merge into it.
func enqueueTokenPostflightReviews(ctx *gin.Context, triggers []LLMReviewTrigger) {
	snippet := common.ExtractLLMReviewSnippetFromContext(ctx)
	clientIP := ctx.ClientIP()
	endpoint := ctx.Request.URL.Path
	for i := range triggers {
		triggers[i].RequestSnippet = snippet
		triggers[i].ClientIP = clientIP
		triggers[i].Endpoint = endpoint
	}
	common.RelayCtxGo(context.Background(), func() {
		for _, trigger := range triggers {
			_ = EnqueueLLMReview(context.Background(), trigger)
		}
	})
}
