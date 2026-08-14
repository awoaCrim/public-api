package openai

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyUsagePostProcessingMapsOpenCodeGoCacheUsage(t *testing.T) {
	tests := []struct {
		name         string
		usage        dto.Usage
		responseBody []byte
		wantCached   int
	}{
		{
			name:       "usage field",
			usage:      dto.Usage{PromptCacheHitTokens: 137},
			wantCached: 137,
		},
		{
			name:         "response body field",
			responseBody: []byte(`{"usage":{"prompt_cache_hit_tokens":241}}`),
			wantCached:   241,
		},
		{
			name: "preserves standard cached tokens",
			usage: dto.Usage{
				PromptCacheHitTokens: 137,
				PromptTokensDetails: dto.InputTokenDetails{
					CachedTokens: 89,
				},
			},
			responseBody: []byte(`{"usage":{"prompt_cache_hit_tokens":241}}`),
			wantCached:   89,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usage := test.usage
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
				ChannelType: constant.ChannelTypeOpenCodeGo,
			}}

			applyUsagePostProcessing(info, &usage, test.responseBody)

			assert.Equal(t, test.wantCached, usage.PromptTokensDetails.CachedTokens)
		})
	}
}

func TestApplyUsagePostProcessingRejectsUnrelatedOpenCodeGoBodyFallbacks(t *testing.T) {
	tests := []struct {
		name         string
		usage        dto.Usage
		responseBody []byte
		wantCached   int
	}{
		{
			name:         "top-level cached tokens are ignored",
			responseBody: []byte(`{"usage":{"cached_tokens":311}}`),
		},
		{
			name:         "prompt details cached tokens are ignored",
			responseBody: []byte(`{"usage":{"prompt_tokens_details":{"cached_tokens":312}}}`),
		},
		{
			name: "standard usage value is preserved",
			usage: dto.Usage{
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 91},
			},
			responseBody: []byte(`{"usage":{"cached_tokens":311,"prompt_tokens_details":{"cached_tokens":312}}}`),
			wantCached:   91,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usage := test.usage
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
				ChannelType: constant.ChannelTypeOpenCodeGo,
			}}

			applyUsagePostProcessing(info, &usage, test.responseBody)

			assert.Equal(t, test.wantCached, usage.PromptTokensDetails.CachedTokens)
		})
	}
}

func TestApplyUsagePostProcessingKeepsDeepSeekCachedTokensFallback(t *testing.T) {
	usage := dto.Usage{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: constant.ChannelTypeDeepSeek,
	}}

	applyUsagePostProcessing(info, &usage, []byte(`{"usage":{"cached_tokens":313}}`))

	assert.Equal(t, 313, usage.PromptTokensDetails.CachedTokens)
}

func TestApplyUsagePostProcessingRejectsInvalidCacheFallbacks(t *testing.T) {
	oversized := common.MaxQuota + 1
	tests := []struct {
		name         string
		channelType  int
		usage        dto.Usage
		responseBody []byte
	}{
		{
			name:        "deepseek negative usage alias",
			channelType: constant.ChannelTypeDeepSeek,
			usage:       dto.Usage{PromptCacheHitTokens: -1},
		},
		{
			name:         "deepseek oversized body fallback",
			channelType:  constant.ChannelTypeDeepSeek,
			responseBody: []byte(fmt.Sprintf(`{"usage":{"cached_tokens":%d}}`, oversized)),
		},
		{
			name:        "opencode go oversized usage alias",
			channelType: constant.ChannelTypeOpenCodeGo,
			usage:       dto.Usage{PromptCacheHitTokens: oversized},
		},
		{
			name:         "opencode go negative body fallback",
			channelType:  constant.ChannelTypeOpenCodeGo,
			responseBody: []byte(`{"usage":{"prompt_cache_hit_tokens":-1}}`),
		},
		{
			name:        "zhipu oversized input details fallback",
			channelType: constant.ChannelTypeZhipu_v4,
			usage:       dto.Usage{InputTokensDetails: &dto.InputTokenDetails{CachedTokens: oversized}},
		},
		{
			name:         "zhipu negative body fallback",
			channelType:  constant.ChannelTypeZhipu_v4,
			responseBody: []byte(`{"usage":{"prompt_tokens_details":{"cached_tokens":-1}}}`),
		},
		{
			name:         "moonshot oversized choice fallback",
			channelType:  constant.ChannelTypeMoonshot,
			responseBody: []byte(fmt.Sprintf(`{"choices":[{"usage":{"cached_tokens":%d}}]}`, oversized)),
		},
		{
			name:         "moonshot negative generic body fallback",
			channelType:  constant.ChannelTypeMoonshot,
			responseBody: []byte(`{"usage":{"cached_tokens":-1}}`),
		},
		{
			name:         "openai negative llama fallback",
			channelType:  constant.ChannelTypeOpenAI,
			responseBody: []byte(`{"timings":{"cache_n":-1}}`),
		},
		{
			name:         "openai oversized llama fallback",
			channelType:  constant.ChannelTypeOpenAI,
			responseBody: []byte(fmt.Sprintf(`{"timings":{"cache_n":%d}}`, oversized)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usage := test.usage
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: test.channelType}}

			applyUsagePostProcessing(info, &usage, test.responseBody)

			assert.Zero(t, usage.PromptTokensDetails.CachedTokens)
		})
	}
}

