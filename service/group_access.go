package service

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

const (
	UserGroupGrantSourceManual        = "manual"
	UserGroupGrantSourceSystem        = "system"
	UserGroupGrantSourceRoutingCompat = "routing-compat-v1"
)

// UserGroupAccess is the effective original-group permission set for a user.
// Inherited groups come from the account-tier usable-group rules; granted
// groups come from user_group_grants and are additive.
type UserGroupAccess struct {
	Groups    map[string]string
	Inherited map[string]bool
	Granted   map[string]bool
}

// GetLegacyGroupCatalog returns every group key that may carry permissions:
// the usable-group catalog plus every configured group ratio key.
func GetLegacyGroupCatalog() map[string]string {
	catalog := setting.GetUserUsableGroupsCopy()
	for key := range ratio_setting.GetGroupRatioCopy() {
		if key != "" && key != "auto" {
			if _, ok := catalog[key]; !ok {
				catalog[key] = key
			}
		}
	}
	return catalog
}

// ResolveUserGroupAccess resolves a user's effective original-group
// permissions against db. Callers in a transaction can pass the post-update
// account tier without consulting the process-wide model.DB handle.
func ResolveUserGroupAccess(db *gorm.DB, userID int, userGroup string) (*UserGroupAccess, error) {
	if userID <= 0 {
		return nil, errors.New("user id is required")
	}
	if db == nil {
		db = model.DB
	}
	access := &UserGroupAccess{
		Groups:    make(map[string]string),
		Inherited: make(map[string]bool),
		Granted:   make(map[string]bool),
	}
	for key, desc := range GetUserUsableGroups(userGroup) {
		key = strings.TrimSpace(key)
		if key == "" || key == "auto" {
			continue
		}
		access.Groups[key] = desc
		access.Inherited[key] = true
	}

	if db == nil || !db.Migrator().HasTable(&model.UserGroupGrant{}) {
		return access, nil
	}
	var grants []model.UserGroupGrant
	if err := db.Where("user_id = ? AND (expires_at = 0 OR expires_at > ?)", userID, time.Now().Unix()).
		Order("sort_order ASC, id ASC").Find(&grants).Error; err != nil {
		return nil, err
	}
	catalog := GetLegacyGroupCatalog()
	for _, grant := range grants {
		key := strings.TrimSpace(grant.GroupKey)
		if key == "" || key == "auto" {
			continue
		}
		if _, ok := catalog[key]; !ok {
			continue
		}
		if _, inherited := access.Inherited[key]; inherited {
			continue
		}
		access.Groups[key] = catalog[key]
		access.Granted[key] = true
	}
	return access, nil
}

// GetUserGroupAccess resolves the effective group access for one user.
func GetUserGroupAccess(userID int) (*UserGroupAccess, error) {
	if userID <= 0 {
		return nil, errors.New("user id is required")
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	return ResolveUserGroupAccess(model.DB, userID, user.Group)
}

// GetUserEffectiveGroups returns the user's complete effective group set.
func GetUserEffectiveGroups(userID int) (map[string]string, error) {
	access, err := GetUserGroupAccess(userID)
	if err != nil {
		return nil, err
	}
	return access.Groups, nil
}

// IsUserGroupAllowed reports whether a user may use one fixed group key.
// Root users bypass grant checks (but group existence is checked by the
// resolver's deprecated-group guard in the auth path).
func IsUserGroupAllowed(userID int, groupKey string) (bool, error) {
	groupKey = strings.TrimSpace(groupKey)
	if groupKey == "" || groupKey == "auto" {
		return false, nil
	}
	if model.IsRootUser(userID) {
		return true, nil
	}
	groups, err := GetUserEffectiveGroups(userID)
	if err != nil {
		return false, err
	}
	_, ok := groups[groupKey]
	return ok, nil
}

// GetUserAutoGroupByID preserves the configured auto-group ordering while
// resolving the user's complete effective permission set by user id.
func GetUserAutoGroupByID(userID int) ([]string, error) {
	groups, err := GetUserEffectiveGroups(userID)
	if err != nil {
		return nil, err
	}
	return GetAutoGroupsForUser(groups), nil
}

// GetAutoGroupsForUser preserves the configured preference order while
// ensuring auto routing considers every effective group, including groups
// not listed in the global auto-group preference.
func GetAutoGroupsForUser(groups map[string]string) []string {
	result := make([]string, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, key := range setting.GetAutoGroups() {
		if _, ok := groups[key]; !ok {
			continue
		}
		result = append(result, key)
		seen[key] = struct{}{}
	}
	remaining := make([]string, 0, len(groups)-len(result))
	for key := range groups {
		if _, ok := seen[key]; !ok {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	return append(result, remaining...)
}
