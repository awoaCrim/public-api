package model

import (
	"strings"

	"gorm.io/gorm"
)

// UserGroupGrant stores one additional original-group permission for a user.
// The group itself is still defined by the existing usable-group settings and
// the channel ability tables; this relation only records the extra grant.
// Inherited (account-tier) groups are not stored here.
type UserGroupGrant struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId    int    `json:"user_id" gorm:"not null;index;uniqueIndex:ux_user_group_grant_source"`
	GroupKey  string `json:"group_key" gorm:"type:varchar(64);not null;uniqueIndex:ux_user_group_grant_source"`
	Source    string `json:"source" gorm:"type:varchar(32);not null;default:'manual';uniqueIndex:ux_user_group_grant_source"`
	ExpiresAt int64  `json:"expires_at" gorm:"not null;default:0;index"`
	SortOrder int    `json:"sort_order" gorm:"not null;default:0"`
}

func (UserGroupGrant) TableName() string { return "user_group_grants" }

// ListUserGroupGrants returns all grant rows of one user ordered by sort
// order and id for stable responses.
func ListUserGroupGrants(userId int) ([]UserGroupGrant, error) {
	var grants []UserGroupGrant
	err := DB.Where("user_id = ?", userId).Order("sort_order ASC, id ASC").Find(&grants).Error
	return grants, err
}

// ReplaceUserGroupGrantsWithSource atomically replaces every grant of one
// source for a user. groupKeys are pre-validated by the caller; each key is
// trimmed and de-duplicated. expiresAt applies to every new row (0 =
// permanent).
func ReplaceUserGroupGrantsWithSource(tx *gorm.DB, userId int, source string, groupKeys []string, expiresAt int64) error {
	if tx == nil {
		tx = DB
	}
	if err := tx.Where("user_id = ? AND source = ?", userId, source).Delete(&UserGroupGrant{}).Error; err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(groupKeys))
	rows := make([]UserGroupGrant, 0, len(groupKeys))
	for index, key := range groupKeys {
		key = strings.TrimSpace(key)
		if key == "" || key == "auto" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		rows = append(rows, UserGroupGrant{
			UserId:    userId,
			GroupKey:  key,
			Source:    source,
			ExpiresAt: expiresAt,
			SortOrder: index,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}
