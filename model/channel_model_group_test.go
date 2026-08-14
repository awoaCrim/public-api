package model

import (
	"sort"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/stretchr/testify/require"
)

func channelModelGroupTestAbilities(t *testing.T, channelID int) []string {
	t.Helper()
	var abilities []Ability
	require.NoError(t, DB.Where("channel_id = ?", channelID).Find(&abilities).Error)
	result := make([]string, 0, len(abilities))
	for _, ability := range abilities {
		result = append(result, ability.Model+"|"+ability.Group)
	}
	sort.Strings(result)
	return result
}

func TestChannelModelGroupTriStateProjection(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&ChannelModelGroupOverride{}, &ChannelModelGroupDisabled{}))

	channel := Channel{
		Key:    "tri-state-key",
		Name:   "tri-state-channel",
		Status: common.ChannelStatusEnabled,
		Group:  "default,vip",
		Models: "model-a,model-b,model-c",
	}
	channel.ModelGroupModes = &[]ChannelModelGroupModeInput{
		{Model: "model-a", Mode: ChannelModelGroupModeInherit},
		{Model: "model-b", Mode: ChannelModelGroupModeCustom, Groups: []string{"vip"}},
		{Model: "model-c", Mode: ChannelModelGroupModeDisabled},
	}
	require.NoError(t, channel.Insert())
	t.Cleanup(func() { require.NoError(t, channel.Delete()) })

	require.Equal(t, []string{
		"model-a|default", "model-a|vip",
		"model-b|vip",
	}, channelModelGroupTestAbilities(t, channel.Id),
		"inherit publishes the channel groups, custom only its own set, disabled publishes nothing")

	var overrides []ChannelModelGroupOverride
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&overrides).Error)
	require.Len(t, overrides, 1)
	require.Equal(t, "model-b", overrides[0].Model)
	require.Equal(t, "vip", overrides[0].GroupKey)

	var disabled []ChannelModelGroupDisabled
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&disabled).Error)
	require.Len(t, disabled, 1)
	require.Equal(t, "model-c", disabled[0].Model)
}

func TestChannelModelGroupModesValidation(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&ChannelModelGroupOverride{}, &ChannelModelGroupDisabled{}))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"svip":1}`)) })
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"默认分组","vip":"VIP"}`))

	channel := Channel{
		Key:    "validation-key",
		Name:   "validation-channel",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
		Models: "model-a",
	}
	require.NoError(t, channel.Insert())
	t.Cleanup(func() { require.NoError(t, channel.Delete()) })

	replace := func(modes []ChannelModelGroupModeInput) error {
		return ReplaceChannelModelGroupPolicies(DB, channel.Id, modes, channelPublishedModels(t, channel.Id))
	}

	// Unknown mode rejected.
	require.Error(t, replace([]ChannelModelGroupModeInput{{Model: "model-a", Mode: "banana"}}))

	// Model not published by the channel rejected.
	require.Error(t, replace([]ChannelModelGroupModeInput{{Model: "ghost", Mode: ChannelModelGroupModeDisabled}}))

	// Group key outside the catalog rejected.
	require.Error(t, replace([]ChannelModelGroupModeInput{{Model: "model-a", Mode: ChannelModelGroupModeCustom, Groups: []string{"unknown-group"}}}))

	// custom requires at least one group.
	require.Error(t, replace([]ChannelModelGroupModeInput{{Model: "model-a", Mode: ChannelModelGroupModeCustom}}))

	// inherit and disabled must not carry custom groups.
	require.Error(t, replace([]ChannelModelGroupModeInput{{Model: "model-a", Mode: ChannelModelGroupModeInherit, Groups: []string{"vip"}}}))
	require.Error(t, replace([]ChannelModelGroupModeInput{{Model: "model-a", Mode: ChannelModelGroupModeDisabled, Groups: []string{"vip"}}}))

	// auto is never a valid group key.
	require.Error(t, replace([]ChannelModelGroupModeInput{{Model: "model-a", Mode: ChannelModelGroupModeCustom, Groups: []string{"auto"}}}))

	// Duplicate model entries rejected.
	require.Error(t, replace([]ChannelModelGroupModeInput{
		{Model: "model-a", Mode: ChannelModelGroupModeDisabled},
		{Model: "model-a", Mode: ChannelModelGroupModeDisabled},
	}))
}

