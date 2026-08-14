package requestsnapshot_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeBoundsCapacityAndRetentionValues(t *testing.T) {
	setting := GetSetting()
	previousSetting := *setting
	previousGlobalMax := constant.MaxRequestBodyMB
	t.Cleanup(func() {
		*setting = previousSetting
		constant.MaxRequestBodyMB = previousGlobalMax
	})

	constant.MaxRequestBodyMB = 128
	*setting = RequestSnapshotSetting{
		Enabled:              true,
		StoragePath:          "  ",
		MaxBodyMb:            64,
		MaxTotalMb:           1,
		RetentionDays:        1 << 30,
		CleanupIntervalHours: 1 << 30,
		OrphanGraceMinutes:   1 << 30,
	}

	Normalize()

	assert.Equal(t, DefaultStoragePath, setting.StoragePath)
	assert.Equal(t, 64, setting.MaxBodyMb)
	assert.Equal(t, 64, setting.MaxTotalMb, "capacity must allow at least one maximum-size body")
	assert.Equal(t, 3650, setting.RetentionDays)
	assert.Equal(t, 720, setting.CleanupIntervalHours)
	assert.Equal(t, 10080, setting.OrphanGraceMinutes)
}

func TestNormalizeClampsSingleBodyToGlobalLimit(t *testing.T) {
	setting := GetSetting()
	previousSetting := *setting
	previousGlobalMax := constant.MaxRequestBodyMB
	t.Cleanup(func() {
		*setting = previousSetting
		constant.MaxRequestBodyMB = previousGlobalMax
	})

	constant.MaxRequestBodyMB = 32
	*setting = RequestSnapshotSetting{
		StoragePath:          DefaultStoragePath,
		MaxBodyMb:            64,
		MaxTotalMb:           64,
		RetentionDays:        DefaultRetentionDays,
		CleanupIntervalHours: DefaultCleanupIntervalHours,
		OrphanGraceMinutes:   DefaultOrphanGraceMinutes,
	}

	Normalize()

	assert.Equal(t, 32, setting.MaxBodyMb)
	assert.Equal(t, 64, setting.MaxTotalMb)
}
