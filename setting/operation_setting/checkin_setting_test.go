package operation_setting

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCheckinBalanceThresholdRequiresFinitePositiveValue(t *testing.T) {
	for _, value := range []string{"0", "-1", "NaN", "+Inf", "-Inf", "not-a-number"} {
		_, err := ParseCheckinBalanceThreshold(value)
		assert.Error(t, err, value)
	}

	threshold, err := ParseCheckinBalanceThreshold(" 1.25 ")
	require.NoError(t, err)
	assert.Equal(t, 1.25, threshold)
}

func TestGetCheckinBalanceThresholdQuotaUsesCurrentDisplayCurrencyAndRates(t *testing.T) {
	oldSetting := checkinSetting
	oldGeneral := generalSetting
	oldUSDExchangeRate := USDExchangeRate
	oldQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		checkinSetting = oldSetting
		generalSetting = oldGeneral
		USDExchangeRate = oldUSDExchangeRate
		common.QuotaPerUnit = oldQuotaPerUnit
	})

	checkinSetting.BalanceThreshold = 2
	common.QuotaPerUnit = 500000

	cases := []struct {
		name        string
		displayType string
		usdRate     float64
		customRate  float64
		want        string
	}{
		{name: "USD", displayType: QuotaDisplayTypeUSD, usdRate: 7, want: "1000000"},
		{name: "CNY", displayType: QuotaDisplayTypeCNY, usdRate: 10, want: "100000"},
		{name: "custom", displayType: QuotaDisplayTypeCustom, customRate: 4, want: "250000"},
		{name: "tokens fallback to USD", displayType: QuotaDisplayTypeTokens, usdRate: 10, want: "1000000"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			generalSetting.QuotaDisplayType = tc.displayType
			generalSetting.CustomCurrencyExchangeRate = tc.customRate
			USDExchangeRate = tc.usdRate
			assert.Equal(t, tc.want, GetCheckinBalanceThresholdQuota().String())
		})
	}
}

func TestGetCheckinBalanceThresholdQuotaReinterpretsSavedValueWhenRatesChange(t *testing.T) {
	oldSetting := checkinSetting
	oldGeneral := generalSetting
	oldUSDExchangeRate := USDExchangeRate
	oldQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		checkinSetting = oldSetting
		generalSetting = oldGeneral
		USDExchangeRate = oldUSDExchangeRate
		common.QuotaPerUnit = oldQuotaPerUnit
	})

	checkinSetting.BalanceThreshold = 1.25
	generalSetting.QuotaDisplayType = QuotaDisplayTypeCNY
	common.QuotaPerUnit = 500000
	USDExchangeRate = 5
	assert.Equal(t, "125000", GetCheckinBalanceThresholdQuota().String())

	USDExchangeRate = 10
	assert.Equal(t, "62500", GetCheckinBalanceThresholdQuota().String())
	assert.Equal(t, 1.25, checkinSetting.BalanceThreshold)
}

func TestGetCheckinBalanceThresholdQuotaUsesSafeFallbackForInvalidRates(t *testing.T) {
	oldSetting := checkinSetting
	oldGeneral := generalSetting
	oldUSDExchangeRate := USDExchangeRate
	oldQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		checkinSetting = oldSetting
		generalSetting = oldGeneral
		USDExchangeRate = oldUSDExchangeRate
		common.QuotaPerUnit = oldQuotaPerUnit
	})

	checkinSetting.BalanceThreshold = 2
	common.QuotaPerUnit = 500000
	cases := []struct {
		name         string
		displayType  string
		usdRate      float64
		customRate   float64
		quotaPerUnit float64
	}{
		{name: "zero CNY rate", displayType: QuotaDisplayTypeCNY, usdRate: 0, quotaPerUnit: 500000},
		{name: "NaN CNY rate", displayType: QuotaDisplayTypeCNY, usdRate: math.NaN(), quotaPerUnit: 500000},
		{name: "infinite custom rate", displayType: QuotaDisplayTypeCustom, usdRate: 7, customRate: math.Inf(1), quotaPerUnit: 500000},
		{name: "invalid quota per unit", displayType: QuotaDisplayTypeUSD, usdRate: 7, quotaPerUnit: math.NaN()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			generalSetting.QuotaDisplayType = tc.displayType
			generalSetting.CustomCurrencyExchangeRate = tc.customRate
			USDExchangeRate = tc.usdRate
			common.QuotaPerUnit = tc.quotaPerUnit
			assert.Equal(t, "1000000", GetCheckinBalanceThresholdQuota().String())
		})
	}
}

func TestCheckinConfigRejectsMalformedPersistedThresholdWithoutMutation(t *testing.T) {
	oldSetting := checkinSetting
	t.Cleanup(func() { checkinSetting = oldSetting })

	checkinSetting.BalanceThreshold = 3
	err := config.UpdateConfigFromMap(&checkinSetting, map[string]string{
		"balance_threshold": "NaN",
	})

	require.Error(t, err)
	assert.Equal(t, 3.0, checkinSetting.BalanceThreshold)
}

func TestNormalizeCheckinBalanceThresholdUsesSafeDefault(t *testing.T) {
	for _, threshold := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		assert.Equal(t, DefaultCheckinBalanceThreshold, NormalizeCheckinBalanceThreshold(threshold))
	}
	assert.Equal(t, 2.5, NormalizeCheckinBalanceThreshold(2.5))
}
