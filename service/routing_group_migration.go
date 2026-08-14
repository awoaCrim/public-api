package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// Compatibility records intentionally live in the migration service rather
// than a domain model, so a rolling deployment can read the old tables once
// without keeping the bypass model in the application domain. The old tables
// are never written or deleted.
type legacyRoutingGroupRecord struct {
	Id  int    `gorm:"column:id"`
	Key string `gorm:"column:key"`
}

func (legacyRoutingGroupRecord) TableName() string { return "routing_groups" }

type legacyUserRoutingGrantRecord struct {
	UserId         int   `gorm:"column:user_id"`
	RoutingGroupId int   `gorm:"column:routing_group_id"`
	ExpiresAt      int64 `gorm:"column:expires_at"`
}

func (legacyUserRoutingGrantRecord) TableName() string {
	return "user_routing_group_grants"
}

type legacyTokenRoutingRecord struct {
	Id             int    `gorm:"column:id"`
	Group          string `gorm:"column:group"`
	RoutingMode    string `gorm:"column:routing_mode"`
	RoutingGroupId *int   `gorm:"column:routing_group_id"`
}

func (legacyTokenRoutingRecord) TableName() string { return "tokens" }

// RoutingGroupMigrationReport is the read-only dry-run result of the legacy
// routing-group compatibility migration. It never mutates the old tables.
type RoutingGroupMigrationReport struct {
	// MappedGroups maps mappable legacy group ids to catalog keys.
	MappedGroups map[int]string `json:"mapped_groups"`
	// UnmappableGroups lists legacy group keys absent from the catalog.
	UnmappableGroups []string `json:"unmappable_groups"`
	// GrantImports previews the grants that would be created or materially updated.
	GrantImports []RoutingGrantImportPreview `json:"grant_imports"`
	// UnmappableGrants lists active legacy grants whose routing group cannot be mapped.
	UnmappableGrants []RoutingUnmappableGrantPreview `json:"unmappable_grants"`
	// TokenUpdates previews token group normalizations.
	TokenUpdates []RoutingTokenUpdatePreview `json:"token_updates"`
	// UnmappableTokens lists token ids referencing unmappable groups; these
	// stay unchanged (fail-closed) so administrators can decide.
	UnmappableTokens []int `json:"unmappable_tokens"`
}

type RoutingGrantImportPreview struct {
	UserId    int    `json:"user_id"`
	GroupKey  string `json:"group_key"`
	ExpiresAt int64  `json:"expires_at"`
}

type RoutingUnmappableGrantPreview struct {
	UserId         int   `json:"user_id"`
	RoutingGroupId int   `json:"routing_group_id"`
	ExpiresAt      int64 `json:"expires_at"`
}

type RoutingTokenUpdatePreview struct {
	TokenId  int    `json:"token_id"`
	OldGroup string `json:"old_group"`
	NewGroup string `json:"new_group"`
}

// migrationScan reads the legacy tables (read-only) and computes the full
// preview state.
func migrationScan(tx *gorm.DB) (*RoutingGroupMigrationReport, error) {
	report := &RoutingGroupMigrationReport{
		MappedGroups: map[int]string{},
	}
	groupKeys, err := scanLegacyRoutingGroupKeys(tx, report)
	if err != nil {
		return nil, err
	}
	if err := scanLegacyRoutingGrants(tx, groupKeys, report); err != nil {
		return nil, err
	}
	if err := scanLegacyTokenGroups(tx, groupKeys, report); err != nil {
		return nil, err
	}
	return report, nil
}

func scanLegacyRoutingGroupKeys(tx *gorm.DB, report *RoutingGroupMigrationReport) (map[int]string, error) {
	keys := make(map[int]string)
	if !tx.Migrator().HasTable("routing_groups") {
		return keys, nil
	}
	catalog := GetLegacyGroupCatalog()
	var groups []legacyRoutingGroupRecord
	if err := tx.Find(&groups).Error; err != nil {
		return nil, err
	}
	for _, group := range groups {
		key := strings.ToLower(strings.TrimSpace(group.Key))
		if key == "" || key == "auto" {
			continue
		}
		if _, ok := catalog[key]; !ok {
			report.UnmappableGroups = append(report.UnmappableGroups, group.Key)
			continue
		}
		keys[group.Id] = key
		report.MappedGroups[group.Id] = key
	}
	return keys, nil
}

