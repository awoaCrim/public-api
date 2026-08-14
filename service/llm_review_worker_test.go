package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// reviewVerdictBody renders a valid strict-schema verdict response body.
func reviewVerdictBody(verdict, category string, confidence float64) string {
	inner := fmt.Sprintf(`{"verdict":"%s","category":"%s","confidence":%v,"reason":"r","evidence":["e"]}`, verdict, category, confidence)
	contentJSON, _ := common.Marshal(inner)
	return fmt.Sprintf(`{"choices":[{"message":{"content":%s}}]}`, string(contentJSON))
}

// newReviewWorkerServer starts a local reviewer stub. With the private switch
// on, the review client may dial the loopback test server.
func newReviewWorkerServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *operation_setting.LLMReviewSetting) {
	t.Helper()
	originalSecret := common.CryptoSecret
	common.CryptoSecret = "llm-review-worker-test"
	t.Cleanup(func() { common.CryptoSecret = originalSecret })

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := llmReviewSettingForTest(t)
	cfg.Enabled = true
	cfg.BaseURL = server.URL
	cfg.ModelName = "reviewer"
	cfg.APIKeyEncrypted = ""
	cfg.TimeoutSeconds = 5
	cfg.AllowPrivateAddress = true
	cfg.SchemaTested = true
	cfg.PolicyText = "Do not share accounts."
	cfg.MaxAttempts = 1
	cfg.RetryIntervalSeconds = 1
	return server, cfg
}

// setupReviewTaskEnv prepares the DB fixture for worker tests.
func setupReviewTaskEnv(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	db := openReviewWorkerTestDB(t)
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalDB
	})
}

func TestProcessLLMReviewTaskCompliantVerdict(t *testing.T) {
	setupReviewTaskEnv(t)
	_, cfg := newReviewWorkerServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(reviewVerdictBody("compliant", "none", 0.9)))
	})
	cfg.PolicyText = "No sharing."

	task := &model.LLMReviewTask{
		UserId:  7,
		Status:  model.LLMReviewTaskReviewing,
		Payload: `{"request_snippet":"x","policy_text":"No sharing."}`,
	}
	require.NoError(t, model.DB.Create(task).Error)

	processLLMReviewTask(task)

	var stored model.LLMReviewTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.LLMReviewTaskCompliant, stored.Status)
	assert.Equal(t, model.LLMReviewVerdictCompliant, stored.Verdict)
	assert.True(t, stored.SchemaPassed)

	var attempts int64
	require.NoError(t, model.DB.Model(&model.LLMReviewAttempt{}).Where("task_id = ?", task.ID).Count(&attempts).Error)
	assert.Equal(t, int64(1), attempts)

	grace, err := model.GetLLMReviewGrace(7)
	require.NoError(t, err)
	require.NotNil(t, grace)
	assert.Equal(t, 1, grace.CompliantCount, "compliant verdicts must increment the grace counter")
}

func TestProcessLLMReviewTaskViolationWithoutAutoBan(t *testing.T) {
	setupReviewTaskEnv(t)
	_, cfg := newReviewWorkerServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(reviewVerdictBody("violation", "code_generation", 0.99)))
	})
	cfg.PolicyText = "No sharing."

	task := &model.LLMReviewTask{
		UserId:  7,
		Status:  model.LLMReviewTaskReviewing,
		Payload: `{"request_snippet":"x","policy_text":"No sharing."}`,
	}
	require.NoError(t, model.DB.Create(task).Error)

	processLLMReviewTask(task)

	var stored model.LLMReviewTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.LLMReviewTaskViolation, stored.Status)
	assert.False(t, stored.Banned, "content-semantic categories must never auto-ban")
}

func TestProcessLLMReviewTaskAutoBanInvokesSeam(t *testing.T) {
	setupReviewTaskEnv(t)
	_, cfg := newReviewWorkerServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(reviewVerdictBody("violation", "account_sharing", 0.97)))
	})
	cfg.PolicyText = "No sharing."

	originalBan := reviewAutoBanUser
	var bannedUserId int
	var banMessage string
	reviewAutoBanUser = func(task *model.LLMReviewTask, message string) error {
		bannedUserId = task.UserId
		banMessage = message
		return nil
	}
	t.Cleanup(func() { reviewAutoBanUser = originalBan })

	task := &model.LLMReviewTask{
		UserId:  7,
		Status:  model.LLMReviewTaskReviewing,
		Payload: `{"request_snippet":"x","policy_text":"No sharing."}`,
	}
	require.NoError(t, model.DB.Create(task).Error)

	processLLMReviewTask(task)

	assert.Equal(t, 7, bannedUserId)
	assert.Contains(t, banMessage, task.ReviewID, "the ban message must reference the review number")

	var stored model.LLMReviewTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.True(t, stored.Banned)
	assert.Equal(t, model.LLMReviewTaskViolation, stored.Status)
}

