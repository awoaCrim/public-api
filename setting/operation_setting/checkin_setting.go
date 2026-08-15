package operation_setting

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/shopspring/decimal"
)

const DefaultCheckinBalanceThreshold = 1.0

// CheckinSetting 签到功能配置
type CheckinSetting struct {
	Enabled                 bool    `json:"enabled"` // 是否启用签到功能
	MinQuota                int     `json:"min_quota"`
	MaxQuota                int     `json:"max_quota"`
	BalanceThresholdEnabled bool    `json:"balance_threshold_enabled"`
	BalanceThreshold        float64 `json:"balance_threshold"`
}

// 默认配置
var checkinSetting = CheckinSetting{
	Enabled:                 false, // 默认关闭
	MinQuota:                1000,  // 默认最小额度 1000 (约 0.002 USD)
	MaxQuota:                10000, // 默认最大额度 10000 (约 0.02 USD)
	BalanceThresholdEnabled: false,
	BalanceThreshold:        DefaultCheckinBalanceThreshold,
}

// ValidateConfigValue rejects malformed threshold values before they can mutate
// the live setting. Other check-in fields retain the generic config handling.
func (s *CheckinSetting) ValidateConfigValue(key, value string) error {
	if key != "balance_threshold" {
		return nil
	}
	if _, err := ParseCheckinBalanceThreshold(value); err != nil {
		return err
	}
	return nil
}

// ParseCheckinBalanceThreshold validates the persisted/display-unit threshold.
func ParseCheckinBalanceThreshold(value string) (float64, error) {
	threshold, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold <= 0 {
		return 0, fmt.Errorf("check-in balance threshold must be finite and greater than zero")
	}
	return threshold, nil
}

// NormalizeCheckinBalanceThreshold keeps malformed legacy configuration fail-closed.
func NormalizeCheckinBalanceThreshold(threshold float64) float64 {
	if math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold <= 0 {
		return DefaultCheckinBalanceThreshold
	}
	return threshold
}

// GetCheckinBalanceThresholdQuota converts the configured display amount into
// raw quota without rounding, so the model can enforce the inclusive boundary.
func GetCheckinBalanceThresholdQuota() decimal.Decimal {
	threshold := NormalizeCheckinBalanceThreshold(checkinSetting.BalanceThreshold)
	rate := GetUsdToCurrencyRate(USDExchangeRate)
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 {
		rate = 1
	}
	quotaPerUnit := common.QuotaPerUnit
	if math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) || quotaPerUnit <= 0 {
		quotaPerUnit = 500 * 1000.0
	}
	return decimal.NewFromFloat(threshold).
		Div(decimal.NewFromFloat(rate)).
		Mul(decimal.NewFromFloat(quotaPerUnit))
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("checkin_setting", &checkinSetting)
}

// GetCheckinSetting 获取签到配置
func GetCheckinSetting() *CheckinSetting {
	return &checkinSetting
}

// IsCheckinEnabled 是否启用签到功能
func IsCheckinEnabled() bool {
	return checkinSetting.Enabled
}

// GetCheckinQuotaRange 获取签到额度范围
func GetCheckinQuotaRange() (min, max int) {
	return checkinSetting.MinQuota, checkinSetting.MaxQuota
}
