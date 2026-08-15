package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func prepareCheckinTest(t *testing.T, userID int, quota int) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Checkin{}))
	require.NoError(t, DB.Create(&User{
		Id:       userID,
		Username: fmt.Sprintf("checkin-test-user-%d", userID),
		Password: "password123",
		Quota:    quota,
		AffCode:  fmt.Sprintf("checkin-aff-%d", userID),
	}).Error)
	t.Cleanup(func() {
		DB.Where("user_id = ?", userID).Delete(&Checkin{})
		DB.Delete(&User{}, userID)
	})
}

func TestInvalidCheckinThresholdOptionDoesNotPublishToOptionMap(t *testing.T) {
	oldSetting := *operation_setting.GetCheckinSetting()
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		"checkin_setting.balance_threshold": "1",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		*operation_setting.GetCheckinSetting() = oldSetting
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	err := updateOptionMap("checkin_setting.balance_threshold", "NaN")

	require.Error(t, err)
	common.OptionMapRWMutex.RLock()
	publishedValue := common.OptionMap["checkin_setting.balance_threshold"]
	common.OptionMapRWMutex.RUnlock()
	assert.Equal(t, "1", publishedValue)
}

func TestUpdateOptionRejectsInvalidCheckinThresholdBeforePersistence(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))
	t.Cleanup(func() {
		DB.Where("key = ?", "checkin_setting.balance_threshold").Delete(&Option{})
	})

	err := UpdateOption("checkin_setting.balance_threshold", "Infinity")

	require.Error(t, err)
	var option Option
	queryErr := DB.Where("key = ?", "checkin_setting.balance_threshold").First(&option).Error
	assert.ErrorIs(t, queryErr, gorm.ErrRecordNotFound)
}

func TestUserCheckinRejectsBalanceAtThresholdWithoutSideEffects(t *testing.T) {
	oldSetting := *operation_setting.GetCheckinSetting()
	oldQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		*operation_setting.GetCheckinSetting() = oldSetting
		common.QuotaPerUnit = oldQuotaPerUnit
	})

	const userID = 98101
	prepareCheckinTest(t, userID, 500000)
	common.QuotaPerUnit = 500000
	setting := operation_setting.GetCheckinSetting()
	setting.Enabled = true
	setting.BalanceThresholdEnabled = true
	setting.BalanceThreshold = 1

	checkin, err := UserCheckin(userID)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCheckinBalanceThreshold)
	assert.Nil(t, checkin)
	var records []Checkin
	require.NoError(t, DB.Where("user_id = ?", userID).Find(&records).Error)
	assert.Empty(t, records)
	quota, err := GetUserQuota(userID, true)
	require.NoError(t, err)
	assert.Equal(t, 500000, quota)
}

func TestUserCheckinSkipsBalanceThresholdWhenDisabled(t *testing.T) {
	oldSetting := *operation_setting.GetCheckinSetting()
	oldQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		*operation_setting.GetCheckinSetting() = oldSetting
		common.QuotaPerUnit = oldQuotaPerUnit
	})

	const userID = 98104
	prepareCheckinTest(t, userID, 500001)
	common.QuotaPerUnit = 500000
	setting := operation_setting.GetCheckinSetting()
	setting.Enabled = true
	setting.MinQuota = 10
	setting.MaxQuota = 10
	setting.BalanceThresholdEnabled = false
	setting.BalanceThreshold = 1

	checkin, err := UserCheckin(userID)

	require.NoError(t, err)
	require.NotNil(t, checkin)
	assert.Equal(t, 10, checkin.QuotaAwarded)
}

func TestUserCheckinFailsClosedWhenUserBalanceRowIsMissing(t *testing.T) {
	oldSetting := *operation_setting.GetCheckinSetting()
	t.Cleanup(func() { *operation_setting.GetCheckinSetting() = oldSetting })

	require.NoError(t, DB.AutoMigrate(&Checkin{}))
	setting := operation_setting.GetCheckinSetting()
	setting.Enabled = true
	setting.BalanceThresholdEnabled = true
	setting.BalanceThreshold = 1

	checkin, err := UserCheckin(98105)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCheckinBalanceRead)
	assert.Nil(t, checkin)
	var records []Checkin
	require.NoError(t, DB.Where("user_id = ?", 98105).Find(&records).Error)
	assert.Empty(t, records)
}

