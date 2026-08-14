package openai

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// normalizeOpenAICompatibleUsage maps the alternate input_tokens/output_tokens
// field names used by some OpenAI-compatible upstreams onto the standard
// prompt_tokens/completion_tokens fields. Fill-if-zero: a non-zero standard
// value always wins, and negative alias values are never accepted as a fill
// source. When total_tokens is absent it is recomputed from the filled pair.
// A total-only usage is left untouched: without prompt/completion values the
// input/output split is unknown.
func normalizeOpenAICompatibleUsage(usage *dto.Usage) {
	if usage == nil {
		return
	}
	if usage.PromptTokens == 0 && usage.InputTokens > 0 {
		usage.PromptTokens = usage.InputTokens
	}
	if usage.CompletionTokens == 0 && usage.OutputTokens > 0 {
		usage.CompletionTokens = usage.OutputTokens
	}
	if usage.TotalTokens == 0 && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
}

// cloneUsage returns a deep copy of usage so chunk-derived values never alias
// a shared InputTokensDetails (or BillingUsage) pointer across chunks.
func cloneUsage(u *dto.Usage) *dto.Usage {
	if u == nil {
		return nil
	}
	c := *u
	if u.InputTokensDetails != nil {
		d := *u.InputTokensDetails
		c.InputTokensDetails = &d
	}
	if u.BillingUsage != nil {
		b := *u.BillingUsage
		c.BillingUsage = &b
	}
	return &c
}

// mergeInputTokenDetails fills any zero fields of dst from positive src values.
// Non-zero dst values and negative src values are never copied.
func mergeInputTokenDetails(dst *dto.InputTokenDetails, src dto.InputTokenDetails) {
	if dst.CachedTokens == 0 && src.CachedTokens > 0 {
		dst.CachedTokens = src.CachedTokens
	}
	if dst.CachedCreationTokens == 0 && src.CachedCreationTokens > 0 {
		dst.CachedCreationTokens = src.CachedCreationTokens
	}
	if dst.CacheWriteTokens == 0 && src.CacheWriteTokens > 0 {
		dst.CacheWriteTokens = src.CacheWriteTokens
	}
	if dst.TextTokens == 0 && src.TextTokens > 0 {
		dst.TextTokens = src.TextTokens
	}
	if dst.AudioTokens == 0 && src.AudioTokens > 0 {
		dst.AudioTokens = src.AudioTokens
	}
	if dst.ImageTokens == 0 && src.ImageTokens > 0 {
		dst.ImageTokens = src.ImageTokens
	}
}

// mergeUsageDetails copies any missing (zero) token, cache and detail fields
// from src into dst. Non-zero values in dst are never overwritten, so the
// standard values from the last valid usage chunk stay authoritative, and only
// positive src values are accepted as fill sources.
func mergeUsageDetails(dst, src *dto.Usage) {
	if dst == nil || src == nil {
		return
	}
	if dst.PromptTokens == 0 && src.PromptTokens > 0 {
		dst.PromptTokens = src.PromptTokens
	}
	if dst.CompletionTokens == 0 && src.CompletionTokens > 0 {
		dst.CompletionTokens = src.CompletionTokens
	}
	if dst.TotalTokens == 0 && src.TotalTokens > 0 {
		dst.TotalTokens = src.TotalTokens
	}
	if dst.InputTokens == 0 && src.InputTokens > 0 {
		dst.InputTokens = src.InputTokens
	}
	if dst.OutputTokens == 0 && src.OutputTokens > 0 {
		dst.OutputTokens = src.OutputTokens
	}
	if dst.PromptCacheHitTokens == 0 && src.PromptCacheHitTokens > 0 {
		dst.PromptCacheHitTokens = src.PromptCacheHitTokens
	}
	mergeInputTokenDetails(&dst.PromptTokensDetails, src.PromptTokensDetails)
	if src.InputTokensDetails != nil {
		if dst.InputTokensDetails == nil {
			d := *src.InputTokensDetails
			dst.InputTokensDetails = &d
		} else {
			mergeInputTokenDetails(dst.InputTokensDetails, *src.InputTokensDetails)
		}
	}
}

// extractStreamUsage returns a deep-copied, normalized usage object from an
// OpenAI chat-completions SSE chunk, or nil when the chunk carries no usage.
// The copy keeps per-chunk values independent so a later merge cannot mutate a
// previously captured chunk through a shared pointer.
func extractStreamUsage(data string) *dto.Usage {
	if data == "" || data == "[DONE]" {
		return nil
	}
	var chunk dto.ChatCompletionsStreamResponse
	if err := common.Unmarshal(common.StringToByteSlice(data), &chunk); err != nil {
		return nil
	}
	if chunk.Usage == nil {
		return nil
	}
	u := cloneUsage(chunk.Usage)
	normalizeOpenAICompatibleUsage(u)
	return u
}