func TestApplyUsagePostProcessingSkipsInvalidHigherPriorityFallbacks(t *testing.T) {
	oversized := common.MaxQuota + 1
	tests := []struct {
		name         string
		channelType  int
		usage        dto.Usage
		responseBody []byte
		wantCached   int
	}{
		{
			name:         "deepseek negative alias falls through to body",
			channelType:  constant.ChannelTypeDeepSeek,
			usage:        dto.Usage{PromptCacheHitTokens: -1},
			responseBody: []byte(`{"usage":{"cached_tokens":44}}`),
			wantCached:   44,
		},
		{
			name:         "opencode go oversized alias falls through to body",
			channelType:  constant.ChannelTypeOpenCodeGo,
			usage:        dto.Usage{PromptCacheHitTokens: oversized},
			responseBody: []byte(`{"usage":{"prompt_cache_hit_tokens":45}}`),
			wantCached:   45,
		},
		{
			name:         "zhipu oversized input details fall through to body",
			channelType:  constant.ChannelTypeZhipu_v4,
			usage:        dto.Usage{InputTokensDetails: &dto.InputTokenDetails{CachedTokens: oversized}},
			responseBody: []byte(`{"usage":{"cached_tokens":46}}`),
			wantCached:   46,
		},
		{
			name:         "moonshot oversized input details fall through to choice",
			channelType:  constant.ChannelTypeMoonshot,
			usage:        dto.Usage{InputTokensDetails: &dto.InputTokenDetails{CachedTokens: oversized}},
			responseBody: []byte(`{"choices":[{"usage":{"cached_tokens":47}}]}`),
			wantCached:   47,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usage := test.usage
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: test.channelType}}

			applyUsagePostProcessing(info, &usage, test.responseBody)

			assert.Equal(t, test.wantCached, usage.PromptTokensDetails.CachedTokens)
		})
	}
}

func TestApplyUsagePostProcessingKeepsValidProviderFallbacks(t *testing.T) {
	tests := []struct {
		name         string
		channelType  int
		usage        dto.Usage
		responseBody []byte
		wantCached   int
	}{
		{
			name:        "zhipu input details",
			channelType: constant.ChannelTypeZhipu_v4,
			usage:       dto.Usage{InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 71}},
			wantCached:  71,
		},
		{
			name:         "moonshot choice usage",
			channelType:  constant.ChannelTypeMoonshot,
			responseBody: []byte(`{"choices":[{"usage":{"cached_tokens":72}}]}`),
			wantCached:   72,
		},
		{
			name:         "openai llama timings",
			channelType:  constant.ChannelTypeOpenAI,
			responseBody: []byte(`{"timings":{"cache_n":73}}`),
			wantCached:   73,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usage := test.usage
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: test.channelType}}

			applyUsagePostProcessing(info, &usage, test.responseBody)

			assert.Equal(t, test.wantCached, usage.PromptTokensDetails.CachedTokens)
		})
	}
}

func TestApplyUsagePostProcessingPreservesCanonicalCachedTokens(t *testing.T) {
	usage := dto.Usage{
		PromptCacheHitTokens: common.MaxQuota + 1,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 89,
		},
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeDeepSeek}}

	applyUsagePostProcessing(info, &usage, []byte(`{"usage":{"cached_tokens":-1}}`))

	assert.Equal(t, 89, usage.PromptTokensDetails.CachedTokens)
}