func TestProcessLLMReviewTaskRetriesThenFails(t *testing.T) {
	setupReviewTaskEnv(t)
	calls := 0
	_, cfg := newReviewWorkerServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})
	cfg.PolicyText = "No sharing."
	cfg.MaxAttempts = 3

	task := &model.LLMReviewTask{
		UserId:  7,
		Status:  model.LLMReviewTaskReviewing,
		Payload: `{"request_snippet":"x","policy_text":"No sharing."}`,
	}
	require.NoError(t, model.DB.Create(task).Error)

	processLLMReviewTask(task)

	// First attempt fails and schedules a retry; the task returns to pending.
	var stored model.LLMReviewTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.LLMReviewTaskPending, stored.Status)
	assert.Equal(t, 1, stored.Attempts)
	assert.Greater(t, stored.NextRetryAt, common.GetTimestamp()-5)

	// Claim and process again with attempts=1.
	claimed, ok, err := model.ClaimLLMReviewTask(task.ID)
	require.NoError(t, err)
	require.True(t, ok)
	processLLMReviewTask(claimed)

	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.LLMReviewTaskPending, stored.Status)
	assert.Equal(t, 2, stored.Attempts)

	// Final attempt exhausts the retry budget.
	claimed, ok, err = model.ClaimLLMReviewTask(task.ID)
	require.NoError(t, err)
	require.True(t, ok)
	processLLMReviewTask(claimed)

	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.LLMReviewTaskFailed, stored.Status)
	assert.Equal(t, 3, calls, "each processing pass must make exactly one call before the budget is exhausted")
}

func TestProcessLLMReviewTaskManualOverrideSupersedes(t *testing.T) {
	setupReviewTaskEnv(t)
	task := &model.LLMReviewTask{
		UserId:    7,
		Status:    model.LLMReviewTaskReviewing,
		CreatedAt: common.GetTimestamp() - 100,
		Payload:   `{}`,
	}
	require.NoError(t, model.DB.Create(task).Error)
	_, err := model.GetOrCreateLLMReviewGrace(7)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.LLMReviewGrace{}).Where("user_id = ?", 7).
		Update("last_manual_ban_at", common.GetTimestamp()).Error)

	processLLMReviewTask(task)

	var stored model.LLMReviewTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.LLMReviewTaskSuperseded, stored.Status)
	assert.Equal(t, model.SkipReasonManualBan, stored.SupersededBy)
}

func TestProcessLLMReviewTaskWithoutPolicyCompletesUncertain(t *testing.T) {
	setupReviewTaskEnv(t)
	cfg := llmReviewSettingForTest(t)
	cfg.Enabled = true
	cfg.PolicyText = ""

	task := &model.LLMReviewTask{
		UserId:  7,
		Status:  model.LLMReviewTaskReviewing,
		Payload: `{"request_snippet":"x"}`,
	}
	require.NoError(t, model.DB.Create(task).Error)

	processLLMReviewTask(task)

	var stored model.LLMReviewTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.LLMReviewTaskUncertain, stored.Status)
	assert.False(t, stored.SchemaPassed)
	assert.Equal(t, "missing policy", stored.SchemaError)
}

func TestCheckManualOverrideSemantics(t *testing.T) {
	setupReviewTaskEnv(t)
	now := common.GetTimestamp()
	_, err := model.GetOrCreateLLMReviewGrace(7)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.LLMReviewGrace{}).Where("user_id = ?", 7).Updates(map[string]any{
		"last_manual_ban_at":   now,
		"last_manual_unban_at": 0,
	}).Error)

	assert.Equal(t, model.SkipReasonManualBan, checkManualOverride(&model.LLMReviewTask{UserId: 7, CreatedAt: now - 10}))
	assert.Empty(t, checkManualOverride(&model.LLMReviewTask{UserId: 7, CreatedAt: now + 10}))

	require.NoError(t, model.DB.Model(&model.LLMReviewGrace{}).Where("user_id = ?", 7).Updates(map[string]any{
		"last_manual_ban_at":   0,
		"last_manual_unban_at": now,
	}).Error)
	assert.Equal(t, model.SkipReasonManualUnban, checkManualOverride(&model.LLMReviewTask{UserId: 7, CreatedAt: now - 10}))
}

func TestIsRetryableLLMReviewCall(t *testing.T) {
	assert.True(t, isRetryableLLMReviewCall(LLMReviewCallResult{Error: context.DeadlineExceeded}))
	assert.True(t, isRetryableLLMReviewCall(LLMReviewCallResult{Error: assert.AnError, HTTPStatus: 429}))
	assert.True(t, isRetryableLLMReviewCall(LLMReviewCallResult{Error: assert.AnError, HTTPStatus: 502}))
	assert.False(t, isRetryableLLMReviewCall(LLMReviewCallResult{Error: assert.AnError, HTTPStatus: 401}))
	assert.False(t, isRetryableLLMReviewCall(LLMReviewCallResult{}))
}

// openReviewWorkerTestDB builds an isolated in-memory SQLite fixture so worker
// tests never share state with the model package TestMain database.
func openReviewWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(
		&model.LLMReviewTask{},
		&model.LLMReviewAttempt{},
		&model.LLMReviewGrace{},
		&model.LLMReviewCalibration{},
		&model.User{},
	))
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
