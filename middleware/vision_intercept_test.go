package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupVisionInterceptTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
}

// runVisionMiddleware executes the middleware with the given setting and
// request JSON, returning the rewritten body and the intercept flag.
func runVisionMiddleware(t *testing.T, setting dto.UserSetting, modelPath string, body string) (string, bool) {
	t.Helper()
	setupVisionInterceptTest(t)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, modelPath, bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 42)
	c.Set(string(constant.ContextKeyUserSetting), setting)
	c.Set(common.RequestIdKey, "vision-test-request")

	called := false
	mw := VisionIntercept()
	mw(c)
	// The middleware only calls c.Next(); simulate the handler contract.
	// gin.CreateTestContext has no handler chain, so we read the body after
	// middleware execution directly.
	_ = called

	raw, err := readBodyViaStorage(c)
	require.NoError(t, err)
	_, intercepted := c.Get("vision_intercepted")
	return string(raw), intercepted
}

func readBodyViaStorage(c *gin.Context) ([]byte, error) {
	seeker, err := common.GetRequestBody(c)
	if err != nil {
		return nil, err
	}
	return readSeekerAll(seeker)
}

func TestVisionInterceptDisabledByDefault(t *testing.T) {
	body := `{"model":"gpt-4o-vision","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`
	setting := dto.UserSetting{}
	out, intercepted := runVisionMiddleware(t, setting, "/v1/chat/completions", body)
	assert.Equal(t, body, out)
	assert.False(t, intercepted)
}

func TestVisionInterceptRequiresSuffix(t *testing.T) {
	body := `{"model":"gpt-4o-vision","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`
	setting := dto.UserSetting{Vision: &dto.UserVisionSetting{Enabled: true, VisionModel: "qwen-vl", VisionSuffix: ""}}
	out, intercepted := runVisionMiddleware(t, setting, "/v1/chat/completions", body)
	assert.Equal(t, body, out)
	assert.False(t, intercepted)
}

func TestVisionInterceptSuffixMismatchSkips(t *testing.T) {
	body := `{"model":"gpt-4o","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`
	setting := dto.UserSetting{Vision: &dto.UserVisionSetting{Enabled: true, VisionModel: "qwen-vl", VisionSuffix: "-vision"}}
	out, intercepted := runVisionMiddleware(t, setting, "/v1/chat/completions", body)
	assert.Equal(t, body, out)
	assert.False(t, intercepted)
}

func TestVisionInterceptIgnoresUnrelatedPaths(t *testing.T) {
	body := `{"model":"gpt-4o-vision"}`
	setting := dto.UserSetting{Vision: &dto.UserVisionSetting{Enabled: true, VisionModel: "qwen-vl", VisionSuffix: "-vision"}}
	out, intercepted := runVisionMiddleware(t, setting, "/v1/embeddings", body)
	assert.Equal(t, body, out)
	assert.False(t, intercepted)
}

func TestVisionInterceptFailOpenWhenAnalysisUnavailable(t *testing.T) {
	// An undownloadable private-IP image makes analysis fail; the request
	// must continue with the original body untouched (fail-open), never a
	// 500 or a partial rewrite.
	body := `{"model":"gpt-4o-vision","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"http://127.0.0.1/x.png"}}]}]}`
	setting := dto.UserSetting{Vision: &dto.UserVisionSetting{Enabled: true, VisionModel: "qwen-vl", VisionSuffix: "-vision", PromptTemplate: "describe"}}
	out, intercepted := runVisionMiddleware(t, setting, "/v1/chat/completions", body)
	assert.Equal(t, body, out)
	assert.False(t, intercepted)
}
