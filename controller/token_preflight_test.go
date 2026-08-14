package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

func TestIsTextTokenLimitMode(t *testing.T) {
	assert.True(t, relayconstant.IsTextTokenLimitMode(relayconstant.RelayModeChatCompletions))
	assert.True(t, relayconstant.IsTextTokenLimitMode(relayconstant.RelayModeCompletions))
	assert.True(t, relayconstant.IsTextTokenLimitMode(relayconstant.RelayModeResponses))
	assert.True(t, relayconstant.IsTextTokenLimitMode(relayconstant.RelayModeResponsesCompact))

	assert.False(t, relayconstant.IsTextTokenLimitMode(relayconstant.RelayModeEmbeddings))
	assert.False(t, relayconstant.IsTextTokenLimitMode(relayconstant.RelayModeImagesGenerations))
	assert.False(t, relayconstant.IsTextTokenLimitMode(relayconstant.RelayModeAudioTranscription))
	assert.False(t, relayconstant.IsTextTokenLimitMode(relayconstant.RelayModeUnknown))
	assert.False(t, relayconstant.IsTextTokenLimitMode(relayconstant.RelayModeRealtime))
}

func preflightBanSettingForTest(t *testing.T) *operation_setting.RateLimitBanSetting {
	t.Helper()
	cfg := operation_setting.GetRateLimitBanSetting()
	original := *cfg
	t.Cleanup(func() { *cfg = original })
	return cfg
}

func newPreflightRelayInfo(model string, userId int) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: model,
		UserId:          userId,
		RelayMode:       relayconstant.RelayModeChatCompletions,
	}
}

func TestCheckInputTokenPreflightDisabledSettingFailsOpen(t *testing.T) {
	db := useLLMReviewControllerTestDB(t)
	user := createReviewControllerUser(t, db, "preflight-off", common.RoleCommonUser, common.UserStatusEnabled)
	cfg := preflightBanSettingForTest(t)
	cfg.Enabled = false
	cfg.MaxInputTokens = 200000

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))

	assert.False(t, checkInputTokenPreflight(c, newPreflightRelayInfo("gpt-4o", user.Id), 300000))
	assert.NotEqual(t, http.StatusTooManyRequests, w.Code)
}

func TestCheckInputTokenPreflightFailsOpenWithoutCalibration(t *testing.T) {
	db := useLLMReviewControllerTestDB(t)
	user := createReviewControllerUser(t, db, "preflight-uncalibrated", common.RoleCommonUser, common.UserStatusEnabled)
	cfg := preflightBanSettingForTest(t)
	cfg.Enabled = true
	cfg.MaxInputTokens = 200000

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))

	assert.False(t, checkInputTokenPreflight(c, newPreflightRelayInfo("uncalibrated-model", user.Id), 300000))
	assert.NotEqual(t, http.StatusTooManyRequests, w.Code)
}

func TestCheckInputTokenPreflightRespectsWhitelist(t *testing.T) {
	db := useLLMReviewControllerTestDB(t)
	user := createReviewControllerUser(t, db, "preflight-whitelist", common.RoleCommonUser, common.UserStatusEnabled)
	cfg := preflightBanSettingForTest(t)
	cfg.Enabled = true
	cfg.MaxInputTokens = 200000
	cfg.WhitelistModels = []string{"whitelisted-*"}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))

	assert.False(t, checkInputTokenPreflight(c, newPreflightRelayInfo("whitelisted-flash", user.Id), 300000))
	assert.NotEqual(t, http.StatusTooManyRequests, w.Code)
}

func TestCheckInputTokenPreflightExemptsRoot(t *testing.T) {
	db := useLLMReviewControllerTestDB(t)
	root := createReviewControllerUser(t, db, "preflight-root", common.RoleRootUser, common.UserStatusEnabled)
	cfg := preflightBanSettingForTest(t)
	cfg.Enabled = true
	cfg.MaxInputTokens = 200000

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))

	assert.False(t, checkInputTokenPreflight(c, newPreflightRelayInfo("gpt-4o", root.Id), 300000))
	assert.NotEqual(t, http.StatusTooManyRequests, w.Code)
}

// TestCheckInputTokenPreflightBlocksAcceptedEstimator seeds a calibrated model
// (>=1000 samples, all within tolerance, no near-threshold false rejects),
// then verifies an over-limit estimate produces the OpenAI-compatible 429 and
// asynchronously enqueues a preflight review task.
func TestCheckInputTokenPreflightBlocksAcceptedEstimator(t *testing.T) {
	db := useLLMReviewControllerTestDB(t)
	user := createReviewControllerUser(t, db, "preflight-block", common.RoleCommonUser, common.UserStatusEnabled)
	cfg := preflightBanSettingForTest(t)
	cfg.Enabled = true
	cfg.MaxInputTokens = 200000
	cfg.WhitelistModels = nil
	// The review pipeline must be switched on for the async trigger to land
	// as a pending task instead of an audit-only skipped record.
	reviewCfg := reviewSettingControllerForTest(t)
	reviewCfg.Enabled = true
	reviewCfg.PolicyText = "No sharing."

	// Bulk-seed 1000 acceptance samples for the model.
	samples := make([]*model.LLMReviewCalibration, 0, 1000)
	for i := 0; i < 1000; i++ {
		samples = append(samples, &model.LLMReviewCalibration{
			ModelName:        "calibrated-model",
			EstimateTokens:   1000,
			ActualTokens:     1000,
			RelativeError:    0,
			EstimatorVersion: "estimator-v1",
			SampleTime:       common.GetTimestamp(),
			ModelPassed:      true,
		})
	}
	require.NoError(t, db.CreateInBatches(samples, 200).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))

	blocked := checkInputTokenPreflight(c, newPreflightRelayInfo("calibrated-model", user.Id), 300000)
	assert.True(t, blocked)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
	assert.Contains(t, w.Body.String(), "rate_limit_exceeded")

	// The review trigger is enqueued asynchronously and must land as a
	// pending input_token/preflight task.
	require.Eventually(t, func() bool {
		var count int64
		_ = db.Model(&model.LLMReviewTask{}).
			Where("user_id = ? AND trigger_type = ? AND stage = ? AND status = ?",
				user.Id, model.LLMReviewTriggerInputToken, model.LLMReviewStagePreflight, model.LLMReviewTaskPending).
			Count(&count).Error
		return count == 1
	}, 5*time.Second, 50*time.Millisecond, "preflight review task must be enqueued")
}

// createReviewControllerUser seeds a user row in the controller test fixture.
func createReviewControllerUser(t *testing.T, db *gorm.DB, username string, role int, status int) *model.User {
	t.Helper()
	user := &model.User{
		Username: username, Password: "placeholder", Role: role, Status: status,
		Group: "default", AuthVersion: 1,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}
