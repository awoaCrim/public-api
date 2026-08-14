package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newEnqueueCandidate(userId int, trigger LLMReviewTriggerType, current int) *LLMReviewTask {
	return &LLMReviewTask{
		UserId:          userId,
		ModelName:       "gpt-4o",
		Endpoint:        "/v1/chat/completions",
		TriggerType:     trigger,
		Stage:           LLMReviewStagePreflight,
		CurrentValue:    current,
		MaxCurrentValue: current,
		Payload:         `{"request_snippet":"sanitized"}`,
	}
}

func TestEnqueueLLMReviewTaskCreatesAndBindsClaimableTask(t *testing.T) {
	truncateTables(t)
	task, created, err := EnqueueLLMReviewTask(7, newEnqueueCandidate(7, LLMReviewTriggerRPM, 12))
	require.NoError(t, err)
	require.True(t, created)
	assert.NotEmpty(t, task.ReviewID)
	assert.Equal(t, LLMReviewTaskPending, task.Status)

	// The slot is bound and the draft marker cleared, so the worker sees it.
	grace, err := GetLLMReviewGrace(7)
	require.NoError(t, err)
	assert.Equal(t, task.ID, grace.ActiveTaskId)

	claimable, err := FindClaimableLLMReviewTasks(10)
	require.NoError(t, err)
	require.Len(t, claimable, 1)
	assert.Equal(t, task.ID, claimable[0].ID)
	assert.Zero(t, claimable[0].NextRetryAt, "draft marker must be cleared after slot binding")
}

func TestEnqueueLLMReviewTaskSecondEnqueueReturnsActiveTask(t *testing.T) {
	truncateTables(t)
	first, created, err := EnqueueLLMReviewTask(7, newEnqueueCandidate(7, LLMReviewTriggerRPM, 12))
	require.NoError(t, err)
	require.True(t, created)

	second, created, err := EnqueueLLMReviewTask(7, newEnqueueCandidate(7, LLMReviewTriggerInputToken, 3000))
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, first.ID, second.ID, "the second enqueue must return the existing active task")

	var count int64
	require.NoError(t, DB.Model(&LLMReviewTask{}).
		Where("user_id = ? AND status IN ?", 7, LLMReviewActiveStatuses).Count(&count).Error)
	assert.Equal(t, int64(1), count, "at most one active task per user")
}

// TestEnqueueLLMReviewTaskConcurrentSingleCreator guards the cross-instance
// atomicity: N concurrent enqueues for one user produce exactly one new task
// and the rest merge into it.
func TestEnqueueLLMReviewTaskConcurrentSingleCreator(t *testing.T) {
	truncateTables(t)
	const workers = 8
	createdIDs := make([]int64, 0, workers)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, created, err := EnqueueLLMReviewTask(7, newEnqueueCandidate(7, LLMReviewTriggerRPM, 12))
			require.NoError(t, err)
			mu.Lock()
			if created {
				createdIDs = append(createdIDs, task.ID)
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	require.Len(t, createdIDs, 1, "exactly one concurrent enqueue creates the task")
	grace, err := GetLLMReviewGrace(7)
	require.NoError(t, err)
	assert.Equal(t, createdIDs[0], grace.ActiveTaskId)
}

func TestEnqueueLLMReviewTaskRecoversInconsistentSlot(t *testing.T) {
	truncateTables(t)
	// Point the slot at a task id that does not exist.
	_, err := GetOrCreateLLMReviewGrace(7)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&LLMReviewGrace{}).Where("user_id = ?", 7).
		Update("active_task_id", int64(999999)).Error)

	task, created, err := EnqueueLLMReviewTask(7, newEnqueueCandidate(7, LLMReviewTriggerRPM, 12))
	require.NoError(t, err)
	require.True(t, created)

	grace, err := GetLLMReviewGrace(7)
	require.NoError(t, err)
	assert.Equal(t, task.ID, grace.ActiveTaskId, "the inconsistent slot must be recovered and rebound")
}