func TestUserCheckinRejectsFractionalBalanceAtThresholdWithoutSideEffects(t *testing.T) {
	oldSetting := *operation_setting.GetCheckinSetting()
	oldQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		*operation_setting.GetCheckinSetting() = oldSetting
		common.QuotaPerUnit = oldQuotaPerUnit
	})

	const userID = 98106
	prepareCheckinTest(t, userID, 250000)
	common.QuotaPerUnit = 500000
	setting := operation_setting.GetCheckinSetting()
	setting.Enabled = true
	setting.BalanceThresholdEnabled = true
	setting.BalanceThreshold = 0.5

	checkin, err := UserCheckin(userID)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCheckinBalanceThreshold)
	assert.Nil(t, checkin)
	var records []Checkin
	require.NoError(t, DB.Where("user_id = ?", userID).Find(&records).Error)
	assert.Empty(t, records)
}

func TestUserCheckinRejectsBalanceAboveThresholdWithoutSideEffects(t *testing.T) {
	oldSetting := *operation_setting.GetCheckinSetting()
	oldQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		*operation_setting.GetCheckinSetting() = oldSetting
		common.QuotaPerUnit = oldQuotaPerUnit
	})

	const userID = 98107
	prepareCheckinTest(t, userID, 500001)
	common.QuotaPerUnit = 500000
	setting := operation_setting.GetCheckinSetting()
	setting.Enabled = true
	setting.BalanceThresholdEnabled = true
	setting.BalanceThreshold = 1

	checkin, err := UserCheckin(userID)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCheckinBalanceThreshold)
	assert.Nil(t, checkin)
	var records []Checkin
	require.NoError(t, DB.Where("user_id = ?", userID).Find(&records).Error)
	assert.Empty(t, records)
}

func TestUserCheckinAllowsBalanceBelowThresholdAndPreservesDailyUniqueness(t *testing.T) {
	oldSetting := *operation_setting.GetCheckinSetting()
	oldQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		*operation_setting.GetCheckinSetting() = oldSetting
		common.QuotaPerUnit = oldQuotaPerUnit
	})

	const userID = 98102
	prepareCheckinTest(t, userID, 1)
	setting := operation_setting.GetCheckinSetting()
	setting.Enabled = true
	setting.MinQuota = 10
	setting.MaxQuota = 10
	setting.BalanceThresholdEnabled = true
	setting.BalanceThreshold = 1
	common.QuotaPerUnit = 500000

	checkin, err := UserCheckin(userID)
	require.NoError(t, err)
	require.NotNil(t, checkin)
	quota, err := GetUserQuota(userID, true)
	require.NoError(t, err)
	assert.Equal(t, 11, quota)

	second, err := UserCheckin(userID)
	require.Error(t, err)
	assert.Nil(t, second)
	assert.EqualError(t, err, "今日已签到")
	quota, err = GetUserQuota(userID, true)
	require.NoError(t, err)
	assert.Equal(t, 11, quota)
}

func TestUserCheckinFailsClosedWhenAuthoritativeBalanceReadFails(t *testing.T) {
	oldSetting := *operation_setting.GetCheckinSetting()
	oldDB := DB
	t.Cleanup(func() {
		*operation_setting.GetCheckinSetting() = oldSetting
		DB = oldDB
	})

	badDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, badDB.AutoMigrate(&Checkin{}))
	DB = badDB
	setting := operation_setting.GetCheckinSetting()
	setting.Enabled = true
	setting.BalanceThresholdEnabled = true
	setting.BalanceThreshold = 1

	checkin, err := UserCheckin(98103)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCheckinBalanceRead))
	assert.Nil(t, checkin)
}