func scanLegacyRoutingGrants(tx *gorm.DB, groupKeys map[int]string, report *RoutingGroupMigrationReport) error {
	if !tx.Migrator().HasTable("user_routing_group_grants") {
		return nil
	}
	var grants []legacyUserRoutingGrantRecord
	if err := tx.Find(&grants).Error; err != nil {
		return err
	}
	type grantIdentity struct {
		userID   int
		groupKey string
	}
	type orphanGrantIdentity struct {
		userID         int
		routingGroupID int
	}
	merged := make(map[grantIdentity]int64)
	order := make([]grantIdentity, 0, len(grants))
	orphaned := make(map[orphanGrantIdentity]int64)
	orphanOrder := make([]orphanGrantIdentity, 0)
	for _, grant := range grants {
		if grant.UserId <= 0 {
			continue
		}
		key, ok := groupKeys[grant.RoutingGroupId]
		if !ok {
			identity := orphanGrantIdentity{userID: grant.UserId, routingGroupID: grant.RoutingGroupId}
			if _, exists := orphaned[identity]; !exists {
				orphanOrder = append(orphanOrder, identity)
				orphaned[identity] = grant.ExpiresAt
			} else {
				orphaned[identity] = mergeGrantExpiry(orphaned[identity], grant.ExpiresAt)
			}
			continue
		}
		identity := grantIdentity{userID: grant.UserId, groupKey: key}
		if _, exists := merged[identity]; !exists {
			order = append(order, identity)
			merged[identity] = grant.ExpiresAt
		} else {
			merged[identity] = mergeGrantExpiry(merged[identity], grant.ExpiresAt)
		}
	}
	now := common.GetTimestamp()
	for _, identity := range orphanOrder {
		expiresAt := orphaned[identity]
		if expiresAt != 0 && expiresAt <= now {
			continue
		}
		report.UnmappableGrants = append(report.UnmappableGrants, RoutingUnmappableGrantPreview{
			UserId:         identity.userID,
			RoutingGroupId: identity.routingGroupID,
			ExpiresAt:      expiresAt,
		})
	}
	existingByIdentity := make(map[grantIdentity]model.UserGroupGrant, len(order))
	if len(order) > 0 {
		userIDs := make([]int, 0, len(order))
		seenUserIDs := make(map[int]struct{}, len(order))
		for _, identity := range order {
			if _, exists := seenUserIDs[identity.userID]; exists {
				continue
			}
			seenUserIDs[identity.userID] = struct{}{}
			userIDs = append(userIDs, identity.userID)
		}
		var existingGrants []model.UserGroupGrant
		if err := tx.Where("user_id IN ?", userIDs).Order("id ASC").Find(&existingGrants).Error; err != nil {
			return err
		}
		for _, existing := range existingGrants {
			identity := grantIdentity{userID: existing.UserId, groupKey: existing.GroupKey}
			if _, relevant := merged[identity]; !relevant {
				continue
			}
			if _, exists := existingByIdentity[identity]; !exists {
				existingByIdentity[identity] = existing
			}
		}
	}
	for _, identity := range order {
		preview := RoutingGrantImportPreview{
			UserId:    identity.userID,
			GroupKey:  identity.groupKey,
			ExpiresAt: merged[identity],
		}
		existing, exists := existingByIdentity[identity]
		if !exists || len(legacyRoutingGrantUpdates(existing, preview)) > 0 {
			report.GrantImports = append(report.GrantImports, preview)
		}
	}
	return nil
}

func scanLegacyTokenGroups(tx *gorm.DB, groupKeys map[int]string, report *RoutingGroupMigrationReport) error {
	if !tx.Migrator().HasTable("tokens") {
		return nil
	}
	if !tx.Migrator().HasColumn("tokens", "routing_mode") ||
		!tx.Migrator().HasColumn("tokens", "routing_group_id") {
		return nil
	}
	var tokens []legacyTokenRoutingRecord
	if err := tx.Find(&tokens).Error; err != nil {
		return err
	}
	for _, token := range tokens {
		rawGroup := strings.TrimSpace(token.Group)
		group := strings.ToLower(rawGroup)
		desired := group
		switch {
		case strings.EqualFold(group, "auto"), token.RoutingMode == "legacy_auto":
			desired = "auto"
		case group == "" && token.RoutingGroupId != nil:
			mapped, ok := groupKeys[*token.RoutingGroupId]
			if !ok {
				report.UnmappableTokens = append(report.UnmappableTokens, token.Id)
				continue
			}
			desired = mapped
		}
		if desired == "" || (desired == group && rawGroup == desired) {
			continue
		}
		report.TokenUpdates = append(report.TokenUpdates, RoutingTokenUpdatePreview{
			TokenId:  token.Id,
			OldGroup: token.Group,
			NewGroup: desired,
		})
	}
	return nil
}

