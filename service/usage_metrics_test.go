package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestApplyUsageMetricsToConsumeLogParamsUsesNormalizedUsage(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	relayInfo := &relaycommon.RelayInfo{StartTime: time.Now()}
	usage := &dto.Usage{
		PromptTokens:     70,
		CompletionTokens: 7,
		InputTokens:      120,
		UsageSemantic:    dto.BillingUsageSemanticAnthropic,
	}
	usage.PromptTokensDetails.CachedTokens = 30
	usage.PromptTokensDetails.CachedCreationTokens = 20
	usage.ClaudeCacheCreation5mTokens = 12
	usage.ClaudeCacheCreation1hTokens = 8
	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	params := model.RecordConsumeLogParams{}
	applyUsageMetricsToConsumeLogParams(&params, summary, usage)

	assert.Equal(t, 30, params.CacheReadTokens)
	assert.Equal(t, 20, params.CacheWriteTokens)
	assert.Equal(t, 12, params.CacheWriteTokens5m)
	assert.Equal(t, 8, params.CacheWriteTokens1h)
	assert.Equal(t, 120, params.InputTokensTotal)
}

func TestApplyUsageMetricsToConsumeLogParamsUsesBillingUsageAndAliasDetails(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens: 500,
		InputTokensDetails: &dto.InputTokenDetails{
			CachedTokens:     180,
			CacheWriteTokens: 25,
		},
		BillingUsage: &dto.BillingUsage{
			Source:   dto.BillingUsageSourceClaudeMessages,
			Semantic: dto.BillingUsageSemanticAnthropic,
			ClaudeUsage: &dto.ClaudeUsage{
				InputTokens:                 70,
				OutputTokens:                7,
				CacheReadInputTokens:        30,
				CacheCreationInputTokens:    20,
				ClaudeCacheCreation5mTokens: 12,
				ClaudeCacheCreation1hTokens: 8,
			},
		},
	}

	params := model.RecordConsumeLogParams{}
	ApplyUsageMetricsToConsumeLogParams(&params, usage)

	assert.Equal(t, 30, params.CacheReadTokens)
	assert.Equal(t, 20, params.CacheWriteTokens)
	assert.Equal(t, 12, params.CacheWriteTokens5m)
	assert.Equal(t, 8, params.CacheWriteTokens1h)
	assert.Equal(t, 120, params.InputTokensTotal)
}

func TestApplyUsageMetricsToConsumeLogParamsClampsNegativeMetrics(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens: -1,
		InputTokensDetails: &dto.InputTokenDetails{
			CachedTokens:     -10,
			CacheWriteTokens: -20,
		},
		ClaudeCacheCreation5mTokens: -12,
		ClaudeCacheCreation1hTokens: -8,
	}

	params := model.RecordConsumeLogParams{}
	ApplyUsageMetricsToConsumeLogParams(&params, usage)

	assert.Zero(t, params.CacheReadTokens)
	assert.Zero(t, params.CacheWriteTokens)
	assert.Zero(t, params.CacheWriteTokens5m)
	assert.Zero(t, params.CacheWriteTokens1h)
	assert.Zero(t, params.InputTokensTotal)
}

func TestApplyUsageMetricsToConsumeLogParamsClampsOversizedMetrics(t *testing.T) {
	usage := &dto.Usage{
		InputTokens: common.MaxQuota + 1,
		InputTokensDetails: &dto.InputTokenDetails{
			CachedTokens:     common.MaxQuota + 1,
			CacheWriteTokens: common.MaxQuota + 1,
		},
		ClaudeCacheCreation5mTokens: common.MaxQuota + 1,
		ClaudeCacheCreation1hTokens: common.MaxQuota + 1,
	}

	params := model.RecordConsumeLogParams{}
	ApplyUsageMetricsToConsumeLogParams(&params, usage)

	assert.Equal(t, common.MaxQuota, params.CacheReadTokens)
	assert.Equal(t, common.MaxQuota, params.CacheWriteTokens)
	assert.Equal(t, common.MaxQuota, params.CacheWriteTokens5m)
	assert.Equal(t, common.MaxQuota, params.CacheWriteTokens1h)
	assert.Equal(t, common.MaxQuota, params.InputTokensTotal)
}

func TestApplyUsageMetricsToConsumeLogParamsUsesOpenAIAliasDetails(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens: 240,
		InputTokensDetails: &dto.InputTokenDetails{
			CachedTokens:     90,
			CacheWriteTokens: 14,
		},
	}

	params := model.RecordConsumeLogParams{}
	ApplyUsageMetricsToConsumeLogParams(&params, usage)

	assert.Equal(t, 90, params.CacheReadTokens)
	assert.Equal(t, 14, params.CacheWriteTokens)
	assert.Equal(t, 240, params.InputTokensTotal)
}
