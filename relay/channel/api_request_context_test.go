package channel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type contextPropagationAdaptor struct {
	Adaptor
	url string
}

func (a *contextPropagationAdaptor) GetRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.url, nil
}

func (a *contextPropagationAdaptor) SetupRequestHeader(_ *gin.Context, _ *http.Header, _ *relaycommon.RelayInfo) error {
	return nil
}

func TestDoApiRequestPropagatesGinRequestCancellationUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestStarted := make(chan struct{})
	releaseServer := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(releaseServer) })
	}
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseServer
	}))
	t.Cleanup(func() {
		release()
		server.CloseClientConnections()
		server.Close()
	})

	parentContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(parentContext)

	result := make(chan error, 1)
	go func() {
		_, err := DoApiRequest(
			&contextPropagationAdaptor{url: server.URL},
			ginContext,
			&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelSetting: dto.ChannelSettings{}}},
			strings.NewReader(`{}`),
		)
		result <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream request did not start")
	}
	cancel()

	select {
	case err := <-result:
		require.Error(t, err)
		require.ErrorIs(t, parentContext.Err(), context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("DoApiRequest did not return after cancellation")
	}
	release()
}