// PreviewRoutingGroupMigration returns the read-only dry-run report. It
// never writes and is safe to run on production data.
func PreviewRoutingGroupMigration() (*RoutingGroupMigrationReport, error) {
	if model.DB == nil || !model.DB.Migrator().HasTable(&model.UserGroupGrant{}) {
		return &RoutingGroupMigrationReport{MappedGroups: map[int]string{}}, nil
	}
	return migrationScan(model.DB)
}

// MigrateRoutingGroupCompatibilityData imports mappings that can be expressed
// by the unified original-group model. It is idempotent and never deletes or
// rewrites the old tables, so it is safe to run during a rolling deployment.
// Legacy grants are converted to ordinary manual grants because the old
// routing-group source no longer exists after the compatibility release.
// Unmappable references are reported and left untouched (fail-closed).
func MigrateRoutingGroupCompatibilityData() error {
	if model.DB == nil || !model.DB.Migrator().HasTable(&model.UserGroupGrant{}) {
		return nil
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		report, err := migrationScan(tx)
		if err != nil {
			return err
		}
		for _, preview := range report.GrantImports {
			if err := applyLegacyRoutingGrant(tx, preview); err != nil {
				return err
			}
		}
		for _, update := range report.TokenUpdates {
			if err := tx.Table("tokens").Where("id = ?", update.TokenId).Update("group", update.NewGroup).Error; err != nil {
				return err
			}
		}
		for _, groupKey := range report.UnmappableGroups {
			common.SysLog(fmt.Sprintf("routing group %q cannot be mapped to the legacy group catalog", groupKey))
		}
		for _, grant := range report.UnmappableGrants {
			common.SysLog(fmt.Sprintf("user %d has an active legacy grant referencing unmappable routing group %d; left unchanged", grant.UserId, grant.RoutingGroupId))
		}
		for _, tokenId := range report.UnmappableTokens {
			common.SysLog(fmt.Sprintf("token %d references an unmappable routing group; left unchanged", tokenId))
		}
		return nil
	})
}

func applyLegacyRoutingGrant(tx *gorm.DB, preview RoutingGrantImportPreview) error {
	var existing model.UserGroupGrant
	err := tx.Where("user_id = ? AND group_key = ?", preview.UserId, preview.GroupKey).
		Order("id ASC").First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return tx.Create(&model.UserGroupGrant{
			UserId:    preview.UserId,
			GroupKey:  preview.GroupKey,
			Source:    UserGroupGrantSourceManual,
			ExpiresAt: preview.ExpiresAt,
		}).Error
	case err != nil:
		return err
	default:
		updates := legacyRoutingGrantUpdates(existing, preview)
		if len(updates) > 0 {
			return tx.Model(&existing).Updates(updates).Error
		}
		return nil
	}
}

func legacyRoutingGrantUpdates(existing model.UserGroupGrant, preview RoutingGrantImportPreview) map[string]any {
	updates := make(map[string]any)
	if existing.Source == UserGroupGrantSourceRoutingCompat {
		updates["source"] = UserGroupGrantSourceManual
	}
	mergedExpiresAt := mergeGrantExpiry(existing.ExpiresAt, preview.ExpiresAt)
	if mergedExpiresAt != existing.ExpiresAt {
		updates["expires_at"] = mergedExpiresAt
	}
	return updates
}

// mergeGrantExpiry merges two grant expiries: 0 (permanent) wins, otherwise
// the later timestamp.
func mergeGrantExpiry(current, incoming int64) int64 {
	if current == 0 || incoming == 0 {
		return 0
	}
	if incoming > current {
		return incoming
	}
	return current
}

// Routing group compatibility migration options keys.
const (
	RoutingGroupMigrationVersionKey = "routing_group_migration_version"
	RoutingGroupMigrationVersion    = "1"
)

