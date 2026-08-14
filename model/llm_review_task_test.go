package model

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertLLMReviewTask(t *testing.T, task *LLMReviewTask) *LLMReviewTask {
	t.Helper()
	if task.ReviewID == "" {
		rid, err := GenerateLLMReviewID()
		require.NoError(t, err)
		task.ReviewID = rid
	}
	if task.Status == "" {
		task.Status = LLMReviewTaskPending
	}
	require.NoError(t, DB.Create(task).Error)
	return task
}

func TestGenerateLLMReviewIDIsUnguessable(t *testing.T) {
	truncateTables(t)
	id1, err := GenerateLLMReviewID()
	require.NoError(t, err)
	id2, err := GenerateLLMReviewID()
	require.NoError(t, err)
	assert.True(t, len(id1) >= 28, "review ids must carry substantial entropy")
	assert.True(t, id1[:4] == "rev-")
	assert.NotEqual(t, id1, id2)
}

func TestCreateLLMReviewTaskAssignsPendingAndReviewID(t *testing.T) {
	truncateTables(t)
	task := insertLLMReviewTask(t, &LLMReviewTask{UserId: 7, ModelName: "gpt-4o"})
	assert.Equal(t, LLMReviewTaskPending, task.Status)
	assert.NotEmpty(t, task.ReviewID)
	assert.NotZero(t, task.CreatedAt)
}

// TestClaimLLMReviewTaskSingleWinner guards the multi-instance claim CAS:
// concurrent claimants of one pending task must produce exactly one winner.
func TestClaimLLMReviewTaskSingleWinner(t *testing.T) {
	truncateTables(t)
	task := insertLLMReviewTask(t, &LLMReviewTask{UserId: 7})

	const workers = 8
	var mu sync.Mutex
	winners := 0
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := ClaimLLMReviewTask(task.ID)
			require.NoError(t, err)
			if ok {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, winners, "exactly one claim must win")

	var stored LLMReviewTask
	require.NoError(t, DB.First(&stored, task.ID).Error)
	assert.Equal(t, LLMReviewTaskReviewing, stored.Status)
}

func TestMarkLLMReviewTaskRetrySchedulesFixedInterval(t *testing.T) {
	truncateTables(t)
	task := insertLLMReviewTask(t, &LLMReviewTask{UserId: 7})
	_, ok, err := ClaimLLMReviewTask(task.ID)
	require.NoError(t, err)
	require.True(t, ok)

	before := common.GetTimestamp()
	require.NoError(t, MarkLLMReviewTaskRetry(task.ID, 1, 30))

	var stored LLMReviewTask
	require.NoError(t, DB.First(&stored, task.ID).Error)
	assert.Equal(t, LLMReviewTaskPending, stored.Status)
	assert.Equal(t, 1, stored.Attempts)
	assert.InDelta(t, before+30, stored.NextRetryAt, 2)
}

func TestRecoverStaleLLMReviewTasksResetsStuckReviewing(t *testing.T) {
	truncateTables(t)
	task := insertLLMReviewTask(t, &LLMReviewTask{UserId: 7})
	_, ok, err := ClaimLLMReviewTask(task.ID)
	require.NoError(t, err)
	require.True(t, ok)

	// Backdate started_at beyond the stale threshold.
	require.NoError(t, DB.Model(&LLMReviewTask{}).Where("id = ?", task.ID).
		Update("started_at", common.GetTimestamp()-int64((20*time.Minute).Seconds())).Error)
	require.NoError(t, RecoverStaleLLMReviewTasks(10*time.Minute))

	var stored LLMReviewTask
	require.NoError(t, DB.First(&stored, task.ID).Error)
	assert.Equal(t, LLMReviewTaskPending, stored.Status)
	assert.Zero(t, stored.StartedAt)
}

func TestMergeLLMReviewTaskAccumulatesAtomically(t *testing.T) {
	truncateTables(t)
	task := insertLLMReviewTask(t, &LLMReviewTask{UserId: 7, CurrentValue: 10, MaxCurrentValue: 10})

	require.NoError(t, MergeLLMReviewTask(task, LLMReviewTriggerRPM, 12, 100))
	require.NoError(t, MergeLLMReviewTask(task, LLMReviewTriggerInputToken, 3000, 2000))
	require.NoError(t, MergeLLMReviewTask(task, LLMReviewTriggerRPM, 8, 100))

	var stored LLMReviewTask
	require.NoError(t, DB.First(&stored, task.ID).Error)
	assert.Equal(t, 3, stored.MergeCount)
	assert.True(t, stored.TriggerRPM)
	assert.True(t, stored.TriggerInputToken)
	assert.False(t, stored.TriggerOutputToken)
	assert.Equal(t, 3000, stored.MaxCurrentValue, "max current value must keep the largest trigger")
	assert.Equal(t, "rpm,input_token", stored.TriggerTypesValue())
	assert.Equal(t, 100, stored.LimitValue)
}

