package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

// reviewSettingControllerForTest returns live settings with restoration.
func reviewSettingControllerForTest(t *testing.T) *operation_setting.LLMReviewSetting {
	t.Helper()
	cfg := operation_setting.GetLLMReviewSetting()
	original := *cfg
	t.Cleanup(func() { *cfg = original })
	return cfg
}

func newLLMReviewGinContext(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	if strings.HasPrefix(path, "/api/llm_review/tasks/") {
		c.Params = gin.Params{{Key: "id", Value: "1"}}
	}
	return c, w
}

func TestGetLLMReviewConfigMasksAPIKey(t *testing.T) {
	originalSecret := common.CryptoSecret
	common.CryptoSecret = "llm-review-controller-test"
	t.Cleanup(func() { common.CryptoSecret = originalSecret })

	cfg := reviewSettingControllerForTest(t)
	cfg.BaseURL = "https://review.example.com"
	cfg.ModelName = "reviewer"
	cfg.PolicyText = "No sharing."
	enc, err := service.EncryptLLMReviewAPIKey("sk-abcdefghijklmn")
	require.NoError(t, err)
	cfg.APIKeyEncrypted = enc

	c, w := newLLMReviewGinContext(t, http.MethodGet, "/api/llm_review/config", "")
	GetLLMReviewConfig(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			APIKey     string `json:"api_key"`
			PolicyText string `json:"policy_text"`
			BaseURL    string `json:"base_url"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, "sk-****klmn", body.Data.APIKey, "the config API must return only a tail-derived mask")
	assert.NotContains(t, body.Data.APIKey, "abcdefghijklmn")
	assert.Equal(t, "No sharing.", body.Data.PolicyText)
}

func TestUpdateLLMReviewConfigRejectsEnableWithoutSchemaTest(t *testing.T) {
	cfg := reviewSettingControllerForTest(t)
	cfg.BaseURL = "https://review.example.com"
	cfg.ModelName = "reviewer"
	cfg.PolicyText = "No sharing."
	cfg.SchemaTested = false

	c, w := newLLMReviewGinContext(t, http.MethodPut, "/api/llm_review/config", `{"enabled":true}`)
	UpdateLLMReviewConfig(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "capability test")
	assert.False(t, cfg.Enabled, "the runtime switch must stay off")
}

func TestUpdateLLMReviewConfigRejectsEnableWithoutConfig(t *testing.T) {
	cfg := reviewSettingControllerForTest(t)
	cfg.BaseURL = ""
	cfg.ModelName = ""

	c, w := newLLMReviewGinContext(t, http.MethodPut, "/api/llm_review/config", `{"enabled":true}`)
	UpdateLLMReviewConfig(c)

	assert.Contains(t, w.Body.String(), "not fully configured")
}

func TestApplyReviewCandidateInvalidatesCapabilityAfterModelChange(t *testing.T) {
	cfg := &operation_setting.LLMReviewSetting{
		BaseURL:                     "https://review.example.com",
		ModelName:                   "old-model",
		StructuredOutputMode:        operation_setting.StructuredOutputModeJSONObject,
		StructuredOutputTested:      true,
		StructuredOutputTestedAt:    123,
		StructuredOutputTestedModel: "old-model",
		StructuredOutputVersion:     "prompt-v2",
	}

	candidate, err := applyReviewCandidate(cfg, reviewCandidateRequest{ModelName: "new-model"})
	require.NoError(t, err)
	assert.Equal(t, "new-model", candidate.ModelName)
	assert.False(t, candidate.StructuredOutputTested)
	assert.Zero(t, candidate.StructuredOutputTestedAt)
	assert.Empty(t, candidate.StructuredOutputTestedModel)
	assert.Empty(t, candidate.StructuredOutputVersion)
}

func TestLLMReviewSchemaRejectsConcurrentProbe(t *testing.T) {
	require.True(t, llmReviewSchemaTestMu.TryLock())
	t.Cleanup(func() { llmReviewSchemaTestMu.Unlock() })

	c, w := newLLMReviewGinContext(t, http.MethodPost, "/api/llm_review/test_schema", `{}`)
	TestLLMReviewSchema(c)

	assert.Contains(t, w.Body.String(), "already in progress")
}

func TestUpdateLLMReviewConfigRejectsWhileProbeIsInProgress(t *testing.T) {
	require.True(t, llmReviewSchemaTestMu.TryLock())
	t.Cleanup(func() { llmReviewSchemaTestMu.Unlock() })

	c, w := newLLMReviewGinContext(t, http.MethodPut, "/api/llm_review/config", `{"base_url":"https://new.example.com"}`)
	UpdateLLMReviewConfig(c)

	assert.Contains(t, w.Body.String(), "already in progress")
}

func TestApplyReviewCandidateInvalidatesCapabilityAfterModeChange(t *testing.T) {
	mode := operation_setting.StructuredOutputModePromptJSON
	cfg := &operation_setting.LLMReviewSetting{
		BaseURL:                     "https://review.example.com",
		ModelName:                   "reviewer",
		StructuredOutputMode:        operation_setting.StructuredOutputModeJSONObject,
		StructuredOutputTested:      true,
		StructuredOutputTestedAt:    123,
		StructuredOutputTestedModel: "reviewer",
		StructuredOutputVersion:     "prompt-v2",
	}

	candidate, err := applyReviewCandidate(cfg, reviewCandidateRequest{StructuredOutputMode: &mode})
	require.NoError(t, err)
	assert.Equal(t, mode, candidate.StructuredOutputMode)
	assert.False(t, candidate.StructuredOutputTested)
	assert.Zero(t, candidate.StructuredOutputTestedAt)
	assert.Empty(t, candidate.StructuredOutputTestedModel)
	assert.Empty(t, candidate.StructuredOutputVersion)
}

func TestUpdateLLMReviewConfigInvalidatesSchemaOnCriticalChange(t *testing.T) {
	db := useLLMReviewControllerTestDB(t)

	cfg := reviewSettingControllerForTest(t)
	cfg.BaseURL = "https://old.example.com"
	cfg.ModelName = "reviewer"
	cfg.SchemaTested = true

	c, w := newLLMReviewGinContext(t, http.MethodPut, "/api/llm_review/config", `{"base_url":"https://new.example.com"}`)
	UpdateLLMReviewConfig(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, strings.Contains(w.Body.String(), `"success":true`) || strings.Contains(w.Body.String(), "success"), "persisted update must succeed")

	var schemaOption model.Option
	require.NoError(t, db.Where("`key` = ?", "llm_review_setting.schema_tested").First(&schemaOption).Error)
	assert.Equal(t, "false", schemaOption.Value, "critical config changes must reset the capability state")
	var baseURLOption model.Option
	require.NoError(t, db.Where("`key` = ?", "llm_review_setting.base_url").First(&baseURLOption).Error)
	assert.Equal(t, "https://new.example.com", baseURLOption.Value)
}

func TestGetLLMReviewTaskDetailDoesNotRequireProof(t *testing.T) {
	useLLMReviewControllerTestDB(t)

	tests := []struct {
		name  string
		proof string
	}{
		{name: "missing proof"},
		{name: "arbitrary proof header", proof: "not-a-security-proof"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := newLLMReviewGinContext(t, http.MethodGet, "/api/llm_review/tasks/1", "")
			if tt.proof != "" {
				c.Request.Header.Set("X-Security-Proof", tt.proof)
			}

			GetLLMReviewTaskDetail(c)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "task not found")
			assert.NotContains(t, w.Body.String(), "SECURITY_PROOF")
		})
	}
}

func TestRetryLLMReviewTaskDoesNotRequireProof(t *testing.T) {
	useLLMReviewControllerTestDB(t)

	for _, proof := range []string{"", "not-a-security-proof"} {
		c, w := newLLMReviewGinContext(t, http.MethodPost, "/api/llm_review/tasks/1/retry", "")
		if proof != "" {
			c.Request.Header.Set("X-Security-Proof", proof)
		}

		RetryLLMReviewTask(c)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "SECURITY_PROOF")
	}
}

func TestListLLMReviewTasksReturnsRows(t *testing.T) {
	db := useLLMReviewControllerTestDB(t)

	require.NoError(t, db.Create(&model.LLMReviewTask{
		UserId: 1, Username: "alice", ModelName: "gpt-4o", Status: model.LLMReviewTaskPending,
		ReviewID: "rev-test-1",
	}).Error)

	c, w := newLLMReviewGinContext(t, http.MethodGet, "/api/llm_review/tasks?page=1&page_size=20", "")
	ListLLMReviewTasks(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, 1, body.Data.Total)
}

func useLLMReviewControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	db := openLLMReviewControllerTestDB(t)
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
	})
	return db
}

// openLLMReviewControllerTestDB builds an isolated fixture with the tables the
// review controller reads/writes.
func openLLMReviewControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	originalRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.OptionMap = originalOptionMap
		common.RedisEnabled = originalRedis
	})

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(
		&model.Option{},
		&model.LLMReviewTask{},
		&model.LLMReviewAttempt{},
		&model.LLMReviewGrace{},
		&model.LLMReviewCalibration{},
		&model.User{},
		&model.Log{},
	))
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
