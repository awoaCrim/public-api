package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/opencodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAdaptorReturnsOpenCodeGoAdaptor(t *testing.T) {
	adaptor := GetAdaptor(constant.APITypeOpenCodeGo)

	require.IsType(t, &opencodego.Adaptor{}, adaptor)
	assert.Equal(t, "OpenCode Go", adaptor.GetChannelName())
}
