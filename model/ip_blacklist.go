package model

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/go-redis/redis/v8"

	"gorm.io/gorm/clause"
)

type IPBlacklist struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	IP        string `json:"ip" gorm:"type:varchar(45);uniqueIndex;not null"`
	Reason    string `json:"reason" gorm:"type:varchar(255);default:''"`
	CreatedBy int    `json:"created_by" gorm:"default:0"`
	CreatedAt int64  `json:"created_at" gorm:"default:0"`
}

var (
	ipBlacklistCache       map[string]struct{}
	ipBlacklistCacheMu     sync.RWMutex
	ipBlacklistCacheLoaded atomic.Bool
)

func ipBlacklistRedisKey(ip string) string {
	return "security:ip_blacklist:" + ip
}

func blacklistCacheTTL() time.Duration {
	ttl := common.RedisKeyCacheSeconds()
	if ttl <= 0 {
		ttl = 60
	}
	return time.Duration(ttl) * time.Second
}

func loadIPBlacklistCache() error {
	if DB == nil {
		return fmt.Errorf("main database is not initialized")
	}
	var entries []IPBlacklist
	if err := DB.Select("ip").Find(&entries).Error; err != nil {
		return err
	}
	cache := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if normalized, ok := common.NormalizeIPv4(entry.IP); ok {
			cache[normalized] = struct{}{}
		}
	}
	ipBlacklistCacheMu.Lock()
	ipBlacklistCache = cache
	ipBlacklistCacheMu.Unlock()
	ipBlacklistCacheLoaded.Store(true)
	return nil
}

func RefreshIPBlacklistCache() error {
	return loadIPBlacklistCache()
}

func setDistributedIPBlacklistEntry(ip string, blocked bool) error {
	if !common.RedisEnabled || common.RDB == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	value := "0"
	if blocked {
		value = "1"
	}
	return common.RDB.Set(ctx, ipBlacklistRedisKey(ip), value, blacklistCacheTTL()).Err()
}

func isIPBlacklistedInDatabase(ip string) (bool, error) {
	if DB == nil {
		return false, fmt.Errorf("main database is not initialized")
	}
	var count int64
	if err := DB.Model(&IPBlacklist{}).Where("ip = ?", ip).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func IsIPBlacklisted(ip string) bool {
	if !operation_setting.IsIPBlacklistEnabled() {
		return false
	}
	normalized, ok := common.NormalizeIPv4(ip)
	if !ok {
		return false
	}
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		value, err := common.RDB.Get(ctx, ipBlacklistRedisKey(normalized)).Result()
		if err == nil {
			return value == "1"
		}
		if err != nil && !errors.Is(err, redis.Nil) {
			common.SysLog("failed to read distributed IP blacklist cache: " + err.Error())
		}
		blocked, databaseErr := isIPBlacklistedInDatabase(normalized)
		if databaseErr != nil {
			common.SysLog("failed to query IP blacklist after distributed cache miss: " + databaseErr.Error())
			return false
		}
		if err := setDistributedIPBlacklistEntry(normalized, blocked); err != nil {
			common.SysLog("failed to populate distributed IP blacklist cache: " + err.Error())
		}
		return blocked
	}
	if !ipBlacklistCacheLoaded.Load() {
		if err := loadIPBlacklistCache(); err != nil {
			common.SysLog("failed to load IP blacklist cache: " + err.Error())
			return false
		}
	}
	ipBlacklistCacheMu.RLock()
	_, blocked := ipBlacklistCache[normalized]
	ipBlacklistCacheMu.RUnlock()
	return blocked
}

func GetIPBlacklist() ([]IPBlacklist, error) {
	var entries []IPBlacklist
	err := DB.Order("id DESC").Find(&entries).Error
	return entries, err
}

func AddIPBlacklist(ip string, reason string, createdBy int) error {
	normalized, ok := common.NormalizeIPv4(ip)
	if !ok {
		return fmt.Errorf("invalid exact IPv4 address: %q", ip)
	}
	entry := IPBlacklist{
		IP:        normalized,
		Reason:    reason,
		CreatedBy: createdBy,
		CreatedAt: time.Now().Unix(),
	}
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error; err != nil {
		return err
	}
	refreshErr := RefreshIPBlacklistCache()
	if err := setDistributedIPBlacklistEntry(normalized, true); err != nil {
		common.SysLog("failed to update distributed IP blacklist cache: " + err.Error())
	}
	if err := invalidateBlacklistIPUsersCache(normalized); err != nil {
		common.SysLog("failed to invalidate users after adding IP blacklist entry: " + err.Error())
	}
	return refreshErr
}

func RemoveIPBlacklist(id int) error {
	if id <= 0 {
		return fmt.Errorf("invalid IP blacklist id")
	}
	var entry IPBlacklist
	if err := DB.First(&entry, id).Error; err != nil {
		return err
	}
	if err := DB.Delete(&entry).Error; err != nil {
		return err
	}
	refreshErr := RefreshIPBlacklistCache()
	if err := setDistributedIPBlacklistEntry(entry.IP, false); err != nil {
		common.SysLog("failed to update distributed IP blacklist cache: " + err.Error())
	}
	if err := invalidateBlacklistIPUsersCache(entry.IP); err != nil {
		common.SysLog("failed to invalidate users after removing IP blacklist entry: " + err.Error())
	}
	return refreshErr
}

func invalidateBlacklistIPUsersCache(ip string) error {
	if LOG_DB == nil {
		return nil
	}
	var userIDs []int
	if err := LOG_DB.Model(&Log{}).
		Where("user_id > ? AND ip = ?", 0, ip).
		Distinct("user_id").
		Pluck("user_id", &userIDs).Error; err != nil {
		return err
	}
	for _, userID := range userIDs {
		if err := invalidateUserCache(userID); err != nil {
			return err
		}
	}
	return nil
}
