package requestsnapshot_setting

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/config"
)

// RequestSnapshotSetting holds the operational settings for the request
// snapshot feature. The values are owned by the layered config system
// (request_snapshot_setting.* option keys) and normalized on every load/update
// so runtime code never has to defensively re-clamp them.
type RequestSnapshotSetting struct {
	// Enabled turns request body capturing on. It is only operational when a
	// stable key material exists (see requestsnapshot.IsStableKeyConfigured).
	Enabled bool `json:"enabled"`
	// StoragePath is the base directory for snapshot files. Files live under a
	// per-node subdirectory derived from the node name.
	StoragePath string `json:"storage_path"`
	// MaxBodyMb caps a single captured request body. It can never exceed the
	// global request body limit.
	MaxBodyMb int `json:"max_body_mb"`
	// MaxTotalMb caps the total on-disk snapshot size for this node.
	MaxTotalMb int `json:"max_total_mb"`
	// RetentionDays keeps snapshots and their metadata at least this long.
	RetentionDays int `json:"retention_days"`
	// CleanupIntervalHours is the period of the node-local cleanup loop.
	CleanupIntervalHours int `json:"cleanup_interval_hours"`
	// OrphanGraceMinutes is how long an ownerless file may exist before the
	// node-local cleanup removes it.
	OrphanGraceMinutes int `json:"orphan_grace_minutes"`
}

const (
	DefaultStoragePath          = "./request_snapshots"
	DefaultMaxBodyMb            = 10
	DefaultMaxTotalMb           = 1024
	DefaultRetentionDays        = 30
	DefaultCleanupIntervalHours = 24
	DefaultOrphanGraceMinutes   = 60
)

// DefaultSetting returns the built-in defaults.
func DefaultSetting() RequestSnapshotSetting {
	return RequestSnapshotSetting{
		Enabled:              false,
		StoragePath:          DefaultStoragePath,
		MaxBodyMb:            DefaultMaxBodyMb,
		MaxTotalMb:           DefaultMaxTotalMb,
		RetentionDays:        DefaultRetentionDays,
		CleanupIntervalHours: DefaultCleanupIntervalHours,
		OrphanGraceMinutes:   DefaultOrphanGraceMinutes,
	}
}

var requestSnapshotSetting = DefaultSetting()

func init() {
	config.GlobalConfig.Register("request_snapshot_setting", &requestSnapshotSetting)
}

// GetSetting returns the live (normalized) setting.
func GetSetting() *RequestSnapshotSetting {
	Normalize()
	return &requestSnapshotSetting
}

// IsEnabled reports whether capturing is turned on.
func IsEnabled() bool {
	return requestSnapshotSetting.Enabled
}

// Normalize clamps every field to a safe, well-formed value. It is invoked
// after config loads and after every live option update so invalid persisted
// values never reach runtime code.
func Normalize() {
	s := &requestSnapshotSetting
	s.StoragePath = strings.TrimSpace(s.StoragePath)
	if s.StoragePath == "" {
		s.StoragePath = DefaultStoragePath
	}

	globalMaxBodyMB := constant.MaxRequestBodyMB
	if globalMaxBodyMB <= 0 {
		globalMaxBodyMB = 128
	}
	if s.MaxBodyMb <= 0 {
		s.MaxBodyMb = DefaultMaxBodyMb
	}
	if s.MaxBodyMb > globalMaxBodyMB {
		common.SysError("request_snapshot_setting.max_body_mb exceeds the global request body limit; clamping to the global limit")
		s.MaxBodyMb = globalMaxBodyMB
	}
	const maxSnapshotSizeMB = 1024 * 1024
	if s.MaxTotalMb <= 0 {
		s.MaxTotalMb = DefaultMaxTotalMb
	}
	if s.MaxTotalMb < s.MaxBodyMb {
		common.SysError("request_snapshot_setting.max_total_mb is smaller than max_body_mb; clamping to max_body_mb")
		s.MaxTotalMb = s.MaxBodyMb
	}
	if s.MaxTotalMb > maxSnapshotSizeMB {
		common.SysError(fmt.Sprintf("request_snapshot_setting.max_total_mb is too large; clamping to %d MB", maxSnapshotSizeMB))
		s.MaxTotalMb = maxSnapshotSizeMB
	}
	const maxRetentionDays = 3650
	if s.RetentionDays <= 0 {
		s.RetentionDays = DefaultRetentionDays
	}
	if s.RetentionDays > maxRetentionDays {
		common.SysError("request_snapshot_setting.retention_days is too large; clamping to 3650 days")
		s.RetentionDays = maxRetentionDays
	}
	const maxCleanupIntervalHours = 24 * 30
	if s.CleanupIntervalHours <= 0 {
		s.CleanupIntervalHours = DefaultCleanupIntervalHours
	}
	if s.CleanupIntervalHours > maxCleanupIntervalHours {
		common.SysError("request_snapshot_setting.cleanup_interval_hours is too large; clamping to 720 hours")
		s.CleanupIntervalHours = maxCleanupIntervalHours
	}
	const maxOrphanGraceMinutes = 60 * 24 * 7
	if s.OrphanGraceMinutes <= 0 {
		s.OrphanGraceMinutes = DefaultOrphanGraceMinutes
	}
	if s.OrphanGraceMinutes > maxOrphanGraceMinutes {
		common.SysError("request_snapshot_setting.orphan_grace_minutes is too large; clamping to 10080 minutes")
		s.OrphanGraceMinutes = maxOrphanGraceMinutes
	}

	// These guards are architecture-independent: even on 32-bit builds, every
	// normalized multiplier remains safe before conversion to int64 durations
	// or byte counts.
	if int64(s.MaxTotalMb) > math.MaxInt64/(1<<20) {
		s.MaxTotalMb = maxSnapshotSizeMB
	}
}