func TestNormalizeOpenAICompatibleUsageFillsAliasesAndTotal(t *testing.T) {
	usage := &dto.Usage{InputTokens: 1000, OutputTokens: 50}

	normalizeOpenAICompatibleUsage(usage)

	assert.Equal(t, 1000, usage.PromptTokens)
	assert.Equal(t, 50, usage.CompletionTokens)
	assert.Equal(t, 1050, usage.TotalTokens)
}

func TestNormalizeOpenAICompatibleUsagePrefersStandardValues(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     500,
		CompletionTokens: 20,
		TotalTokens:      520,
		InputTokens:      1000,
		OutputTokens:     99,
	}

	normalizeOpenAICompatibleUsage(usage)

	assert.Equal(t, 500, usage.PromptTokens)
	assert.Equal(t, 20, usage.CompletionTokens)
	assert.Equal(t, 520, usage.TotalTokens)
}

func TestNormalizeOpenAICompatibleUsageKeepsProvidedTotal(t *testing.T) {
	usage := &dto.Usage{InputTokens: 1000, OutputTokens: 50, TotalTokens: 9999}

	normalizeOpenAICompatibleUsage(usage)

	assert.Equal(t, 1000, usage.PromptTokens)
	assert.Equal(t, 50, usage.CompletionTokens)
	assert.Equal(t, 9999, usage.TotalTokens)
}

func TestNormalizeOpenAICompatibleUsageIgnoresNegativeAliases(t *testing.T) {
	usage := &dto.Usage{InputTokens: -10, OutputTokens: -3}

	normalizeOpenAICompatibleUsage(usage)

	assert.Zero(t, usage.PromptTokens)
	assert.Zero(t, usage.CompletionTokens)
	assert.Zero(t, usage.TotalTokens)
}

func TestNormalizeOpenAICompatibleUsageTotalOnlyDoesNotInventSplit(t *testing.T) {
	usage := &dto.Usage{TotalTokens: 1000}

	normalizeOpenAICompatibleUsage(usage)

	assert.Zero(t, usage.PromptTokens)
	assert.Zero(t, usage.CompletionTokens)
}

func TestMergeUsageDetailsFillsCacheFromOtherChunk(t *testing.T) {
	dst := &dto.Usage{PromptTokens: 1000, CompletionTokens: 10, TotalTokens: 1010}
	src := &dto.Usage{PromptCacheHitTokens: 800}

	mergeUsageDetails(dst, src)

	assert.Equal(t, 800, dst.PromptCacheHitTokens)
	assert.Equal(t, 1000, dst.PromptTokens)
	assert.Equal(t, 10, dst.CompletionTokens)
	assert.Equal(t, 1010, dst.TotalTokens)
}

func TestMergeUsageDetailsDoesNotOverwriteNonZeroValues(t *testing.T) {
	dst := &dto.Usage{
		PromptTokens:         2000,
		CompletionTokens:     20,
		TotalTokens:          2020,
		PromptCacheHitTokens: 100,
	}
	src := &dto.Usage{
		PromptTokens:         3000,
		CompletionTokens:     30,
		TotalTokens:          3030,
		PromptCacheHitTokens: 900,
		InputTokens:          7,
		OutputTokens:         8,
	}

	mergeUsageDetails(dst, src)

	assert.Equal(t, 2000, dst.PromptTokens)
	assert.Equal(t, 20, dst.CompletionTokens)
	assert.Equal(t, 2020, dst.TotalTokens)
	assert.Equal(t, 100, dst.PromptCacheHitTokens)
	assert.Equal(t, 7, dst.InputTokens)
	assert.Equal(t, 8, dst.OutputTokens)
}

