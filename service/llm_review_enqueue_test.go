package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// openReviewEnqueueTestDB builds an isolated fixture with the review tables
// plus the minimal user table the enqueue path reads.
func openReviewEnqueueTestDB(t *testing.T) *gorm.DB {
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

func useReviewEnqueueTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	db := openReviewEnqueueTestDB(t)
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	return db
}

func createReviewEnqueueUser(t *testing.T, db *gorm.DB, username string, role int, status int) *model.User {
	t.Helper()
	user := &model.User{
		Username:    username,
		Password:    "placeholder",
		Role:        role,
		Status:      status,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func TestEnqueueLLMReviewPersistsPendingTaskForEnabledUser(t *testing.T) {
	db := useReviewEnqueueTestDB(t)
	user := createReviewEnqueueUser(t, db, "review-user", common.RoleCommonUser, common.UserStatusEnabled)
	cfg := llmReviewSettingForTest(t)
	cfg.Enabled = true
	cfg.BaseURL = "https://review.example.com"
	cfg.ModelName = "reviewer"
	cfg.PolicyText = "No sharing."
	cfg.StructuredOutputMode = operation_setting.StructuredOutputModeStrictSchema
	cfg.StructuredOutputTested = false
	cfg.SchemaTested = true

	err := EnqueueLLMReview(context.Background(), LLMReviewTrigger{
		UserId:         user.Id,
		ModelName:      "gpt-4o",
		Endpoint:       "/v1/chat/completions",
		IsStream:       true,
		TriggerType:    LLMReviewTriggerRPM,
		Stage:          LLMReviewStagePreflight,
		CurrentValue:   6,
		LimitValue:     5,
		RequestSnippet: "sanitized snippet",
		ClientIP:       "203.0.113.7",
	})
	require.NoError(t, err)

	var task model.LLMReviewTask
	require.NoError(t, db.First(&task).Error)
	assert.Equal(t, model.LLMReviewTaskPending, task.Status)
	assert.Equal(t, user.Id, task.UserId)
	assert.Equal(t, "review-user", task.Username)
	assert.Equal(t, "gpt-4o", task.ModelName)
	assert.Equal(t, 6, task.CurrentValue)
	assert.Equal(t, 5, task.LimitValue)
	assert.NotEmpty(t, task.ReviewID)
	assert.Equal(t, "203.0.***.***", task.MaskedIP, "only a partial IP mask may be persisted")
	assert.NotContains(t, task.Payload, "203.0.113.7", "the payload must never contain the raw IP")
	assert.Contains(t, task.Payload, "policy_text")
}

func TestEnqueueLLMReviewPersistsSkippedDisabledForAudit(t *testing.T) {
	db := useReviewEnqueueTestDB(t)
	user := createReviewEnqueueUser(t, db, "review-disabled", common.RoleCommonUser, common.UserStatusDisabled)
	cfg := llmReviewSettingForTest(t)
	cfg.Enabled = true

	err := EnqueueLLMReview(context.Background(), LLMReviewTrigger{
		UserId:       user.Id,
		ModelName:    "gpt-4o",
		TriggerType:  LLMReviewTriggerRPM,
		Stage:        LLMReviewStagePreflight,
		CurrentValue: 6,
		LimitValue:   5,
	})
	require.NoError(t, err)

	var task model.LLMReviewTask
	require.NoError(t, db.First(&task).Error)
	assert.Equal(t, model.LLMReviewTaskSkipped, task.Status)
	assert.Equal(t, model.SkipReasonDisabledUser, task.SkipReason)
}

func TestEnqueueLLMReviewSkipsRoot(t *testing.T) {
	db := useReviewEnqueueTestDB(t)
	root := createReviewEnqueueUser(t, db, "review-root", common.RoleRootUser, common.UserStatusEnabled)
	cfg := llmReviewSettingForTest(t)
	cfg.Enabled = true

	err := EnqueueLLMReview(context.Background(), LLMReviewTrigger{UserId: root.Id, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&model.LLMReviewTask{}).Count(&count).Error)
	assert.Zero(t, count, "root users must never create review tasks")
}

func TestEnqueueLLMReviewDisabledSettingRecordsSkipped(t *testing.T) {
	db := useReviewEnqueueTestDB(t)
	user := createReviewEnqueueUser(t, db, "review-off", common.RoleCommonUser, common.UserStatusEnabled)
	cfg := llmReviewSettingForTest(t)
	cfg.Enabled = false

	err := EnqueueLLMReview(context.Background(), LLMReviewTrigger{UserId: user.Id, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight})
	require.NoError(t, err)

	var task model.LLMReviewTask
	require.NoError(t, db.First(&task).Error)
	assert.Equal(t, model.LLMReviewTaskSkipped, task.Status)
	assert.Equal(t, model.SkipReasonReviewDisabled, task.SkipReason)
}

func TestEnqueueLLMReviewGracePeriodRecordsSkipped(t *testing.T) {
	db := useReviewEnqueueTestDB(t)
	user := createReviewEnqueueUser(t, db, "review-grace", common.RoleCommonUser, common.UserStatusEnabled)
	cfg := llmReviewSettingForTest(t)
	cfg.Enabled = true

	grace, err := model.GetOrCreateLLMReviewGrace(user.Id)
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.LLMReviewGrace{}).Where("user_id = ?", user.Id).Updates(map[string]any{
		"grace_start_at": common.GetTimestamp(),
		"grace_end_at":   common.GetTimestamp() + 3600,
	}).Error)

	err = EnqueueLLMReview(context.Background(), LLMReviewTrigger{UserId: user.Id, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight})
	require.NoError(t, err)

	var task model.LLMReviewTask
	require.NoError(t, db.First(&task).Error)
	assert.Equal(t, model.LLMReviewTaskSkipped, task.Status)
	assert.Equal(t, model.SkipReasonGracePeriod, task.SkipReason)
	_ = grace
}

