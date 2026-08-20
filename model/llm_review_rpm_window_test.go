package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyLLMReviewGraceForRPMMigration struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserId    int   `gorm:"uniqueIndex"`
	UpdatedAt int64 `gorm:"bigint"`
}

func (legacyLLMReviewGraceForRPMMigration) TableName() string {
	return "llm_review_graces"
}

type legacyLLMReviewTaskForRPMMigration struct {
	ID        int64               `gorm:"primaryKey;autoIncrement"`
	ReviewID  string              `gorm:"type:varchar(64);uniqueIndex"`
	UserId    int                 `gorm:"index"`
	Status    LLMReviewTaskStatus `gorm:"type:varchar(32);index"`
	CreatedAt int64               `gorm:"bigint;index"`
	UpdatedAt int64               `gorm:"bigint"`
}

func (legacyLLMReviewTaskForRPMMigration) TableName() string {
	return "llm_review_tasks"
}

func TestLLMReviewRPMWindowMigrationDefaultsExistingRowsToZero(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyLLMReviewGraceForRPMMigration{}, &legacyLLMReviewTaskForRPMMigration{}))
	require.NoError(t, db.Create(&legacyLLMReviewGraceForRPMMigration{UserId: 17, UpdatedAt: 1}).Error)
	require.NoError(t, db.Create(&legacyLLMReviewTaskForRPMMigration{ReviewID: "legacy", UserId: 17, Status: LLMReviewTaskCompliant, CreatedAt: 1, UpdatedAt: 1}).Error)

	require.NoError(t, db.AutoMigrate(&LLMReviewGrace{}, &LLMReviewTask{}))

	var grace LLMReviewGrace
	require.NoError(t, db.Where("user_id = ?", 17).First(&grace).Error)
	assert.Zero(t, grace.RPMReviewWindowStartAt)
	assert.Zero(t, grace.RPMReviewWindowEndAt)
	assert.Zero(t, grace.RPMReviewTaskId)
	assert.Zero(t, grace.LastCompliantRPMWindowStartAt)

	var task LLMReviewTask
	require.NoError(t, db.Where("review_id = ?", "legacy").First(&task).Error)
	assert.Zero(t, task.TriggerWindowStartAt)
	assert.Zero(t, task.TriggerWindowEndAt)
}

func TestRecordLLMReviewCompliantForRPMWindowCountsOnce(t *testing.T) {
	truncateTables(t)
	cfg := reviewSettingForTest(t)
	cfg.MaxCompliantCount = 3

	now := common.GetTimestamp()
	require.NoError(t, RecordLLMReviewCompliantForRPMWindow(12, 700, now))
	require.NoError(t, RecordLLMReviewCompliantForRPMWindow(12, 700, now+1))

	grace, err := GetLLMReviewGrace(12)
	require.NoError(t, err)
	require.NotNil(t, grace)
	assert.Equal(t, 1, grace.CompliantCount)
	assert.Equal(t, int64(700), grace.LastCompliantRPMWindowStartAt)
}

func TestEnqueueLLMReviewTaskForRPMWindowKeepsMarkerAfterCompletion(t *testing.T) {
	truncateTables(t)
	const (
		windowStart = int64(100)
		windowEnd   = int64(200)
		now         = int64(150)
	)

	first := &LLMReviewTask{
		UserId:       7,
		ModelName:    "gpt-4o",
		TriggerType:  LLMReviewTriggerRPM,
		Stage:        LLMReviewStagePreflight,
		CurrentValue: 6,
		LimitValue:   5,
		Payload:      `{"request_body":"first"}`,
	}
	selected, claimed, created, err := EnqueueLLMReviewTaskForRPMWindow(7, first, windowStart, windowEnd, now, "")
	require.NoError(t, err)
	require.True(t, claimed)
	require.True(t, created)
	require.Equal(t, first.ID, selected.ID)

	_, ok, err := ClaimLLMReviewTask(first.ID)
	require.NoError(t, err)
	require.True(t, ok)
	first.Status = LLMReviewTaskCompliant
	first.Verdict = LLMReviewVerdictCompliant
	require.NoError(t, CompleteLLMReviewTask(first, nil))

	duplicate := &LLMReviewTask{
		UserId:       7,
		TriggerType:  LLMReviewTriggerRPM,
		Stage:        LLMReviewStagePreflight,
		CurrentValue: 7,
		LimitValue:   5,
		Payload:      `{"request_body":"duplicate"}`,
	}
	selected, claimed, created, err = EnqueueLLMReviewTaskForRPMWindow(7, duplicate, windowStart, windowEnd, 180, "")
	require.NoError(t, err)
	assert.False(t, claimed)
	assert.False(t, created)
	assert.Nil(t, selected)

	grace, err := GetLLMReviewGrace(7)
	require.NoError(t, err)
	require.NotNil(t, grace)
	assert.Equal(t, windowStart, grace.RPMReviewWindowStartAt)
	assert.Equal(t, windowEnd, grace.RPMReviewWindowEndAt)
	assert.Equal(t, first.ID, grace.RPMReviewTaskId)

	next := &LLMReviewTask{
		UserId:       7,
		TriggerType:  LLMReviewTriggerRPM,
		Stage:        LLMReviewStagePreflight,
		CurrentValue: 8,
		LimitValue:   5,
		Payload:      `{"request_body":"next"}`,
	}
	selected, claimed, created, err = EnqueueLLMReviewTaskForRPMWindow(7, next, windowEnd, windowEnd+100, windowEnd, "")
	require.NoError(t, err)
	assert.True(t, claimed)
	assert.True(t, created)
	assert.Equal(t, next.ID, selected.ID)
}

