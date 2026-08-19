package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service/vision"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupVisionInterceptTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
}

func runVisionMiddleware(t *testing.T, setting dto.UserSetting, requestPath string, body string) *gin.Context {
	t.Helper()
	return runVisionMiddlewareWithInterceptor(t, setting, requestPath, body, vision.InterceptImages)
}

func runVisionMiddlewareWithInterceptor(t *testing.T, setting dto.UserSetting, requestPath string, body string, interceptImages visionImageInterceptor) *gin.Context {
	t.Helper()
	setupVisionInterceptTest(t)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, requestPath, bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 42)
	c.Set(string(constant.ContextKeyUserSetting), setting)
	c.Set(common.RequestIdKey, "vision-test-request")

	visionIntercept(interceptImages)(c)
	return c
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
	c := runVisionMiddleware(t, dto.UserSetting{}, "/v1/chat/completions", body)

	out, err := readBodyViaStorage(c)
	require.NoError(t, err)
	assert.Equal(t, body, string(out))
	_, intercepted := c.Get("vision_intercepted")
	assert.False(t, intercepted)
}

func TestVisionInterceptRequiresSuffix(t *testing.T) {
	body := `{"model":"gpt-4o-vision","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`
	setting := dto.UserSetting{Vision: &dto.UserVisionSetting{Enabled: true, VisionModel: "qwen-vl", VisionSuffix: ""}}
	c := runVisionMiddleware(t, setting, "/v1/chat/completions", body)

	out, err := readBodyViaStorage(c)
	require.NoError(t, err)
	assert.Equal(t, body, string(out))
	_, intercepted := c.Get("vision_intercepted")
	assert.False(t, intercepted)
}

func TestVisionInterceptSuffixMismatchSkips(t *testing.T) {
	body := `{"model":"gpt-4o","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`
	setting := dto.UserSetting{Vision: &dto.UserVisionSetting{Enabled: true, VisionModel: "qwen-vl", VisionSuffix: "-vision"}}
	c := runVisionMiddleware(t, setting, "/v1/chat/completions", body)

	out, err := readBodyViaStorage(c)
	require.NoError(t, err)
	assert.Equal(t, body, string(out))
	_, intercepted := c.Get("vision_intercepted")
	assert.False(t, intercepted)
}

func TestVisionInterceptIgnoresUnrelatedPaths(t *testing.T) {
	body := `{"model":"gpt-4o-vision"}`
	setting := dto.UserSetting{Vision: &dto.UserVisionSetting{Enabled: true, VisionModel: "qwen-vl", VisionSuffix: "-vision"}}
	c := runVisionMiddleware(t, setting, "/v1/embeddings", body)

	out, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	assert.Equal(t, body, string(out))
	_, intercepted := c.Get("vision_intercepted")
	assert.False(t, intercepted)
}

func TestVisionInterceptResponsesRewritesBodyAndPreservesModel(t *testing.T) {
	interceptImages := func(_ *gin.Context, _ map[string]any, entries []vision.ImageEntry, _ dto.UserVisionSetting, _ string) error {
		require.Len(t, entries, 1)
		assert.Equal(t, "https://img.example/cat.png", entries[0].URL)
		entries[0].Replace("A black cat sitting on a windowsill.")
		return nil
	}

	body := `{"model":"gpt-4o-vision","input":[{"role":"user","content":[{"type":"input_text","text":"What is shown?"},{"type":"input_image","image_url":"https://img.example/cat.png","detail":"high"}]}]}`
	setting := dto.UserSetting{Vision: &dto.UserVisionSetting{
		Enabled:        true,
		VisionModel:    "qwen-vl",
		VisionSuffix:   "-vision",
		PromptTemplate: "Describe visible evidence.",
		PhashThreshold: 0,
	}}
	c := runVisionMiddlewareWithInterceptor(t, setting, "/v1/responses", body, interceptImages)

	requestBody, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	cachedBody, exists := c.Get(common.KeyRequestBody)
	require.True(t, exists)
	assert.Equal(t, requestBody, cachedBody)
	assert.Equal(t, int64(len(requestBody)), c.Request.ContentLength)
	assert.Equal(t, true, c.GetBool("vision_intercepted"))

	var rewritten map[string]any
	require.NoError(t, common.Unmarshal(requestBody, &rewritten))
	assert.Equal(t, "gpt-4o-vision", rewritten["model"])
	input := rewritten["input"].([]any)
	content := input[0].(map[string]any)["content"].([]any)
	imagePart := content[1].(map[string]any)
	assert.Equal(t, "input_text", imagePart["type"])
	assert.Equal(t, "A black cat sitting on a windowsill.", imagePart["text"])
	assert.NotContains(t, imagePart, "image_url")
	assert.NotContains(t, imagePart, "detail")
}

func TestVisionInterceptFailOpenWhenExtractionFails(t *testing.T) {
	content := make([]any, 0, vision.MaxImagesPerRequest+1)
	for i := 0; i < vision.MaxImagesPerRequest+1; i++ {
		content = append(content, map[string]any{
			"type":      "input_image",
			"image_url": "https://img.example/image.png",
		})
	}
	request := map[string]any{
		"model": "gpt-4o-vision",
		"input": []any{map[string]any{"role": "user", "content": content}},
	}
	bodyBytes, err := common.Marshal(request)
	require.NoError(t, err)
	body := string(bodyBytes)
	setting := dto.UserSetting{Vision: &dto.UserVisionSetting{Enabled: true, VisionModel: "qwen-vl", VisionSuffix: "-vision"}}

	c := runVisionMiddleware(t, setting, "/v1/responses", body)

	out, err := readBodyViaStorage(c)
	require.NoError(t, err)
	assert.Equal(t, body, string(out))
	_, intercepted := c.Get("vision_intercepted")
	assert.False(t, intercepted)
}

func TestVisionInterceptFailOpenWhenAnalysisFails(t *testing.T) {
	interceptImages := func(_ *gin.Context, _ map[string]any, entries []vision.ImageEntry, _ dto.UserVisionSetting, _ string) error {
		require.Len(t, entries, 1)
		entries[0].Replace("partial description that must not reach downstream")
		return errors.New("vision provider unavailable")
	}

	body := `{"model":"gpt-4o-vision","input":[{"role":"user","content":[{"type":"input_image","image_url":"https://img.example/cat.png"}]}]}`
	setting := dto.UserSetting{Vision: &dto.UserVisionSetting{Enabled: true, VisionModel: "qwen-vl", VisionSuffix: "-vision", PromptTemplate: "describe"}}

	c := runVisionMiddlewareWithInterceptor(t, setting, "/v1/responses", body, interceptImages)

	out, err := readBodyViaStorage(c)
	require.NoError(t, err)
	assert.Equal(t, body, string(out))
	_, intercepted := c.Get("vision_intercepted")
	assert.False(t, intercepted)
}