func TestMergeUsageDetailsMergesPromptAndInputDetails(t *testing.T) {
	dst := &dto.Usage{PromptTokens: 1000}
	src := &dto.Usage{
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         900,
			CachedCreationTokens: 10,
			CacheWriteTokens:     20,
			TextTokens:           800,
			AudioTokens:          100,
			ImageTokens:          100,
		},
		InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 850},
	}

	mergeUsageDetails(dst, src)

	assert.Equal(t, 900, dst.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 10, dst.PromptTokensDetails.CachedCreationTokens)
	assert.Equal(t, 20, dst.PromptTokensDetails.CacheWriteTokens)
	assert.Equal(t, 800, dst.PromptTokensDetails.TextTokens)
	assert.Equal(t, 100, dst.PromptTokensDetails.AudioTokens)
	assert.Equal(t, 100, dst.PromptTokensDetails.ImageTokens)
	require.NotNil(t, dst.InputTokensDetails)
	assert.Equal(t, 850, dst.InputTokensDetails.CachedTokens)
}

func TestMergeUsageDetailsIgnoresNegativeSources(t *testing.T) {
	dst := &dto.Usage{}
	src := &dto.Usage{
		PromptTokens:         -5,
		CompletionTokens:     -3,
		TotalTokens:          -8,
		PromptCacheHitTokens: -10,
	}

	mergeUsageDetails(dst, src)

	assert.Zero(t, dst.PromptTokens)
	assert.Zero(t, dst.CompletionTokens)
	assert.Zero(t, dst.TotalTokens)
	assert.Zero(t, dst.PromptCacheHitTokens)
}

func TestCloneUsageDeepCopiesDetailPointers(t *testing.T) {
	original := &dto.Usage{
		PromptTokens:       1000,
		InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 900, TextTokens: 100},
	}

	cloned := cloneUsage(original)
	require.NotNil(t, cloned)
	cloned.PromptTokens = 1
	cloned.InputTokensDetails.CachedTokens = 1

	assert.Equal(t, 1000, original.PromptTokens)
	assert.Equal(t, 900, original.InputTokensDetails.CachedTokens)
}

func TestExtractStreamUsageReturnsNilWithoutUsage(t *testing.T) {
	assert.Nil(t, extractStreamUsage(""))
	assert.Nil(t, extractStreamUsage("[DONE]"))
	assert.Nil(t, extractStreamUsage(`{"choices":[{"delta":{"content":"hi"}}]}`))
	assert.Nil(t, extractStreamUsage(`not json`))
}

func TestExtractStreamUsageNormalizesAliasesAndCopies(t *testing.T) {
	data := `{"id":"x","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"input_tokens":1000,"output_tokens":20,"total_tokens":1020,"prompt_cache_hit_tokens":900}}`

	usage := extractStreamUsage(data)

	require.NotNil(t, usage)
	assert.Equal(t, 1000, usage.PromptTokens)
	assert.Equal(t, 20, usage.CompletionTokens)
	assert.Equal(t, 1020, usage.TotalTokens)
	assert.Equal(t, 900, usage.PromptCacheHitTokens)
}

// TestNormalizeDoesNotPreemptChannelCacheMappingPriority pins that normalize
// only maps token aliases and never fills PromptTokensDetails.CachedTokens,
// otherwise Zhipu's InputTokensDetails priority would be inverted.
func TestNormalizeDoesNotPreemptChannelCacheMappingPriority(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:         100,
		CompletionTokens:     10,
		PromptCacheHitTokens: 200,
		InputTokensDetails:   &dto.InputTokenDetails{CachedTokens: 150},
	}

	normalizeOpenAICompatibleUsage(usage)

	assert.Zero(t, usage.PromptTokensDetails.CachedTokens)

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: constant.ChannelTypeZhipu_v4,
	}}
	applyUsagePostProcessing(info, usage, nil)
	assert.Equal(t, 150, usage.PromptTokensDetails.CachedTokens)
}

// TestNormalizeAndSettleClampCacheBeyondPrompt pins the billing-safety chain:
// cache hits that exceed the prompt total must never produce a negative base
// prompt token count in tiered settlement.
func TestNormalizeAndSettleClampCacheBeyondPrompt(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:         100,
		CompletionTokens:     10,
		TotalTokens:          110,
		PromptCacheHitTokens: 300,
	}

	normalizeOpenAICompatibleUsage(usage)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: constant.ChannelTypeOpenCodeGo,
	}}
	applyUsagePostProcessing(info, usage, nil)
	assert.Equal(t, 300, usage.PromptTokensDetails.CachedTokens)

	params := service.BuildTieredTokenParams(usage, false, map[string]bool{"cr": true})
	assert.Equal(t, float64(0), params.P)
	assert.Equal(t, float64(300), params.CR)
}
