package common

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureLLMReviewRequestContextRedactsBodyHeadersAndPreservesStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"password":"do-not-store","image":{"data":"data:image/png;base64,` + strings.Repeat("A", 256) + `"}}`
	storage, err := CreateBodyStorage([]byte(body))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.Close()) })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer top-secret")
	c.Request.Header.Set("Cookie", "session=secret")
	c.Request.Header.Set("X-Trace", "ordinary")
	c.Set(KeyBodyStorage, storage)

	captured := CaptureLLMReviewRequestContext(c)
	assert.Contains(t, captured.Summary, "hello")
	assert.Contains(t, captured.Body, "gpt-4o")
	assert.NotContains(t, captured.Body, "do-not-store")
	assert.NotContains(t, captured.Body, "data:image/png;base64")
	assert.Equal(t, []string{"***"}, captured.Headers["Authorization"])
	assert.Equal(t, []string{"***"}, captured.Headers["Cookie"])
	assert.Equal(t, []string{"ordinary"}, captured.Headers["X-Trace"])
	assert.LessOrEqual(t, len(captured.Body), reviewRequestBodyMaxBytes+32)

	stored, err := storage.Bytes()
	require.NoError(t, err)
	assert.Equal(t, body, string(stored))
}

func TestCaptureLLMReviewRequestContextParsesLargeJSONAndPreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"gpt-4o","messages":[{"role":"system","content":"` + strings.Repeat("prefix ", 10000) + `"},{"role":"user","content":"message after the old 64 KiB prefix"}],"password":"do-not-store"}`
	require.Greater(t, len(body), 64<<10)
	storage, err := CreateBodyStorage([]byte(body))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.Close()) })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(KeyBodyStorage, storage)

	captured := CaptureLLMReviewRequestContext(c)
	assert.Contains(t, captured.Summary, "message after the old 64 KiB prefix")
	assert.Contains(t, captured.Body, "message after the old 64 KiB prefix")
	assert.NotContains(t, captured.Body, "do-not-store")
	assert.NotContains(t, captured.Body, "[json body omitted: invalid or truncated]")
	assert.LessOrEqual(t, len(captured.Body), reviewRequestBodyMaxBytes)

	reader, err := storage.NewReader()
	require.NoError(t, err)
	replayed, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	assert.Equal(t, body, string(replayed))
}

func TestCaptureLLMReviewRequestContextBoundsOversizedJSONWithoutTreatingItAsInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"` + strings.Repeat("x", reviewRequestBodyReadBytes) + `"}],"password":"do-not-store"}`
	require.Greater(t, len(body), reviewRequestBodyReadBytes)
	var decoded any
	require.NoError(t, Unmarshal([]byte(body), &decoded))

	storage, err := CreateBodyStorage([]byte(body))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.Close()) })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(KeyBodyStorage, storage)

	captured := CaptureLLMReviewRequestContext(c)
	assert.Empty(t, captured.Summary)
	assert.Equal(t, "[json body omitted: exceeds review capture limit]", captured.Body)
	assert.NotContains(t, captured.Body, "do-not-store")
	assert.LessOrEqual(t, len(captured.Body), reviewRequestBodyMaxBytes)

	replayed, err := storage.Bytes()
	require.NoError(t, err)
	assert.Equal(t, body, string(replayed))
}

func TestCaptureLLMReviewRequestContextRedactsSnakeCaseCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"access_token":"access-secret","refresh_token":"refresh-secret","private_key":"private-secret","session_token":"session-secret","api_key":"api-secret"}`
	storage, err := CreateBodyStorage([]byte(body))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.Close()) })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/test", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(KeyBodyStorage, storage)

	captured := CaptureLLMReviewRequestContext(c)
	for _, secret := range []string{"access-secret", "refresh-secret", "private-secret", "session-secret", "api-secret"} {
		assert.NotContains(t, captured.Body, secret)
	}
	assert.Contains(t, captured.Body, `"access_token":"***"`)
	assert.Contains(t, captured.Body, `"private_key":"***"`)
}

func TestCaptureLLMReviewRequestContextOmitsOpaqueMediaText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opaque := strings.Repeat("A", 256)
	body := `{"messages":[{"role":"user","content":"` + opaque + `"}]}`
	storage, err := CreateBodyStorage([]byte(body))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.Close()) })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(KeyBodyStorage, storage)

	captured := CaptureLLMReviewRequestContext(c)
	assert.NotContains(t, captured.Summary, opaque)
	assert.NotContains(t, captured.Body, opaque)
}

func TestCaptureLLMReviewRequestContextPrintsAndBoundsOtherBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.Repeat("\x00\x01binary-secret\n", 2000)
	storage, err := CreateBodyStorage([]byte(body))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.Close()) })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/test", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/octet-stream")
	c.Set(KeyBodyStorage, storage)

	captured := CaptureLLMReviewRequestContext(c)
	assert.LessOrEqual(t, len(captured.Body), reviewRequestBodyMaxBytes)
	assert.NotContains(t, captured.Body, "\x00")
	assert.NotContains(t, captured.Body, "\x01")
}

func TestCaptureLLMReviewRequestContextMultipartOmitsFileBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte("--boundary\r\nContent-Disposition: form-data; name=prompt\r\n\r\nhello\r\n--boundary\r\nContent-Disposition: form-data; name=file; filename=secret.txt\r\nContent-Type: text/plain\r\n\r\nsecret-file-bytes\r\n--boundary--\r\n")
	storage, err := CreateBodyStorage(body)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.Close()) })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/upload", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	c.Set(KeyBodyStorage, storage)

	captured := CaptureLLMReviewRequestContext(c)
	assert.Empty(t, captured.Summary)
	assert.Contains(t, captured.Body, "hello")
	assert.Contains(t, captured.Body, "filename")
	assert.NotContains(t, captured.Body, "secret-file-bytes")
}