func TestEnqueueLLMReviewTaskForRPMWindowDelayedDuplicateStaysIdempotent(t *testing.T) {
	truncateTables(t)
	const (
		windowStart = int64(100)
		windowEnd   = int64(200)
	)

	first := &LLMReviewTask{UserId: 13, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight}
	_, selected, created, err := EnqueueLLMReviewTaskForRPMWindow(13, first, windowStart, windowEnd, 150, "")
	require.NoError(t, err)
	require.True(t, selected)
	require.True(t, created)

	// Async enqueue may be delayed until after the limiter window expires. The
	// same window identity must remain a duplicate instead of reclaiming itself.
	duplicate := &LLMReviewTask{UserId: 13, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight}
	_, selected, created, err = EnqueueLLMReviewTaskForRPMWindow(13, duplicate, windowStart, windowEnd, windowEnd+30, "")
	require.NoError(t, err)
	assert.False(t, selected)
	assert.False(t, created)

	var count int64
	require.NoError(t, DB.Model(&LLMReviewTask{}).Where("user_id = ?", 13).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestEnqueueLLMReviewTaskForRPMWindowConcurrentSingleWinner(t *testing.T) {
	truncateTables(t)
	const workers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	selected := 0
	errors := make([]error, 0)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task := &LLMReviewTask{UserId: 9, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight}
			_, won, made, err := EnqueueLLMReviewTaskForRPMWindow(9, task, 500, 560, 520, "")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, err)
				return
			}
			if won {
				selected++
			}
			if made {
				created++
			}
		}()
	}
	wg.Wait()

	require.Empty(t, errors)
	assert.Equal(t, 1, selected)
	assert.Equal(t, 1, created)
	var count int64
	require.NoError(t, DB.Model(&LLMReviewTask{}).Where("user_id = ?", 9).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestEnqueueLLMReviewTaskForRPMWindowKeepsFirstSkippedAuditWithActiveTask(t *testing.T) {
	truncateTables(t)
	active := &LLMReviewTask{UserId: 10, TriggerType: LLMReviewTriggerInputToken, Stage: LLMReviewStagePreflight}
	require.NoError(t, DB.Create(active).Error)
	_, err := GetOrCreateLLMReviewGrace(10)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&LLMReviewGrace{}).Where("user_id = ?", 10).Update("active_task_id", active.ID).Error)

	skipped := &LLMReviewTask{UserId: 10, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight}
	selected, claimed, created, err := EnqueueLLMReviewTaskForRPMWindow(10, skipped, 300, 360, 320, SkipReasonReviewUnavailable)
	require.NoError(t, err)
	assert.True(t, claimed)
	assert.True(t, created)
	assert.Equal(t, skipped.ID, selected.ID)
	assert.Equal(t, SkipReasonReviewUnavailable, selected.SkipReason)

	var count int64
	require.NoError(t, DB.Model(&LLMReviewTask{}).Where("user_id = ?", 10).Count(&count).Error)
	assert.Equal(t, int64(2), count)
	grace, err := GetLLMReviewGrace(10)
	require.NoError(t, err)
	assert.Equal(t, skipped.ID, grace.RPMReviewTaskId)
	assert.Equal(t, active.ID, grace.ActiveTaskId)
}

func TestEnqueueLLMReviewTaskForRPMWindowRepairsTerminalActiveSlot(t *testing.T) {
	truncateTables(t)
	stale := &LLMReviewTask{UserId: 11, Status: LLMReviewTaskCompliant, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight}
	require.NoError(t, DB.Create(stale).Error)
	_, err := GetOrCreateLLMReviewGrace(11)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&LLMReviewGrace{}).Where("user_id = ?", 11).Update("active_task_id", stale.ID).Error)

	next := &LLMReviewTask{UserId: 11, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight}
	selected, claimed, created, err := EnqueueLLMReviewTaskForRPMWindow(11, next, 400, 460, 420, "")
	require.NoError(t, err)
	assert.True(t, claimed)
	assert.True(t, created)
	assert.Equal(t, next.ID, selected.ID)

	grace, err := GetLLMReviewGrace(11)
	require.NoError(t, err)
	assert.Equal(t, next.ID, grace.ActiveTaskId)
	assert.Equal(t, next.ID, grace.RPMReviewTaskId)
}

func TestEnqueueLLMReviewTaskForRPMWindowRecordsOneSkippedAudit(t *testing.T) {
	truncateTables(t)
	first := &LLMReviewTask{UserId: 8, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight}
	_, selected, created, err := EnqueueLLMReviewTaskForRPMWindow(8, first, 300, 360, 320, SkipReasonReviewUnavailable)
	require.NoError(t, err)
	assert.True(t, selected)
	assert.True(t, created)

	second := &LLMReviewTask{UserId: 8, TriggerType: LLMReviewTriggerRPM, Stage: LLMReviewStagePreflight}
	_, selected, created, err = EnqueueLLMReviewTaskForRPMWindow(8, second, 300, 360, 340, SkipReasonReviewUnavailable)
	require.NoError(t, err)
	assert.False(t, selected)
	assert.False(t, created)

	var count int64
	require.NoError(t, DB.Model(&LLMReviewTask{}).Where("user_id = ?", 8).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
