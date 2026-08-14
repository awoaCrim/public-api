package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"gorm.io/gorm"
)

// Channel model group tri-state: every published model of a channel is either
// published to the channel default groups (inherit), to its own explicit group
// set (custom), or to no group at all while remaining in the channel model
// list (disabled).
const (
	ChannelModelGroupModeInherit  = "inherit"
	ChannelModelGroupModeCustom   = "custom"
	ChannelModelGroupModeDisabled = "disabled"
)

// ChannelModelGroupOverride stores one custom group row of a model. Rows
// exist only for models in custom mode.
type ChannelModelGroupOverride struct {
	ChannelId int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	Model     string `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	GroupKey  string `json:"group_key" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	SortOrder int    `json:"sort_order" gorm:"not null;default:0"`
}

func (ChannelModelGroupOverride) TableName() string {
	return "channel_model_group_overrides"
}

// ChannelModelGroupDisabled marks one published model as disabled (no
// ability rows). Inherit is expressed by the absence of any policy rows.
type ChannelModelGroupDisabled struct {
	ChannelId int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	Model     string `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
}

func (ChannelModelGroupDisabled) TableName() string {
	return "channel_model_group_disabled"
}

// ChannelModelGroupModeInput is the normalized one-row-per-model policy DTO.
// Mode is one of inherit / custom / disabled; Groups is only meaningful (and
// only non-empty) for custom.
type ChannelModelGroupModeInput struct {
	Model  string   `json:"model"`
	Mode   string   `json:"mode"`
	Groups []string `json:"groups,omitempty"`
}

// IsChannelModelGroupModeValid reports whether mode is a known tri-state.
func IsChannelModelGroupModeValid(mode string) bool {
	switch mode {
	case ChannelModelGroupModeInherit, ChannelModelGroupModeCustom, ChannelModelGroupModeDisabled:
		return true
	}
	return false
}

// channelModelGroupCatalog returns the current legacy group catalog:
// user usable groups plus every configured group ratio key. Groups only live
// here; the tables above never invent groups of their own.
func channelModelGroupCatalog() map[string]struct{} {
	catalog := make(map[string]struct{})
	for key := range ratio_setting.GetGroupRatioCopy() {
		key = strings.TrimSpace(key)
		if key != "" && key != "auto" {
			catalog[key] = struct{}{}
		}
	}
	for key := range setting.GetUserUsableGroupsCopy() {
		key = strings.TrimSpace(key)
		if key != "" && key != "auto" {
			catalog[key] = struct{}{}
		}
	}
	return catalog
}

// ResolveChannelModelGroups returns the effective legacy groups for one
// published model: disabled publishes nothing, custom publishes its own
// rows, and the absence of any policy publishes the channel default groups.
func ResolveChannelModelGroups(db *gorm.DB, channelID int, modelName string, defaultGroups []string) ([]string, error) {
	if db == nil {
		db = DB
	}
	hasPolicyTables := db != nil && db.Migrator().HasTable(&ChannelModelGroupOverride{})
	return resolveChannelModelGroups(db, channelID, modelName, defaultGroups, hasPolicyTables)
}

// resolveChannelModelGroups is the hot-path variant that avoids repeated
// table-existence checks during ability projection builds.
func resolveChannelModelGroups(db *gorm.DB, channelID int, modelName string, defaultGroups []string, hasPolicyTables bool) ([]string, error) {
	if !hasPolicyTables {
		return defaultGroups, nil
	}
	if db.Migrator().HasTable(&ChannelModelGroupDisabled{}) {
		var count int64
		if err := db.Model(&ChannelModelGroupDisabled{}).
			Where("channel_id = ? AND model = ?", channelID, modelName).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return []string{}, nil
		}
	}
	var overrides []ChannelModelGroupOverride
	if err := db.Where("channel_id = ? AND model = ?", channelID, modelName).
		Order("sort_order ASC, group_key ASC").Find(&overrides).Error; err != nil {
		return nil, err
	}
	if len(overrides) > 0 {
		groups := make([]string, 0, len(overrides))
		seen := make(map[string]struct{}, len(overrides))
		for _, override := range overrides {
			group := strings.TrimSpace(override.GroupKey)
			if group == "" || group == "auto" {
				continue
			}
			if _, ok := seen[group]; ok {
				continue
			}
			seen[group] = struct{}{}
			groups = append(groups, group)
		}
		return groups, nil
	}
	return defaultGroups, nil
}

// LoadChannelModelGroupModes reads the current tri-state for every model
// that has an explicit policy, keyed by model name and sorted for stable
// responses.
func LoadChannelModelGroupModes(channelID int) ([]ChannelModelGroupModeInput, error) {
	if DB == nil || !DB.Migrator().HasTable(&ChannelModelGroupOverride{}) {
		return nil, nil
	}
	var overrides []ChannelModelGroupOverride
	if err := DB.Where("channel_id = ?", channelID).Order("model ASC, sort_order ASC, group_key ASC").Find(&overrides).Error; err != nil {
		return nil, err
	}
	var disabled []ChannelModelGroupDisabled
	if DB.Migrator().HasTable(&ChannelModelGroupDisabled{}) {
		if err := DB.Where("channel_id = ?", channelID).Find(&disabled).Error; err != nil {
			return nil, err
		}
	}

	byModel := make(map[string]*ChannelModelGroupModeInput)
	for _, override := range overrides {
		mode := byModel[override.Model]
		if mode == nil {
			mode = &ChannelModelGroupModeInput{Model: override.Model, Mode: ChannelModelGroupModeCustom}
			byModel[override.Model] = mode
		}
		mode.Groups = append(mode.Groups, override.GroupKey)
	}
	for _, row := range disabled {
		byModel[row.Model] = &ChannelModelGroupModeInput{Model: row.Model, Mode: ChannelModelGroupModeDisabled}
	}

	modes := make([]ChannelModelGroupModeInput, 0, len(byModel))
	for _, mode := range byModel {
		modes = append(modes, *mode)
	}
	sort.Slice(modes, func(i, j int) bool { return modes[i].Model < modes[j].Model })
	return modes, nil
}

// ReplaceChannelModelGroupPolicies validates and atomically replaces the
// tri-state policies of one channel. publishedModels must contain the
// channel's current published model names. inherit/disabled entries must not
// carry groups; custom entries need at least one catalog group; every model
// must be published; duplicates are rejected.
func ReplaceChannelModelGroupPolicies(tx *gorm.DB, channelID int, modes []ChannelModelGroupModeInput, publishedModels map[string]struct{}) error {
	if tx == nil {
		tx = DB
	}
	catalog := channelModelGroupCatalog()
	seenModels := make(map[string]struct{}, len(modes))
	for _, input := range modes {
		modelName := strings.TrimSpace(input.Model)
		if modelName == "" {
			return fmt.Errorf("invalid channel model group policy: empty model")
		}
		if !IsChannelModelGroupModeValid(input.Mode) {
			return fmt.Errorf("invalid channel model group mode: %s", input.Mode)
		}
		if _, ok := seenModels[modelName]; ok {
			return fmt.Errorf("duplicate channel model group policy for model %s", modelName)
		}
		seenModels[modelName] = struct{}{}
		if _, ok := publishedModels[modelName]; !ok {
			return fmt.Errorf("model %s is not published by channel", modelName)
		}
		switch input.Mode {
		case ChannelModelGroupModeCustom:
			if len(input.Groups) == 0 {
				return fmt.Errorf("custom channel model group policy for %s requires at least one group", modelName)
			}
			seen := make(map[string]struct{}, len(input.Groups))
			for _, group := range input.Groups {
				group = strings.TrimSpace(group)
				if group == "" || group == "auto" {
					return fmt.Errorf("invalid group %q in channel model group policy for %s", group, modelName)
				}
				if _, ok := catalog[group]; !ok {
					return fmt.Errorf("group %s does not exist in the group catalog", group)
				}
				if _, ok := seen[group]; ok {
					return fmt.Errorf("duplicate group %s in channel model group policy for %s", group, modelName)
				}
				seen[group] = struct{}{}
			}
		case ChannelModelGroupModeInherit, ChannelModelGroupModeDisabled:
			if len(input.Groups) > 0 {
				return fmt.Errorf("%s channel model group policy for %s must not carry groups", input.Mode, modelName)
			}
		}
	}

	if err := DeleteChannelModelGroupPolicies(tx, channelID); err != nil {
		return err
	}

	disabledRows := make([]ChannelModelGroupDisabled, 0, len(modes))
	overrideRows := make([]ChannelModelGroupOverride, 0, len(modes))
	for _, input := range modes {
		modelName := strings.TrimSpace(input.Model)
		switch input.Mode {
		case ChannelModelGroupModeDisabled:
			disabledRows = append(disabledRows, ChannelModelGroupDisabled{ChannelId: channelID, Model: modelName})
		case ChannelModelGroupModeCustom:
			for index, group := range input.Groups {
				overrideRows = append(overrideRows, ChannelModelGroupOverride{
					ChannelId: channelID,
					Model:     modelName,
					GroupKey:  strings.TrimSpace(group),
					SortOrder: index,
				})
			}
		}
	}
	if len(disabledRows) > 0 {
		if err := tx.Create(&disabledRows).Error; err != nil {
			return err
		}
	}
	if len(overrideRows) > 0 {
		if err := tx.Create(&overrideRows).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeleteChannelModelGroupPolicies removes every policy row of a channel.
func DeleteChannelModelGroupPolicies(tx *gorm.DB, channelID int) error {
	if tx == nil {
		tx = DB
	}
	if tx.Migrator().HasTable(&ChannelModelGroupOverride{}) {
		if err := tx.Where("channel_id = ?", channelID).Delete(&ChannelModelGroupOverride{}).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(&ChannelModelGroupDisabled{}) {
		return tx.Where("channel_id = ?", channelID).Delete(&ChannelModelGroupDisabled{}).Error
	}
	return nil
}