func TestEnqueueLLMReviewSkipsWhenCapabilityIsUntested(t *testing.T) {
	db := useReviewEnqueueTestDB(t)
	user := createReviewEnqueueUser(t, db, "review-unready", common.RoleCommonUser, common.UserStatusEnabled)
	cfg := llmReviewSettingForTest(t)
	cfg.Enabled = true
	cfg.BaseURL = "https://review.example.com"
	cfg.ModelName = "reviewer"
	cfg.PolicyText = "No sharing."
	cfg.StructuredOutputMode = operation_setting.StructuredOutputModeStrictSchema
	cfg.SchemaTested = false
	cfg.StructuredOutputTested = false

	require.NoError(t, EnqueueLLMReview(context.Background(), LLMReviewTrigger{
		UserId:      user.Id,
		ModelName:   "gpt-4o",
		TriggerType: LLMReviewTriggerRPM,
		Stage:       LLMReviewStagePreflight,
	}))

	var task model.LLMReviewTask
	require.NoError(t, db.First(&task).Error)
	assert.Equal(t, model.LLMReviewTaskSkipped, task.Status)
	assert.Equal(t, model.SkipReasonReviewUnavailable, task.SkipReason)
	assert.Contains(t, task.FailureReason, "capability")
}

func TestEnqueueLLMReviewMergesIntoActiveTask(t *testing.T) {
	db := useReviewEnqueueTestDB(t)
	user := createReviewEnqueueUser(t, db, "review-merge", common.RoleCommonUser, common.UserStatusEnabled)
	cfg := llmReviewSettingForTest(t)
	cfg.Enabled = true
	cfg.BaseURL = "https://review.example.com"
	cfg.ModelName = "reviewer"
	cfg.PolicyText = "No sharing."
	cfg.StructuredOutputMode = operation_setting.StructuredOutputModeStrictSchema
	cfg.StructuredOutputTested = false
	cfg.SchemaTested = true

	first := LLMReviewTrigger{UserId: user.Id, ModelName: "gpt-4o", TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight, CurrentValue: 6, LimitValue: 5}
	require.NoError(t, EnqueueLLMReview(context.Background(), first))
	second := first
	second.TriggerType = LLMReviewTriggerInputToken
	second.CurrentValue = 3000
	second.LimitValue = 2000
	require.NoError(t, EnqueueLLMReview(context.Background(), second))

	var count int64
	require.NoError(t, db.Model(&model.LLMReviewTask{}).Where("user_id = ? AND status IN ?", user.Id, model.LLMReviewActiveStatuses).Count(&count).Error)
	assert.Equal(t, int64(1), count, "only one active task per user")

	var task model.LLMReviewTask
	require.NoError(t, db.First(&task).Error)
	assert.Equal(t, 1, task.MergeCount)
	assert.True(t, task.TriggerRPM)
	assert.True(t, task.TriggerInputToken)
	assert.Equal(t, 3000, task.MaxCurrentValue)
}