// RoutingGroupMigrationStatus is the persisted/derived state of the legacy
// compatibility migration.
type RoutingGroupMigrationStatus struct {
	Migrated         bool                            `json:"migrated"`
	Version          string                          `json:"version,omitempty"`
	UnmappableGroups []string                        `json:"unmappable_groups,omitempty"`
	UnmappableGrants []RoutingUnmappableGrantPreview `json:"unmappable_grants,omitempty"`
	UnmappableTokens []int                           `json:"unmappable_tokens,omitempty"`
	PendingGrants    int                             `json:"pending_grants,omitempty"`
	PendingTokens    int                             `json:"pending_tokens,omitempty"`
	InSync           bool                            `json:"in_sync"`
}

// RoutingGroupMigrationReadiness reports whether the legacy routing-group
// data is fully migratable. Active unmappable token or grant references and
// orphan group keys block readiness; the service itself keeps serving (the
// check is diagnostic, not a boot failure).
func RoutingGroupMigrationReadiness() (bool, []string, error) {
	if model.DB == nil || !model.DB.Migrator().HasTable(&model.UserGroupGrant{}) {
		return true, nil, nil
	}
	report, err := PreviewRoutingGroupMigration()
	if err != nil {
		return false, nil, err
	}
	var blockers []string
	if len(report.UnmappableTokens) > 0 {
		blockers = append(blockers, fmt.Sprintf("%d token(s) reference unmappable routing groups: %v", len(report.UnmappableTokens), report.UnmappableTokens))
	}
	if len(report.UnmappableGrants) > 0 {
		blockers = append(blockers, fmt.Sprintf("%d active grant(s) reference unmappable routing groups: %v", len(report.UnmappableGrants), report.UnmappableGrants))
	}
	if len(report.UnmappableGroups) > 0 {
		blockers = append(blockers, fmt.Sprintf("unmappable legacy routing group keys: %v", report.UnmappableGroups))
	}
	return len(blockers) == 0, blockers, nil
}

// MigrateRoutingGroupCompatibilityDataStrict runs the compatibility migration
// only when every active legacy reference can be mapped (fail-closed). On
// success it persists the migration version marker and a status summary;
// the legacy tables are never written or deleted. Idempotent.
func MigrateRoutingGroupCompatibilityDataStrict() (*RoutingGroupMigrationReport, error) {
	ready, blockers, err := RoutingGroupMigrationReadiness()
	if err != nil {
		return nil, err
	}
	if !ready {
		report, reportErr := PreviewRoutingGroupMigration()
		if reportErr != nil {
			return nil, reportErr
		}
		return report, fmt.Errorf("routing group compatibility migration blocked: %v", blockers)
	}
	if err := MigrateRoutingGroupCompatibilityData(); err != nil {
		return nil, err
	}
	status, err := GetRoutingGroupMigrationStatus()
	if err != nil {
		return nil, err
	}
	status.Migrated = true
	status.Version = RoutingGroupMigrationVersion
	if err := model.UpdateOption(RoutingGroupMigrationVersionKey, RoutingGroupMigrationVersion); err != nil {
		return nil, err
	}
	report, err := PreviewRoutingGroupMigration()
	if err != nil {
		return nil, err
	}
	return report, nil
}

// GetRoutingGroupMigrationStatus derives the current migration state: marker
// options plus a live dry-run so operators can reconcile without guessing.
func GetRoutingGroupMigrationStatus() (*RoutingGroupMigrationStatus, error) {
	status := &RoutingGroupMigrationStatus{}
	common.OptionMapRWMutex.RLock()
	version := common.OptionMap[RoutingGroupMigrationVersionKey]
	common.OptionMapRWMutex.RUnlock()
	if version != "" {
		status.Migrated = true
		status.Version = version
	}
	report, err := PreviewRoutingGroupMigration()
	if err != nil {
		return nil, err
	}
	status.UnmappableGroups = report.UnmappableGroups
	status.UnmappableGrants = report.UnmappableGrants
	status.UnmappableTokens = report.UnmappableTokens
	status.PendingGrants = len(report.GrantImports)
	status.PendingTokens = len(report.TokenUpdates)
	status.InSync = status.PendingGrants == 0 && status.PendingTokens == 0 && len(status.UnmappableGroups) == 0 && len(status.UnmappableGrants) == 0 && len(status.UnmappableTokens) == 0
	return status, nil
}
