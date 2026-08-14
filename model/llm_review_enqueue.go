package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
)

// draftRetryMarker parks a candidate task beyond the worker's claim window
// until the active slot binds successfully.
const draftRetryMarker int64 = 3600

// enqueueLLMReviewTask atomically creates or returns the user's active task:
//
//  1. Ensure the user's grace row exists (unique-index race safe).
//  2. Create the candidate task with next_retry_at = now + draft marker so
//     FindClaimableLLMReviewTasks cannot pick it up before the slot binds.
//  3. Atomically claim the active slot (UPDATE ... WHERE active_task_id = 0).
//  4. Slot claimed: clear the draft marker; the task becomes claimable.
//  5. Slot busy: delete the orphan candidate, read the slot's active task for
//     merging. An inconsistent slot (points at nothing) is released once and
//     retried.
//
// Returns (activeTask, created, err). created=true means this call created a
// new task.
func enqueueLLMReviewTask(userId int, task *LLMReviewTask, retry bool) (*LLMReviewTask, bool, error) {
	if userId <= 0 {
		return nil, false, errors.New("invalid user id")
	}
	if _, err := GetOrCreateLLMReviewGrace(userId); err != nil {
		return nil, false, err
	}
	now := common.GetTimestamp()

	if task.ReviewID == "" {
		rid, err := GenerateLLMReviewID()
		if err != nil {
			return nil, false, err
		}
		task.ReviewID = rid
	}
	task.UserId = userId
	task.Status = LLMReviewTaskPending
	task.NextRetryAt = now + draftRetryMarker
	if task.MaxCurrentValue < task.CurrentValue {
		task.MaxCurrentValue = task.CurrentValue
	}
	if err := DB.Create(task).Error; err != nil {
		return nil, false, err
	}

	// Atomically bind the slot.
	result := DB.Model(&LLMReviewGrace{}).
		Where("user_id = ? AND active_task_id = 0", userId).
		Updates(map[string]any{
			"active_task_id": task.ID,
			"updated_at":     now,
		})
	if result.Error != nil {
		_ = DB.Delete(&LLMReviewTask{}, task.ID).Error
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		if err := DB.Model(&LLMReviewTask{}).Where("id = ?", task.ID).
			Update("next_retry_at", 0).Error; err != nil {
			_ = ReleaseActiveReviewSlot(userId, task.ID)
			_ = DB.Delete(&LLMReviewTask{}, task.ID).Error
			return nil, false, err
		}
		return task, true, nil
	}

	// Slot busy: drop the orphan candidate and merge into the active task.
	_ = DB.Delete(&LLMReviewTask{}, task.ID).Error
	active, err := GetActiveLLMReviewTask(userId)
	if err != nil {
		return nil, false, err
	}
	if active != nil {
		return active, false, nil
	}
	// The slot points at a finished task but no active task exists.
	if !retry {
		return nil, false, errors.New("active review slot is inconsistent")
	}
	_ = ReleaseActiveReviewSlot(userId, 0)
	return enqueueLLMReviewTask(userId, task, false)
}

// EnqueueLLMReviewTask atomically creates a new active task or returns the
// existing one for merging.
func EnqueueLLMReviewTask(userId int, task *LLMReviewTask) (*LLMReviewTask, bool, error) {
	return enqueueLLMReviewTask(userId, task, true)
}