func TestMergeLLMReviewTaskRejectsInactiveTarget(t *testing.T) {
	truncateTables(t)
	task := insertLLMReviewTask(t, &LLMReviewTask{UserId: 7})
	require.NoError(t, MarkLLMReviewTaskSkipped(task, SkipReasonGracePeriod))

	err := MergeLLMReviewTask(task, LLMReviewTriggerRPM, 1, 100)
	assert.ErrorIs(t, err, ErrLLMReviewTaskNoLongerActive)
}

func TestCompleteAndFailReleaseActiveSlot(t *testing.T) {
	truncateTables(t)
	task := insertLLMReviewTask(t, &LLMReviewTask{UserId: 7})
	_, ok, err := ClaimLLMReviewTask(task.ID)
	require.NoError(t, err)
	require.True(t, ok)
	// The enqueue path owns slot creation; simulate the bound slot directly.
	_, err = GetOrCreateLLMReviewGrace(7)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&LLMReviewGrace{}).Where("user_id = ?", 7).
		Update("active_task_id", task.ID).Error)

	task.Status = LLMReviewTaskCompliant
	task.Verdict = LLMReviewVerdictCompliant
	require.NoError(t, CompleteLLMReviewTask(task, nil))

	grace, err := GetLLMReviewGrace(7)
	require.NoError(t, err)
	require.NotNil(t, grace)
	assert.Zero(t, grace.ActiveTaskId, "completion must release the active slot")

	// Failure path also releases.
	task2 := insertLLMReviewTask(t, &LLMReviewTask{UserId: 8})
	_, ok, err = ClaimLLMReviewTask(task2.ID)
	require.NoError(t, err)
	require.True(t, ok)
	_, err = GetOrCreateLLMReviewGrace(8)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&LLMReviewGrace{}).Where("user_id = ?", 8).
		Update("active_task_id", task2.ID).Error)
	require.NoError(t, FailLLMReviewTask(task2.ID, "exhausted"))
	grace, err = GetLLMReviewGrace(8)
	require.NoError(t, err)
	assert.Zero(t, grace.ActiveTaskId)
}

func TestRetryLLMReviewTaskRules(t *testing.T) {
	truncateTables(t)
	task := insertLLMReviewTask(t, &LLMReviewTask{UserId: 7})
	require.NoError(t, MarkLLMReviewTaskSkipped(task, SkipReasonGracePeriod))

	// skipped is not retryable.
	err := RetryLLMReviewTask(task.ID)
	require.Error(t, err)

	failed := insertLLMReviewTask(t, &LLMReviewTask{UserId: 9})
	_, ok, err := ClaimLLMReviewTask(failed.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, FailLLMReviewTask(failed.ID, "exhausted"))

	// A concurrent active task for the same user blocks the retry.
	other := insertLLMReviewTask(t, &LLMReviewTask{UserId: 9})
	require.NoError(t, DB.Model(&LLMReviewGrace{}).Where("user_id = ?", 9).
		Update("active_task_id", other.ID).Error)
	err = RetryLLMReviewTask(failed.ID)
	require.Error(t, err)

	// Without a conflicting active task the retry re-queues and claims the slot.
	require.NoError(t, MarkLLMReviewTaskSkipped(other, SkipReasonGracePeriod))
	require.NoError(t, DB.Model(&LLMReviewGrace{}).Where("user_id = ?", 9).
		Update("active_task_id", 0).Error)
	require.NoError(t, RetryLLMReviewTask(failed.ID))
	var stored LLMReviewTask
	require.NoError(t, DB.First(&stored, failed.ID).Error)
	assert.Equal(t, LLMReviewTaskPending, stored.Status)
	assert.Zero(t, stored.Attempts)
	grace, err := GetLLMReviewGrace(9)
	require.NoError(t, err)
	assert.Equal(t, failed.ID, grace.ActiveTaskId)
}

