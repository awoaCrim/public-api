package model

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"gorm.io/gorm"
)

// ChannelModelFixedEndpoint pins one published model of a channel to a single
// upstream endpoint (base URL). When a model has a fixed endpoint, every relay
// request for that model is rejected unless the channel's effective base URL
// matches the fixed endpoint. This protects a model from ever being served
// through a different upstream (for example after the channel base URL is
// edited by mistake).
type ChannelModelFixedEndpoint struct {
	ChannelId int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	Model     string `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	Endpoint  string `json:"endpoint" gorm:"type:varchar(2048);not null"`
}

func (ChannelModelFixedEndpoint) TableName() string {
	return "channel_model_fixed_endpoints"
}

// NormalizeChannelModelFixedEndpoint trims whitespace and trailing slashes so
// "https://api.example.com/" and "https://api.example.com" compare equal.
func NormalizeChannelModelFixedEndpoint(endpoint string) string {
	return strings.TrimRight(strings.TrimSpace(endpoint), "/")
}

// ValidateChannelModelFixedEndpoint reports whether endpoint is a usable
// http/https base URL.
func ValidateChannelModelFixedEndpoint(endpoint string) error {
	normalized := NormalizeChannelModelFixedEndpoint(endpoint)
	if normalized == "" {
		return fmt.Errorf("fixed endpoint must not be empty")
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return fmt.Errorf("invalid fixed endpoint %q: %v", endpoint, err)
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("invalid fixed endpoint %q: must use http or https", endpoint)
	}
	if parsed.Host == "" {
		return fmt.Errorf("invalid fixed endpoint %q: missing host", endpoint)
	}
	return nil
}

// fixedEndpointChannelIDModelIndex caches channel_id -> model -> normalized
// fixed endpoint, mirroring channelsIDM. It is refreshed on every full channel
// cache sync and on single channel updates.
var fixedEndpointChannelIDModelIndex map[int]map[string]string
var fixedEndpointIndexLock sync.RWMutex

// GetChannelModelFixedEndpoint returns the normalized fixed endpoint pinned to
// (channelID, model), or "" when none is configured. The memory index is used
// when available; otherwise the database is queried.
func GetChannelModelFixedEndpoint(channelID int, modelName string) string {
	fixedEndpointIndexLock.RLock()
	if fixedEndpointChannelIDModelIndex != nil {
		if channelEndpoints, ok := fixedEndpointChannelIDModelIndex[channelID]; ok {
			fixedEndpointIndexLock.RUnlock()
			return channelEndpoints[modelName]
		}
		// index alive and channel known to have no fixed endpoints
		fixedEndpointIndexLock.RUnlock()
		return ""
	}
	fixedEndpointIndexLock.RUnlock()
	if DB == nil || !DB.Migrator().HasTable(&ChannelModelFixedEndpoint{}) {
		return ""
	}
	var row ChannelModelFixedEndpoint
	if err := DB.Where("channel_id = ? AND model = ?", channelID, modelName).First(&row).Error; err != nil {
		return ""
	}
	return NormalizeChannelModelFixedEndpoint(row.Endpoint)
}

// CheckChannelModelFixedEndpoint rejects requests whose channel base URL does
// not match the model's fixed endpoint. Empty fixed endpoint means no
// restriction.
func CheckChannelModelFixedEndpoint(channelID int, modelName string, currentBaseURL string) error {
	fixed := GetChannelModelFixedEndpoint(channelID, modelName)
	if fixed == "" {
		return nil
	}
	if NormalizeChannelModelFixedEndpoint(currentBaseURL) != fixed {
		return fmt.Errorf("model %s is pinned to fixed endpoint %s, but the channel endpoint %s does not match", modelName, fixed, currentBaseURL)
	}
	return nil
}

// LoadChannelModelFixedEndpoints reads every fixed endpoint row of a channel
// as model -> normalized endpoint (sorted for stable responses).
func LoadChannelModelFixedEndpoints(channelID int) (map[string]string, error) {
	if DB == nil || !DB.Migrator().HasTable(&ChannelModelFixedEndpoint{}) {
		return nil, nil
	}
	var rows []ChannelModelFixedEndpoint
	if err := DB.Where("channel_id = ?", channelID).Order("model ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	endpoints := make(map[string]string, len(rows))
	for _, row := range rows {
		endpoints[row.Model] = NormalizeChannelModelFixedEndpoint(row.Endpoint)
	}
	return endpoints, nil
}

// ChannelModelFixedEndpointsEqual compares two fixed-endpoint maps. A nil map
// means "keep existing" and only equals another nil map.
func ChannelModelFixedEndpointsEqual(a, b *map[string]string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(*a) != len(*b) {
		return false
	}
	for model, endpoint := range *a {
		if NormalizeChannelModelFixedEndpoint((*b)[model]) != NormalizeChannelModelFixedEndpoint(endpoint) {
			return false
		}
	}
	return true
}

// ReplaceChannelModelFixedEndpoints validates and atomically replaces the
// fixed endpoint rows of one channel. Every model must be published by the
// channel and every endpoint must be a valid http/https URL.
func ReplaceChannelModelFixedEndpoints(tx *gorm.DB, channelID int, endpoints map[string]string, publishedModels map[string]struct{}) error {
	if tx == nil {
		tx = DB
	}
	for model, endpoint := range endpoints {
		model = strings.TrimSpace(model)
		if model == "" {
			return fmt.Errorf("invalid channel fixed endpoint: empty model")
		}
		if _, ok := publishedModels[model]; !ok {
			return fmt.Errorf("model %s is not published by channel", model)
		}
		if err := ValidateChannelModelFixedEndpoint(endpoint); err != nil {
			return err
		}
	}

	if tx.Migrator().HasTable(&ChannelModelFixedEndpoint{}) {
		if err := tx.Where("channel_id = ?", channelID).Delete(&ChannelModelFixedEndpoint{}).Error; err != nil {
			return err
		}
	}
	if len(endpoints) == 0 {
		return nil
	}
	rows := make([]ChannelModelFixedEndpoint, 0, len(endpoints))
	for model, endpoint := range endpoints {
		rows = append(rows, ChannelModelFixedEndpoint{
			ChannelId: channelID,
			Model:     strings.TrimSpace(model),
			Endpoint:  NormalizeChannelModelFixedEndpoint(endpoint),
		})
	}
	return tx.Create(&rows).Error
}

// DeleteChannelModelFixedEndpoints removes every fixed endpoint row of a
// channel.
func DeleteChannelModelFixedEndpoints(tx *gorm.DB, channelID int) error {
	if tx == nil {
		tx = DB
	}
	if tx.Migrator().HasTable(&ChannelModelFixedEndpoint{}) {
		return tx.Where("channel_id = ?", channelID).Delete(&ChannelModelFixedEndpoint{}).Error
	}
	return nil
}

// reloadFixedEndpointIndex rebuilds the in-memory fixed endpoint index from
// the database. Callers must hold fixedEndpointIndexLock.
func reloadFixedEndpointIndex() {
	var rows []ChannelModelFixedEndpoint
	if DB != nil && DB.Migrator().HasTable(&ChannelModelFixedEndpoint{}) {
		_ = DB.Find(&rows).Error
	}
	index := make(map[int]map[string]string, len(rows))
	for _, row := range rows {
		modelEndpoints := index[row.ChannelId]
		if modelEndpoints == nil {
			modelEndpoints = make(map[string]string)
			index[row.ChannelId] = modelEndpoints
		}
		modelEndpoints[row.Model] = NormalizeChannelModelFixedEndpoint(row.Endpoint)
	}
	fixedEndpointChannelIDModelIndex = index
}

// RefreshChannelFixedEndpointIndex reloads the in-memory fixed endpoint index
// for all channels. Called from the full channel cache sync.
func RefreshChannelFixedEndpointIndex() {
	fixedEndpointIndexLock.Lock()
	reloadFixedEndpointIndex()
	fixedEndpointIndexLock.Unlock()
}

// RefreshChannelFixedEndpointEntries reloads one channel's fixed endpoint
// entries after an update or delete.
func RefreshChannelFixedEndpointEntries(channelID int) {
	entries, err := LoadChannelModelFixedEndpoints(channelID)
	if err != nil {
		return
	}
	fixedEndpointIndexLock.Lock()
	if fixedEndpointChannelIDModelIndex == nil {
		fixedEndpointIndexLock.Unlock()
		return
	}
	if len(entries) == 0 {
		delete(fixedEndpointChannelIDModelIndex, channelID)
	} else {
		fixedEndpointChannelIDModelIndex[channelID] = entries
	}
	fixedEndpointIndexLock.Unlock()
}