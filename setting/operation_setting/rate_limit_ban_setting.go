package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// RateLimitBanSetting controls the dedicated per-user RPM sliding window and
// the input/output token review thresholds. It is intentionally disabled by
// default and does not perform automatic bans; exceeded limits enqueue LLM
// compliance review instead.
type RateLimitBanSetting struct {
	Enabled         bool     `json:"enabled"`
	MaxRPM          int      `json:"max_rpm"`
	MaxInputTokens  int      `json:"max_input_tokens"`  // per-request input limit, 0=off
	MaxOutputTokens int      `json:"max_output_tokens"` // per-request output limit (postflight review), 0=off
	WhitelistModels []string `json:"whitelist_models"`
}

var rateLimitBanSetting = RateLimitBanSetting{
	Enabled:         false,
	MaxRPM:          5,
	MaxInputTokens:  200000,
	MaxOutputTokens: 10000,
}

func init() {
	config.GlobalConfig.Register("rate_limit_ban_setting", &rateLimitBanSetting)
}

func GetRateLimitBanSetting() *RateLimitBanSetting {
	return &rateLimitBanSetting
}

func IsModelRateLimitWhitelisted(modelName string) bool {
	if modelName == "" {
		return false
	}
	for _, rawPattern := range rateLimitBanSetting.WhitelistModels {
		pattern := strings.TrimSpace(rawPattern)
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "*") && strings.HasPrefix(modelName, strings.TrimSuffix(pattern, "*")) {
			return true
		}
		if pattern == modelName {
			return true
		}
	}
	return false
}
