package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
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

// EnqueueLLMReviewTaskForRPMWindow selects at most one event in a live RPM
// limiter window. The selection marker is independent from ActiveTaskId and is
// committed together with the selected task or active-task merge.
//
// selected=false is a successful silent duplicate. Such a result must not be
// passed to MergeLLMReviewTask by callers because repeated 429s are not merge
// events. created reports whether a new pending task was persisted.
func EnqueueLLMReviewTaskForRPMWindow(userId int, task *LLMReviewTask, windowStart, windowEnd, now int64, skipReason string) (selectedTask *LLMReviewTask, selected, created bool, err error) {
	if userId <= 0 || task == nil {
		return nil, false, false, errors.New("invalid RPM review window task")
	}
	if windowStart <= 0 || windowEnd <= windowStart || now <= 0 {
		return nil, false, false, errors.New("invalid RPM review window")
	}
	if _, err := GetOrCreateLLMReviewGrace(userId); err != nil {
		return nil, false, false, err
	}

	for attempt := 0; attempt < 2; attempt++ {
		selectedTask = nil
		selected = false
		created = false
		err = DB.Transaction(func(tx *gorm.DB) error {
			// Claim the window first with a dialect-neutral conditional UPDATE.
			// This is the SQLite correctness boundary; row locks are only an
			// optimization on databases that support FOR UPDATE. The explicit
			// identity guard keeps a delayed duplicate idempotent even when the
			// asynchronous enqueue runs after that limiter window has expired.
			claim := tx.Model(&LLMReviewGrace{}).
				Where(
					"user_id = ? AND (rpm_review_window_end_at IS NULL OR rpm_review_window_end_at = 0 OR rpm_review_window_end_at <= ?) AND (rpm_review_window_start_at IS NULL OR rpm_review_window_end_at IS NULL OR rpm_review_window_start_at != ? OR rpm_review_window_end_at != ?)",
					userId, now, windowStart, windowEnd,
				).
				Updates(map[string]any{
					"rpm_review_window_start_at": windowStart,
					"rpm_review_window_end_at":   windowEnd,
					"rpm_review_task_id":         0,
					"updated_at":                 now,
				})
			if claim.Error != nil {
				return claim.Error
			}
			if claim.RowsAffected != 1 {
				return nil
			}

			// A skipped first event still needs its own audit row. It must not
			// disappear merely because an unrelated token task occupies the
			// active slot.
			if skipReason != "" {
				task.UserId = userId
				task.TriggerWindowStartAt = windowStart
				task.TriggerWindowEndAt = windowEnd
				task.Status = LLMReviewTaskSkipped
				task.SkipReason = skipReason
				task.CompletedAt = now
				task.UpdatedAt = now
				if task.ReviewID == "" {
					rid, err := GenerateLLMReviewID()
					if err != nil {
						return err
					}
					task.ReviewID = rid
				}
				if task.MaxCurrentValue < task.CurrentValue {
					task.MaxCurrentValue = task.CurrentValue
				}
				if err := tx.Create(task).Error; err != nil {
					return err
				}
				if err := bindRPMReviewWindowTask(tx, userId, windowStart, windowEnd, task.ID, now); err != nil {
					return err
				}
				selectedTask = task
				selected = true
				created = true
				return nil
			}

			active, err := getActiveLLMReviewTask(tx, userId, true)
			if err != nil {
				return err
			}
			if active != nil {
				if err := bindRPMActiveReviewSlot(tx, userId, active.ID); err != nil {
					return err
				}
				if err := mergeLLMReviewTaskTx(tx, active, task.TriggerType, task.CurrentValue, task.LimitValue, now); err != nil {
					return err
				}
				if err := bindRPMReviewWindowTask(tx, userId, windowStart, windowEnd, active.ID, now); err != nil {
					return err
				}
				// Preserve the first RPM window represented by a task. This is
				// needed when the active task was originally a token-triggered
				// task so its compliant result remains tied to the RPM episode.
				if active.TriggerWindowStartAt == 0 {
					result := tx.Model(&LLMReviewTask{}).
						Where("id = ? AND status IN ? AND trigger_window_start_at = 0", active.ID, LLMReviewActiveStatuses).
						Updates(map[string]any{
							"trigger_window_start_at": windowStart,
							"trigger_window_end_at":   windowEnd,
							"updated_at":              now,
						})
					if result.Error != nil {
						return result.Error
					}
					if result.RowsAffected != 1 {
						return ErrLLMReviewTaskNoLongerActive
					}
					active.TriggerWindowStartAt = windowStart
					active.TriggerWindowEndAt = windowEnd
				}
				selectedTask = active
				selected = true
				return nil
			}

			// Completion releases the active slot in a separate update for
			// legacy callers. Repair a stale pointer before binding a new task
			// so a just-finished task cannot suppress the next RPM window.
			if err := clearStaleActiveReviewSlot(tx, userId); err != nil {
				return err
			}

			task.UserId = userId
			task.TriggerWindowStartAt = windowStart
			task.TriggerWindowEndAt = windowEnd
			task.Status = LLMReviewTaskPending
			if task.ReviewID == "" {
				rid, err := GenerateLLMReviewID()
				if err != nil {
					return err
				}
				task.ReviewID = rid
			}
			if task.MaxCurrentValue < task.CurrentValue {
				task.MaxCurrentValue = task.CurrentValue
			}

			// Park the task until the active slot and window marker are bound in
			// one transaction. This mirrors enqueueLLMReviewTask's draft marker.
			task.NextRetryAt = now + draftRetryMarker
			if err := tx.Create(task).Error; err != nil {
				return err
			}
			result := tx.Model(&LLMReviewGrace{}).
				Where("user_id = ? AND active_task_id = 0 AND rpm_review_window_start_at = ? AND rpm_review_window_end_at = ? AND rpm_review_task_id = 0", userId, windowStart, windowEnd).
				Updates(map[string]any{
					"active_task_id":     task.ID,
					"rpm_review_task_id": task.ID,
					"updated_at":         now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("RPM review window claim lost")
			}
			if err := tx.Model(&LLMReviewTask{}).Where("id = ?", task.ID).Update("next_retry_at", 0).Error; err != nil {
				return err
			}
			selectedTask = task
			selected = true
			created = true
			return nil
		})
		if !errors.Is(err, ErrLLMReviewTaskNoLongerActive) {
			break
		}
	}
	if err != nil {
		return nil, false, false, err
	}
	return selectedTask, selected, created, nil
}

func bindRPMActiveReviewSlot(tx *gorm.DB, userId int, taskID int64) error {
	var grace LLMReviewGrace
	if err := lockForUpdate(tx).Where("user_id = ?", userId).First(&grace).Error; err != nil {
		return err
	}
	if grace.ActiveTaskId == taskID {
		return nil
	}
	if grace.ActiveTaskId > 0 {
		var pointed LLMReviewTask
		err := tx.Select("id", "status").Where("id = ?", grace.ActiveTaskId).First(&pointed).Error
		if err == nil && pointed.Active() {
			return ErrLLMReviewTaskNoLongerActive
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result := tx.Model(&LLMReviewGrace{}).
			Where("user_id = ? AND active_task_id = ?", userId, grace.ActiveTaskId).
			Updates(map[string]any{
				"active_task_id": 0,
				"updated_at":     common.GetTimestamp(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrLLMReviewTaskNoLongerActive
		}
	}

	result := tx.Model(&LLMReviewGrace{}).
		Where("user_id = ? AND active_task_id = 0", userId).
		Updates(map[string]any{
			"active_task_id": taskID,
			"updated_at":     common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrLLMReviewTaskNoLongerActive
	}
	return nil
}

func clearStaleActiveReviewSlot(tx *gorm.DB, userId int) error {
	var grace LLMReviewGrace
	if err := lockForUpdate(tx).Where("user_id = ?", userId).First(&grace).Error; err != nil {
		return err
	}
	if grace.ActiveTaskId == 0 {
		return nil
	}

	var task LLMReviewTask
	err := tx.Select("id", "status").Where("id = ?", grace.ActiveTaskId).First(&task).Error
	if err == nil {
		if task.Active() {
			return ErrLLMReviewTaskNoLongerActive
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	result := tx.Model(&LLMReviewGrace{}).
		Where("user_id = ? AND active_task_id = ?", userId, grace.ActiveTaskId).
		Updates(map[string]any{
			"active_task_id": 0,
			"updated_at":     common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrLLMReviewTaskNoLongerActive
	}
	return nil
}

func bindRPMReviewWindowTask(tx *gorm.DB, userId int, windowStart, windowEnd, taskID, now int64) error {
	result := tx.Model(&LLMReviewGrace{}).
		Where("user_id = ? AND rpm_review_window_start_at = ? AND rpm_review_window_end_at = ? AND rpm_review_task_id = 0", userId, windowStart, windowEnd).
		Updates(map[string]any{
			"rpm_review_task_id": taskID,
			"updated_at":         now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("RPM review window claim lost")
	}
	return nil
}

func mergeLLMReviewTaskTx(tx *gorm.DB, task *LLMReviewTask, triggerType LLMReviewTriggerType, current, limit int, now int64) error {
	isRPM := triggerType == LLMReviewTriggerRPM
	isInput := triggerType == LLMReviewTriggerInputToken
	isOutput := triggerType == LLMReviewTriggerOutputToken
	result := tx.Model(&LLMReviewTask{}).
		Where("id = ? AND status IN ?", task.ID, LLMReviewActiveStatuses).
		Updates(map[string]any{
			"merge_count":          gorm.Expr("merge_count + 1"),
			"trigger_rpm":          gorm.Expr("CASE WHEN ? THEN ? ELSE trigger_rpm END", isRPM, isRPM),
			"trigger_input_token":  gorm.Expr("CASE WHEN ? THEN ? ELSE trigger_input_token END", isInput, isInput),
			"trigger_output_token": gorm.Expr("CASE WHEN ? THEN ? ELSE trigger_output_token END", isOutput, isOutput),
			"max_current_value":    gorm.Expr("CASE WHEN ? > max_current_value THEN ? ELSE max_current_value END", current, current),
			"limit_value":          limit,
			"last_trigger_at":      now,
			"updated_at":           now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLLMReviewTaskNoLongerActive
	}
	return nil
}