func TestCleanupLLMReviewOldRecordsKeepsViolationAndBanned(t *testing.T) {
	truncateTables(t)
	old := common.GetTimestamp() - 200*86400

	compliant := insertLLMReviewTask(t, &LLMReviewTask{UserId: 1, Status: LLMReviewTaskCompliant, Verdict: LLMReviewVerdictCompliant})
	require.NoError(t, DB.Model(&LLMReviewTask{}).Where("id = ?", compliant.ID).Update("created_at", old).Error)

	violation := insertLLMReviewTask(t, &LLMReviewTask{UserId: 2, Status: LLMReviewTaskViolation, Verdict: LLMReviewVerdictViolation})
	require.NoError(t, DB.Model(&LLMReviewTask{}).Where("id = ?", violation.ID).Update("created_at", old).Error)

	banned := insertLLMReviewTask(t, &LLMReviewTask{UserId: 3, Status: LLMReviewTaskViolation, Verdict: LLMReviewVerdictViolation, Banned: true})
	require.NoError(t, DB.Model(&LLMReviewTask{}).Where("id = ?", banned.ID).Update("created_at", old).Error)

	removed, err := CleanupLLMReviewOldRecords(common.GetTimestamp(), 90)
	require.NoError(t, err)
	assert.Equal(t, int64(1), removed)

	var count int64
	require.NoError(t, DB.Model(&LLMReviewTask{}).Where("id = ?", violation.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, DB.Model(&LLMReviewTask{}).Where("id = ?", banned.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestGetLLMReviewQueueSummary(t *testing.T) {
	truncateTables(t)
	insertLLMReviewTask(t, &LLMReviewTask{UserId: 1})
	insertLLMReviewTask(t, &LLMReviewTask{UserId: 2})
	claimed := insertLLMReviewTask(t, &LLMReviewTask{UserId: 3})
	_, ok, err := ClaimLLMReviewTask(claimed.ID)
	require.NoError(t, err)
	require.True(t, ok)

	summary, err := GetLLMReviewQueueSummary()
	require.NoError(t, err)
	assert.Equal(t, int64(2), summary.Pending)
	assert.Equal(t, int64(1), summary.Reviewing)
}

func TestListLLMReviewTasksFilters(t *testing.T) {
	truncateTables(t)
	insertLLMReviewTask(t, &LLMReviewTask{UserId: 1, Username: "alice", ModelName: "gpt-4o", TriggerType: LLMReviewTriggerRPM})
	insertLLMReviewTask(t, &LLMReviewTask{UserId: 2, Username: "bob", ModelName: "claude-3", TriggerType: LLMReviewTriggerInputToken})

	tasks, total, err := ListLLMReviewTasks(1, 20, "", 0, "", "", "", "alice", "", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, tasks, 1)
	assert.Equal(t, "alice", tasks[0].Username)

	tasks, total, err = ListLLMReviewTasks(1, 20, "", 0, "claude-3", "", "", "", "", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "claude-3", tasks[0].ModelName)
}

func TestRecordLLMReviewCompliantOpensGraceWindow(t *testing.T) {
	truncateTables(t)
	cfg := reviewSettingForTest(t)
	cfg.MaxCompliantCount = 3
	cfg.GracePeriodHours = 5

	now := common.GetTimestamp()
	for i := 0; i < 3; i++ {
		require.NoError(t, RecordLLMReviewCompliant(7, now))
	}
	inGrace, reason, err := CheckLLMReviewGrace(7, now)
	require.NoError(t, err)
	assert.True(t, inGrace)
	assert.Equal(t, SkipReasonGracePeriod, reason)
}

func TestCheckLLMReviewGraceResetsExpiredWindow(t *testing.T) {
	truncateTables(t)
	grace, err := GetOrCreateLLMReviewGrace(7)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&LLMReviewGrace{}).Where("user_id = ?", 7).Updates(map[string]any{
		"grace_start_at": 100,
		"grace_end_at":   200,
	}).Error)

	inGrace, _, err := CheckLLMReviewGrace(7, 300)
	require.NoError(t, err)
	assert.False(t, inGrace)

	stored, err := GetLLMReviewGrace(7)
	require.NoError(t, err)
	assert.Zero(t, stored.GraceEndAt, "expired windows must be reset")
	_ = grace
}

func TestRecordManualBanAndUnbanTimestamps(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	require.NoError(t, RecordLLMReviewManualBan(7, now))
	require.NoError(t, RecordLLMReviewManualUnban(7, now+10))

	stored, err := GetLLMReviewGrace(7)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, now, stored.LastManualBanAt)
	assert.Equal(t, now+10, stored.LastManualUnbanAt)
}

// reviewSettingForTest returns the live review settings with automatic
// restoration, so grace-window configuration can be exercised without
// touching the database option layer.
func reviewSettingForTest(t *testing.T) *operation_setting.LLMReviewSetting {
	t.Helper()
	cfg := operation_setting.GetLLMReviewSetting()
	original := *cfg
	t.Cleanup(func() { *cfg = original })
	return cfg
}