func isValidCachedTokenFallback(value int) bool {
	return value > 0 && value <= common.MaxQuota
}

func applyUsagePostProcessing(info *relaycommon.RelayInfo, usage *dto.Usage, responseBody []byte) {
	if info == nil || usage == nil {
		return
	}

	switch info.ChannelType {
	case constant.ChannelTypeDeepSeek:
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if isValidCachedTokenFallback(usage.PromptCacheHitTokens) {
				usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
			} else if cachedTokens, ok := extractCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			}
		}
	case constant.ChannelTypeOpenCodeGo:
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if isValidCachedTokenFallback(usage.PromptCacheHitTokens) {
				usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
			} else if cachedTokens, ok := extractOpenCodeGoCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			}
		}
	case constant.ChannelTypeZhipu_v4:
		// 智普的cached_tokens在标准位置: usage.prompt_tokens_details.cached_tokens
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if usage.InputTokensDetails != nil && isValidCachedTokenFallback(usage.InputTokensDetails.CachedTokens) {
				usage.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
			} else if cachedTokens, ok := extractCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			} else if isValidCachedTokenFallback(usage.PromptCacheHitTokens) {
				usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
			}
		}
	case constant.ChannelTypeMoonshot:
		// Moonshot的cached_tokens在非标准位置: choices[].usage.cached_tokens
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if usage.InputTokensDetails != nil && isValidCachedTokenFallback(usage.InputTokensDetails.CachedTokens) {
				usage.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
			} else if cachedTokens, ok := extractMoonshotCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			} else if cachedTokens, ok := extractCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			} else if isValidCachedTokenFallback(usage.PromptCacheHitTokens) {
				usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
			}
		}
	case constant.ChannelTypeOpenAI:
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if cachedTokens, ok := extractLlamaCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			}
		}
	}
}

func extractCachedTokensFromBody(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	var payload struct {
		Usage struct {
			PromptTokensDetails struct {
				CachedTokens *int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CachedTokens         *int `json:"cached_tokens"`
			PromptCacheHitTokens *int `json:"prompt_cache_hit_tokens"`
		} `json:"usage"`
	}

	if err := common.Unmarshal(body, &payload); err != nil {
		return 0, false
	}

	if payload.Usage.PromptTokensDetails.CachedTokens != nil && isValidCachedTokenFallback(*payload.Usage.PromptTokensDetails.CachedTokens) {
		return *payload.Usage.PromptTokensDetails.CachedTokens, true
	}
	if payload.Usage.CachedTokens != nil && isValidCachedTokenFallback(*payload.Usage.CachedTokens) {
		return *payload.Usage.CachedTokens, true
	}
	if payload.Usage.PromptCacheHitTokens != nil && isValidCachedTokenFallback(*payload.Usage.PromptCacheHitTokens) {
		return *payload.Usage.PromptCacheHitTokens, true
	}
	return 0, false
}

func extractOpenCodeGoCachedTokensFromBody(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	var payload struct {
		Usage struct {
			PromptCacheHitTokens *int `json:"prompt_cache_hit_tokens"`
		} `json:"usage"`
	}

	if err := common.Unmarshal(body, &payload); err != nil {
		return 0, false
	}
	if payload.Usage.PromptCacheHitTokens == nil || !isValidCachedTokenFallback(*payload.Usage.PromptCacheHitTokens) {
		return 0, false
	}
	return *payload.Usage.PromptCacheHitTokens, true
}

// extractMoonshotCachedTokensFromBody 从Moonshot的非标准位置提取cached_tokens
// Moonshot的流式响应格式: {"choices":[{"usage":{"cached_tokens":111}}]}
func extractMoonshotCachedTokensFromBody(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	var payload struct {
		Choices []struct {
			Usage struct {
				CachedTokens *int `json:"cached_tokens"`
			} `json:"usage"`
		} `json:"choices"`
	}

	if err := common.Unmarshal(body, &payload); err != nil {
		return 0, false
	}

	// 遍历choices查找cached_tokens
	for _, choice := range payload.Choices {
		if choice.Usage.CachedTokens != nil && isValidCachedTokenFallback(*choice.Usage.CachedTokens) {
			return *choice.Usage.CachedTokens, true
		}
	}

	return 0, false
}

// extractLlamaCachedTokensFromBody 从llama.cpp的非标准位置提取cache_n
func extractLlamaCachedTokensFromBody(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	var payload struct {
		Timings struct {
			CachedTokens *int `json:"cache_n"`
		} `json:"timings"`
	}

	if err := common.Unmarshal(body, &payload); err != nil {
		return 0, false
	}

	if payload.Timings.CachedTokens == nil || !isValidCachedTokenFallback(*payload.Timings.CachedTokens) {
		return 0, false
	}
	return *payload.Timings.CachedTokens, true
}
