package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelModelFixedEndpointValidation(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&ChannelModelFixedEndpoint{}))

	channel := Channel{
		Key:    "fixed-endpoint-validation-key",
		Name:   "fixed-endpoint-validation-channel",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
		Models: "model-a,model-b",
	}
	channel.ModelFixedEndpoints = &map[string]string{
		"model-a": "https://api.example.com/",
		"model-b": "https://api.example.com/v1",
	}
	require.NoError(t, channel.Insert())
	t.Cleanup(func() { require.NoError(t, channel.Delete()) })

	// saved rows are normalized (trailing slashes trimmed)
	require.Equal(t, "https://api.example.com", GetChannelModelFixedEndpoint(channel.Id, "model-a"))
	require.Equal(t, "https://api.example.com/v1", GetChannelModelFixedEndpoint(channel.Id, "model-b"))
	// unconfigured model has no fixed endpoint
	require.Equal(t, "", GetChannelModelFixedEndpoint(channel.Id, "model-c"))

	var rows []ChannelModelFixedEndpoint
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&rows).Error)
	require.Len(t, rows, 2)

	// publishing a fixed endpoint for an unpublished model is rejected
	err := ReplaceChannelModelFixedEndpoints(DB, channel.Id, map[string]string{
		"model-not-published": "https://api.example.com",
	}, map[string]struct{}{"model-a": {}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not published")

	// invalid endpoints are rejected
	for _, bad := range []string{
		"", "not-a-url", "ftp://api.example.com", "https://", "https:///no-host",
	} {
		err := ReplaceChannelModelFixedEndpoints(DB, channel.Id, map[string]string{
			"model-a": bad,
		}, map[string]struct{}{"model-a": {}})
		require.Error(t, err, "endpoint %q must be rejected", bad)
	}

	// nil map keeps existing rows untouched
	channel.ModelFixedEndpoints = nil
	require.NoError(t, channel.Update())
	require.Equal(t, "https://api.example.com", GetChannelModelFixedEndpoint(channel.Id, "model-a"))
}

func TestChannelModelFixedEndpointRejectsMismatchedEndpoint(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&ChannelModelFixedEndpoint{}))

	channel := Channel{
		Key:    "fixed-endpoint-check-key",
		Name:   "fixed-endpoint-check-channel",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
		Models: "model-a,model-b",
	}
	endpoints := map[string]string{
		"model-a": "https://api.allowed.example.com/",
	}
	channel.ModelFixedEndpoints = &endpoints
	require.NoError(t, channel.Insert())
	t.Cleanup(func() { require.NoError(t, channel.Delete()) })
	RefreshChannelFixedEndpointIndex()

	// matching endpoint (normalized) passes
	require.NoError(t, CheckChannelModelFixedEndpoint(channel.Id, "model-a", "https://api.allowed.example.com"))
	require.NoError(t, CheckChannelModelFixedEndpoint(channel.Id, "model-a", "https://api.allowed.example.com/"))
	// unconfigured model always passes
	require.NoError(t, CheckChannelModelFixedEndpoint(channel.Id, "model-b", "https://anything.example.com"))
	// mismatched endpoint is rejected with a readable error naming both endpoints
	err := CheckChannelModelFixedEndpoint(channel.Id, "model-a", "https://api.evil.example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model-a")
	assert.Contains(t, err.Error(), "https://api.allowed.example.com")
	assert.Contains(t, err.Error(), "https://api.evil.example.com")
	// empty channel endpoint never matches a fixed endpoint
	err = CheckChannelModelFixedEndpoint(channel.Id, "model-a", "")
	require.Error(t, err)

	// replacing with an empty map clears the restriction
	channel.ModelFixedEndpoints = &map[string]string{}
	require.NoError(t, channel.Update())
	RefreshChannelFixedEndpointIndex()
	require.NoError(t, CheckChannelModelFixedEndpoint(channel.Id, "model-a", "https://api.evil.example.com"))
}

func TestChannelModelFixedEndpointsEqual(t *testing.T) {
	norm := func(m map[string]string) *map[string]string { return &m }
	assert.True(t, ChannelModelFixedEndpointsEqual(nil, nil))
	assert.False(t, ChannelModelFixedEndpointsEqual(nil, norm(map[string]string{})))
	// normalization applies on comparison: trailing slash is insignificant
	assert.True(t, ChannelModelFixedEndpointsEqual(
		norm(map[string]string{"model-a": "https://api.example.com/"}),
		norm(map[string]string{"model-a": "https://api.example.com"}),
	))
	assert.False(t, ChannelModelFixedEndpointsEqual(
		norm(map[string]string{"model-a": "https://api.example.com"}),
		norm(map[string]string{"model-a": "https://other.example.com"}),
	))
	assert.False(t, ChannelModelFixedEndpointsEqual(
		norm(map[string]string{"model-a": "https://api.example.com", "model-b": "x"}),
		norm(map[string]string{"model-a": "https://api.example.com"}),
	))
}

func TestChannelModelFixedEndpointDeletion(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&ChannelModelFixedEndpoint{}))

	channel := Channel{
		Key:    "fixed-endpoint-delete-key",
		Name:   "fixed-endpoint-delete-channel",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
		Models: "model-a",
	}
	channel.ModelFixedEndpoints = &map[string]string{"model-a": "https://api.example.com"}
	require.NoError(t, channel.Insert())
	require.NoError(t, channel.Delete())
	RefreshChannelFixedEndpointIndex()

	var count int64
	require.NoError(t, DB.Model(&ChannelModelFixedEndpoint{}).Where("channel_id = ?", channel.Id).Count(&count).Error)
	assert.Zero(t, count, "channel delete must remove its fixed endpoint rows")
	assert.Equal(t, "", GetChannelModelFixedEndpoint(channel.Id, "model-a"))

	// deleting a channel without the table must not fail
	truncateTables(t)
	require.NoError(t, DB.Migrator().DropTable(&ChannelModelFixedEndpoint{}))
	channel2 := Channel{Key: "k", Name: "n", Status: common.ChannelStatusEnabled, Group: "default", Models: "m"}
	require.NoError(t, channel2.Insert())
	t.Cleanup(func() { require.NoError(t, channel2.Delete()) })
	require.NoError(t, channel2.Delete())
}
