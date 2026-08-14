package service

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteRateLimitErrorIsOpenAICompatible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	WriteRateLimitError(context, RateLimitExceededMessage, 60, 5, 0)

	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	assert.Equal(t, "60", recorder.Header().Get("Retry-After"))
	assert.Equal(t, "5", recorder.Header().Get("X-RateLimit-Limit-Requests"))
	assert.Equal(t, "0", recorder.Header().Get("X-RateLimit-Remaining-Requests"))
	_, err := strconv.ParseInt(recorder.Header().Get("X-RateLimit-Reset-Requests"), 10, 64)
	require.NoError(t, err)

	var body struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Param   any    `json:"param"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, RateLimitExceededMessage, body.Error.Message)
	assert.Equal(t, "rate_limit_error", body.Error.Type)
	assert.Equal(t, "rate_limit_exceeded", body.Error.Code)
	assert.Nil(t, body.Error.Param)
}