func TestChannelModelGroupUpdateRebuildsAbilities(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&ChannelModelGroupOverride{}, &ChannelModelGroupDisabled{}))

	channel := Channel{
		Key:    "update-key",
		Name:   "update-channel",
		Status: common.ChannelStatusEnabled,
		Group:  "default,vip",
		Models: "model-a",
	}
	require.NoError(t, channel.Insert())
	t.Cleanup(func() { require.NoError(t, channel.Delete()) })
	require.Equal(t, []string{"model-a|default", "model-a|vip"}, channelModelGroupTestAbilities(t, channel.Id))

	channel.ModelGroupModes = &[]ChannelModelGroupModeInput{
		{Model: "model-a", Mode: ChannelModelGroupModeCustom, Groups: []string{"default"}},
	}
	require.NoError(t, channel.Update())
	require.Equal(t, []string{"model-a|default"}, channelModelGroupTestAbilities(t, channel.Id))

	// Switching to disabled removes the ability entirely and leaves no
	// override rows behind.
	channel.ModelGroupModes = &[]ChannelModelGroupModeInput{
		{Model: "model-a", Mode: ChannelModelGroupModeDisabled},
	}
	require.NoError(t, channel.Update())
	require.Empty(t, channelModelGroupTestAbilities(t, channel.Id))
	var overrides []ChannelModelGroupOverride
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&overrides).Error)
	require.Empty(t, overrides)

	// And back to inherit restores the cartesian product.
	channel.ModelGroupModes = &[]ChannelModelGroupModeInput{
		{Model: "model-a", Mode: ChannelModelGroupModeInherit},
	}
	require.NoError(t, channel.Update())
	require.Equal(t, []string{"model-a|default", "model-a|vip"}, channelModelGroupTestAbilities(t, channel.Id))
}

func TestChannelModelGroupBatchLifecycle(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&ChannelModelGroupOverride{}, &ChannelModelGroupDisabled{}))

	withPolicy := Channel{
		Key:    "batch-policy-key",
		Name:   "batch-policy-channel",
		Status: common.ChannelStatusEnabled,
		Group:  "default,vip",
		Models: "batch-model",
	}
	withPolicy.ModelGroupModes = &[]ChannelModelGroupModeInput{
		{Model: "batch-model", Mode: ChannelModelGroupModeCustom, Groups: []string{"vip"}},
	}
	plain := Channel{
		Key:    "batch-plain-key",
		Name:   "batch-plain-channel",
		Status: common.ChannelStatusEnabled,
		Group:  "default,vip",
		Models: "batch-model",
	}
	require.NoError(t, BatchInsertChannels([]Channel{withPolicy, plain}))

	var inserted []Channel
	require.NoError(t, DB.Where("name IN ?", []string{withPolicy.Name, plain.Name}).Find(&inserted).Error)
	require.Len(t, inserted, 2)
	var policyChannel, plainChannel Channel
	require.NoError(t, DB.Where("name = ?", withPolicy.Name).First(&policyChannel).Error)
	require.NoError(t, DB.Where("name = ?", plain.Name).First(&plainChannel).Error)
	t.Cleanup(func() { _, _ = BatchDeleteChannels([]int{policyChannel.Id, plainChannel.Id}) })

	require.Equal(t, []string{"batch-model|vip"}, channelModelGroupTestAbilities(t, policyChannel.Id))
	require.Equal(t, []string{"batch-model|default", "batch-model|vip"}, channelModelGroupTestAbilities(t, plainChannel.Id))

	_, err := BatchDeleteChannels([]int{policyChannel.Id, plainChannel.Id})
	require.NoError(t, err)
	var overrides []ChannelModelGroupOverride
	var disabled []ChannelModelGroupDisabled
	require.Zero(t, DB.Where("channel_id IN ?", []int{policyChannel.Id, plainChannel.Id}).Find(&overrides).RowsAffected)
	require.Zero(t, DB.Where("channel_id IN ?", []int{policyChannel.Id, plainChannel.Id}).Find(&disabled).RowsAffected)
	require.Zero(t, DB.Where("channel_id IN ?", []int{policyChannel.Id, plainChannel.Id}).Find(&[]Ability{}).RowsAffected)
}

func TestChannelModelGroupModesRoundTrip(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&ChannelModelGroupOverride{}, &ChannelModelGroupDisabled{}))

	channel := Channel{
		Key:    "roundtrip-key",
		Name:   "roundtrip-channel",
		Status: common.ChannelStatusEnabled,
		Group:  "default,vip",
		Models: "model-a,model-b",
	}
	channel.ModelGroupModes = &[]ChannelModelGroupModeInput{
		{Model: "model-a", Mode: ChannelModelGroupModeCustom, Groups: []string{"vip"}},
		{Model: "model-b", Mode: ChannelModelGroupModeDisabled},
	}
	require.NoError(t, channel.Insert())
	t.Cleanup(func() { require.NoError(t, channel.Delete()) })

	modes, err := LoadChannelModelGroupModes(channel.Id)
	require.NoError(t, err)
	require.Len(t, modes, 2)

	byModel := map[string]ChannelModelGroupModeInput{}
	for _, mode := range modes {
		byModel[mode.Model] = mode
	}
	require.Equal(t, ChannelModelGroupModeCustom, byModel["model-a"].Mode)
	require.Equal(t, []string{"vip"}, byModel["model-a"].Groups)
	require.Equal(t, ChannelModelGroupModeDisabled, byModel["model-b"].Mode)
	require.Empty(t, byModel["model-b"].Groups)
}

func channelPublishedModels(t *testing.T, channelID int) map[string]struct{} {
	t.Helper()
	var channel Channel
	require.NoError(t, DB.First(&channel, channelID).Error)
	models := map[string]struct{}{}
	for _, name := range channel.GetModels() {
		models[name] = struct{}{}
	}
	return models
}
