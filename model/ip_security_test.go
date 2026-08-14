package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetIPSecurityTestState(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM llm_review_tasks").Error)
	require.NoError(t, DB.Exec("DELETE FROM ip_blacklists").Error)
	require.NoError(t, DB.Exec("DELETE FROM logs").Error)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	previousEnabled := operation_setting.IsIPBlacklistEnabled()
	operation_setting.SetIPBlacklistEnabled(true)
	ipBlacklistCacheMu.Lock()
	ipBlacklistCache = nil
	ipBlacklistCacheMu.Unlock()
	ipBlacklistCacheLoaded.Store(false)
	previousRedisEnabled, previousRDB := common.RedisEnabled, common.RDB
	previousLogDB := LOG_DB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		if common.RDB != nil && common.RDB != previousRDB {
			require.NoError(t, common.RDB.Close())
		}
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
		LOG_DB = previousLogDB
		operation_setting.SetIPBlacklistEnabled(previousEnabled)
		ipBlacklistCacheMu.Lock()
		ipBlacklistCache = nil
		ipBlacklistCacheMu.Unlock()
		ipBlacklistCacheLoaded.Store(false)
	})
}

func createIPSecurityUser(t *testing.T, username string, role int, status int) User {
	t.Helper()
	user := User{
		Username:    username,
		Password:    "password",
		Role:        role,
		Status:      status,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     username + "-aff",
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func TestIPBlacklistMatchesOnlyExactIPv4(t *testing.T) {
	resetIPSecurityTestState(t)
	require.NoError(t, AddIPBlacklist(" 192.168.1.10 ", "test", 1))

	assert.True(t, IsIPBlacklisted("192.168.1.10"))
	assert.False(t, IsIPBlacklisted("192.168.1.11"))
	assert.False(t, IsIPBlacklisted("192.168.1.0/24"))
	assert.False(t, IsIPBlacklisted("2001:db8::1"))
}

func TestIPBlacklistSwitchDefaultsOff(t *testing.T) {
	resetIPSecurityTestState(t)
	require.NoError(t, AddIPBlacklist("192.0.2.40", "test", 1))
	operation_setting.SetIPBlacklistEnabled(false)

	assert.False(t, IsIPBlacklisted("192.0.2.40"))
}

func TestIPBlacklistRedisMissRefreshesDatabaseWithoutNegativePoisoning(t *testing.T) {
	resetIPSecurityTestState(t)
	server := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	require.NoError(t, AddIPBlacklist("192.0.2.44", "test", 1))

	ipBlacklistCacheMu.Lock()
	ipBlacklistCache = map[string]struct{}{}
	ipBlacklistCacheMu.Unlock()
	ipBlacklistCacheLoaded.Store(true)
	require.NoError(t, common.RDB.Del(context.Background(), ipBlacklistRedisKey("192.0.2.44")).Err())

	assert.True(t, IsIPBlacklisted("192.0.2.44"))
	value, err := common.RDB.Get(context.Background(), ipBlacklistRedisKey("192.0.2.44")).Result()
	require.NoError(t, err)
	assert.Equal(t, "1", value)

	assert.False(t, IsIPBlacklisted("192.0.2.45"))
	value, err = common.RDB.Get(context.Background(), ipBlacklistRedisKey("192.0.2.45")).Result()
	require.NoError(t, err)
	assert.Equal(t, "0", value)
}

func TestIPBlacklistDistributedRemovalOverridesStaleLocalCache(t *testing.T) {
	resetIPSecurityTestState(t)
	server := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	require.NoError(t, AddIPBlacklist("192.0.2.46", "test", 1))

	var entry IPBlacklist
	require.NoError(t, DB.Where("ip = ?", "192.0.2.46").First(&entry).Error)
	require.NoError(t, RemoveIPBlacklist(entry.Id))
	ipBlacklistCacheMu.Lock()
	ipBlacklistCache["192.0.2.46"] = struct{}{}
	ipBlacklistCacheMu.Unlock()

	assert.False(t, IsIPBlacklisted("192.0.2.46"))
	value, err := common.RDB.Get(context.Background(), ipBlacklistRedisKey("192.0.2.46")).Result()
	require.NoError(t, err)
	assert.Equal(t, "0", value)
}

func TestAddIPBlacklistDoesNotReportFailureAfterDatabaseCommit(t *testing.T) {
	resetIPSecurityTestState(t)
	LOG_DB = nil

	require.NoError(t, AddIPBlacklist("192.0.2.47", "test", 1))
	var count int64
	require.NoError(t, DB.Model(&IPBlacklist{}).Where("ip = ?", "192.0.2.47").Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestDisableUserPermanentlyCollectsOnlyHistoricalIPv4(t *testing.T) {
	resetIPSecurityTestState(t)
	user := createIPSecurityUser(t, "permanent-ip-user", common.RoleCommonUser, common.UserStatusEnabled)
	for _, ip := range []string{"192.168.1.10", " 192.168.1.10 ", "10.0.0.1", "2001:db8::1", "192.168.1.0/24"} {
		require.NoError(t, LOG_DB.Create(&Log{UserId: user.Id, Ip: ip, Type: LogTypeConsume}).Error)
	}

	require.NoError(t, DisableUserPermanently(user.Id, "203.0.113.5"))

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	assert.Equal(t, common.UserStatusDisabled, stored.Status)
	assert.Greater(t, stored.AuthVersion, int64(1))
	var audit Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", user.Id, LogTypeSystem).First(&audit).Error)
	assert.Equal(t, "Account permanently disabled by IP blacklist", audit.Content)
	assert.Equal(t, "203.0.113.5", audit.Ip)
	var entries []IPBlacklist
	require.NoError(t, DB.Order("ip").Find(&entries).Error)
	ips := make([]string, 0, len(entries))
	for _, entry := range entries {
		ips = append(ips, entry.IP)
	}
	assert.ElementsMatch(t, []string{"10.0.0.1", "192.168.1.10", "203.0.113.5"}, ips)
}

func TestDisableUserPermanentlyExemptsRoot(t *testing.T) {
	resetIPSecurityTestState(t)
	root := createIPSecurityUser(t, "root-ip-user", common.RoleRootUser, common.UserStatusEnabled)

	require.NoError(t, DisableUserPermanently(root.Id, "192.168.1.10"))

	var stored User
	require.NoError(t, DB.First(&stored, root.Id).Error)
	assert.Equal(t, common.UserStatusEnabled, stored.Status)
	var count int64
	require.NoError(t, DB.Model(&IPBlacklist{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestSearchUsersByExactIPv4UsesLogDatabase(t *testing.T) {
	resetIPSecurityTestState(t)
	first := createIPSecurityUser(t, "ip-search-one", common.RoleCommonUser, common.UserStatusEnabled)
	second := createIPSecurityUser(t, "ip-search-two", common.RoleCommonUser, common.UserStatusEnabled)
	other := createIPSecurityUser(t, "ip-search-other", common.RoleCommonUser, common.UserStatusEnabled)
	require.NoError(t, LOG_DB.Create(&Log{UserId: first.Id, Ip: "198.51.100.3"}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: second.Id, Ip: "198.51.100.3"}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: other.Id, Ip: "198.51.100.4"}).Error)

	users, total, err := SearchUsers("198.51.100.3", "", nil, nil, 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, users, 2)
	assert.Equal(t, second.Id, users[0].Id)
	assert.Equal(t, first.Id, users[1].Id)
	assert.Empty(t, users[0].Password)
}
