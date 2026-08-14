package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newChatCompletionsTestContext(t *testing.T, body, contentType string) (*gin.Context, *httptest.ResponseRecorder, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{contentType}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenCodeGo},
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
	}
	info.UpstreamModelName = "test-model"
	info.SetEstimatePromptTokens(500)
	return c, recorder, resp, info
}

// TestOaiStreamHandlerUsesUsageFromMiddleChunk covers the core streaming
// regression: a billable usage chunk in the middle of the SSE stream must be
// used even when a trailing empty chunk (no usage) and [DONE] follow it, and
// must not fall back to the local token estimate.
func TestOaiStreamHandlerUsesUsageFromMiddleChunk(t *testing.T) {
	body := strings.Join([]string{
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":20,"total_tokens":1020,"prompt_cache_hit_tokens":900}}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{}}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	c, recorder, resp, info := newChatCompletionsTestContext(t, body, "text/event-stream")

	usage, apiErr := OaiStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 1000, usage.PromptTokens)
	assert.Equal(t, 20, usage.CompletionTokens)
	assert.Equal(t, 1020, usage.TotalTokens)
	assert.Equal(t, 900, usage.PromptTokensDetails.CachedTokens)
	// Upstream usage must win: the local estimator must not have run.
	assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens))
	assert.Contains(t, recorder.Body.String(), "data: [DONE]")
}

// TestOaiStreamHandlerMergesSplitUsageAndCacheChunks covers a totals chunk
// followed by a cache-only chunk (prompt_cache_hit_tokens) and a trailing
// empty chunk: the cache must be merged into the totals usage and the standard
// totals must not be overwritten.
func TestOaiStreamHandlerMergesSplitUsageAndCacheChunks(t *testing.T) {
	body := strings.Join([]string{
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{}}],"usage":{"prompt_tokens":2000,"completion_tokens":50,"total_tokens":2050}}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{}}],"usage":{"prompt_cache_hit_tokens":1500}}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	c, recorder, resp, info := newChatCompletionsTestContext(t, body, "text/event-stream")

	usage, apiErr := OaiStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 2000, usage.PromptTokens)
	assert.Equal(t, 50, usage.CompletionTokens)
	assert.Equal(t, 2050, usage.TotalTokens)
	assert.Equal(t, 1500, usage.PromptTokensDetails.CachedTokens)
	assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens))
	assert.Contains(t, recorder.Body.String(), "data: [DONE]")
}

// TestOaiStreamHandlerMergesInputTokensDetailsCache covers a cache-only chunk
// carrying input_tokens_details.cached_tokens on a channel (Zhipu) whose cache
// mapping reads that field: the split cache must survive into the billed usage.
func TestOaiStreamHandlerMergesInputTokensDetailsCache(t *testing.T) {
	body := strings.Join([]string{
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{}}],"usage":{"prompt_tokens":3000,"completion_tokens":40,"total_tokens":3040}}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{}}],"usage":{"input_tokens_details":{"cached_tokens":1200}}}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	c, recorder, resp, info := newChatCompletionsTestContext(t, body, "text/event-stream")
	info.ChannelMeta.ChannelType = constant.ChannelTypeZhipu_v4

	usage, apiErr := OaiStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 3000, usage.PromptTokens)
	assert.Equal(t, 40, usage.CompletionTokens)
	assert.Equal(t, 3040, usage.TotalTokens)
	assert.Equal(t, 1200, usage.PromptTokensDetails.CachedTokens)
	require.NotNil(t, usage.InputTokensDetails)
	assert.Equal(t, 1200, usage.InputTokensDetails.CachedTokens)
	assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens))
	assert.Contains(t, recorder.Body.String(), "data: [DONE]")
}

// TestOaiStreamHandlerAcceptsAliasOnlyUsageInLastChunk covers input_tokens /
// output_tokens aliases on the final chunk: they must map to canonical
// prompt/completion totals instead of triggering the local estimate.
func TestOaiStreamHandlerAcceptsAliasOnlyUsageInLastChunk(t *testing.T) {
	body := strings.Join([]string{
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"input_tokens":800,"output_tokens":12,"total_tokens":812}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	c, recorder, resp, info := newChatCompletionsTestContext(t, body, "text/event-stream")

	usage, apiErr := OaiStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 800, usage.PromptTokens)
	assert.Equal(t, 12, usage.CompletionTokens)
	assert.Equal(t, 812, usage.TotalTokens)
	assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens))
	assert.Contains(t, recorder.Body.String(), "data: [DONE]")
}

// TestOaiStreamHandlerCacheOnlyChunkDoesNotSuppressEstimate pins the
// under-billing guard: a chunk carrying only cache tokens (no prompt/completion
// totals) must not be treated as billable, so the local estimate still runs
// for the totals. The pre-existing post-processing still maps the cache-only
// body's prompt_cache_hit_tokens onto the billed usage.
func TestOaiStreamHandlerCacheOnlyChunkDoesNotSuppressEstimate(t *testing.T) {
	body := strings.Join([]string{
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_cache_hit_tokens":1500}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	c, recorder, resp, info := newChatCompletionsTestContext(t, body, "text/event-stream")

	usage, apiErr := OaiStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 500, usage.PromptTokens)
	assert.True(t, common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens))
	assert.NotZero(t, usage.CompletionTokens)
	// The cache-only chunk is not billable on its own; its body cache still
	// maps onto the billed usage (pre-existing post-processing behavior).
	assert.Equal(t, 1500, usage.PromptTokensDetails.CachedTokens)
	assert.Contains(t, recorder.Body.String(), "data: [DONE]")
}

// TestOpenaiHandlerKeepsAliasUsageWithoutEstimate covers the non-stream path:
// input_tokens/output_tokens usage must be returned as real canonical totals
// instead of being overwritten by the local estimate, and the response body
// must pass through untouched.
func TestOpenaiHandlerKeepsAliasUsageWithoutEstimate(t *testing.T) {
	body := `{"id":"x","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"input_tokens":1000,"output_tokens":30,"total_tokens":1030}}`

	c, recorder, resp, info := newChatCompletionsTestContext(t, body, "application/json")

	usage, apiErr := OpenaiHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 1000, usage.PromptTokens)
	assert.Equal(t, 30, usage.CompletionTokens)
	assert.Equal(t, 1030, usage.TotalTokens)
	assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens))
	// Passthrough: the client body is the untouched upstream body.
	assert.Equal(t, body, recorder.Body.String())
}

// TestOpenaiHandlerStillEstimatesWithoutCanonicalUsage guards the fallback:
// when neither canonical nor alias totals exist, the estimate branch must still
// run and rewrite the response usage.
func TestOpenaiHandlerStillEstimatesWithoutCanonicalUsage(t *testing.T) {
	body := `{"id":"x","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"Hello world"},"finish_reason":"stop"}]}`

	c, recorder, resp, info := newChatCompletionsTestContext(t, body, "application/json")

	usage, apiErr := OpenaiHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, info.GetEstimatePromptTokens(), usage.PromptTokens)
	assert.NotZero(t, usage.CompletionTokens)
	assert.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens)
	assert.Contains(t, recorder.Body.String(), `"prompt_tokens":500`)
}
