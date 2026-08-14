package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AutomaticIPBlacklistReason = "permanent user ban"
	BlacklistedIPBanMessage    = "This account was permanently disabled because the request IP is blacklisted"
)

// IsRootUser reports whether the user holds the root role. It is used by the
// review pipeline and permanent-disable paths to keep root users exempt.
func IsRootUser(userID int) bool {
	if userID <= 0 || DB == nil {
		return false
	}
	var role int
	return DB.Model(&User{}).Where("id = ?", userID).Select("role").Scan(&role).Error == nil && role >= common.RoleRootUser
}

func IsUserPermanentlyBanned(userID int) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	var user User
	if err := DB.Select("status").First(&user, userID).Error; err != nil {
		return false, err
	}
	return user.Status == common.UserStatusDisabled, nil
}

func DisableUserPermanently(userID int, triggerIP string) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user id")
	}
	var nextAuthVersion int64
	updated := false
	if err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "role", "status", "auth_version").First(&user, userID).Error; err != nil {
			return err
		}
		if user.Role >= common.RoleRootUser {
			return nil
		}
		var err error
		nextAuthVersion, err = IncrementUserAuthVersionWithTx(tx, userID)
		if err != nil {
			return err
		}
		result := tx.Model(&User{}).Where("id = ? AND role < ?", userID, common.RoleRootUser).
			Updates(map[string]interface{}{"status": common.UserStatusDisabled, "auth_version": nextAuthVersion})
		if result.Error != nil {
			return result.Error
		}
		updated = result.RowsAffected == 1
		return nil
	}); err != nil {
		return err
	}
	if !updated {
		return nil
	}
	if err := PublishUserAuthCache(userID); err != nil {
		common.SysLog(fmt.Sprintf("failed to publish permanently banned user cache for user %d: %v", userID, err))
	}
	if err := InvalidateUserTokensCache(userID); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate token cache for permanently banned user %d: %v", userID, err))
	}
	if _, err := RevokeAllUserSessions(userID, "permanent_ban"); err != nil {
		common.SysLog(fmt.Sprintf("failed to revoke sessions for permanently banned user %d: %v", userID, err))
	}
	RecordSecurityAuditLog(userID, "Account permanently disabled by IP blacklist", triggerIP, "user.permanent_disable", map[string]interface{}{
		"trigger": "ip_blacklist",
	})
	// Best-effort review integration: a later manual re-enable must supersede
	// any in-flight review result, so the manual-disable timestamp is recorded
	// whenever the primary disable succeeded.
	if err := RecordLLMReviewManualBan(userID, common.GetTimestamp()); err != nil {
		common.SysLog(fmt.Sprintf("failed to record llm review manual ban for user %d: %v", userID, err))
	}
	_ = CancelPendingLLMReviewTasks(userID, SkipReasonManualBan)
	if err := CollectAndBlacklistUserIPv4(userID, triggerIP); err != nil {
		common.SysLog(fmt.Sprintf("failed to collect IPv4 history for permanently banned user %d: %v", userID, err))
	}
	return nil
}

func BanUserByBlacklistedIP(userID int, triggerIP string) error {
	return DisableUserPermanently(userID, triggerIP)
}

func CollectAndBlacklistUserIPv4(userID int, triggerIP string) error {
	if !operation_setting.IsIPBlacklistEnabled() {
		return nil
	}
	if LOG_DB == nil {
		return errors.New("log database is not initialized")
	}
	var history []string
	if err := LOG_DB.Model(&Log{}).
		Where("user_id = ? AND ip <> ''", userID).
		Distinct("ip").
		Pluck("ip", &history).Error; err != nil {
		return fmt.Errorf("query user IPv4 history: %w", err)
	}
	if triggerIP != "" {
		history = append(history, triggerIP)
	}
	unique := make(map[string]struct{}, len(history))
	for _, ip := range history {
		if normalized, ok := common.NormalizeIPv4(ip); ok {
			unique[normalized] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return nil
	}
	for ip := range unique {
		entry := IPBlacklist{
			IP:        ip,
			Reason:    AutomaticIPBlacklistReason,
			CreatedAt: time.Now().Unix(),
		}
		if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error; err != nil {
			return fmt.Errorf("add IPv4 %s to blacklist: %w", ip, err)
		}
	}
	if err := RefreshIPBlacklistCache(); err != nil {
		return err
	}
	for ip := range unique {
		if err := setDistributedIPBlacklistEntry(ip, true); err != nil {
			common.SysLog("failed to update distributed IP blacklist cache: " + err.Error())
		}
	}
	return nil
}
