package model

import (
	"errors"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// LLMReviewActiveStatuses is the active task status set (one active task per
// user at most).
var LLMReviewActiveStatuses = []string{
	string(LLMReviewTaskPending),
	string(LLMReviewTaskReviewing),
}

// ---------------------------------------------------------------------------
// Task lifecycle
// ---------------------------------------------------------------------------

// GenerateLLMReviewID returns an unguessable review number so tasks cannot be
// enumerated.
func GenerateLLMReviewID() (string, error) {
	key, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		return "", err
	}
	return "rev-" + key, nil
}

// CreateLLMReviewTask creates a pending task (payload must be sanitized by the
// caller before serialization).
func CreateLLMReviewTask(task *LLMReviewTask) error {
	if task.ReviewID == "" {
		rid, err := GenerateLLMReviewID()
		if err != nil {
			return err
		}
		task.ReviewID = rid
	}
	task.Status = LLMReviewTaskPending
	return DB.Create(task).Error
}

// GetActiveLLMReviewTask returns the user's current active task, preferring
// the grace-row slot and falling back to a status query for legacy rows.
func GetActiveLLMReviewTask(userId int) (*LLMReviewTask, error) {
	grace, err := GetLLMReviewGrace(userId)
	if err == nil && grace != nil && grace.ActiveTaskId > 0 {
		var task LLMReviewTask
		if err := DB.Where("id = ? AND status IN ?", grace.ActiveTaskId, LLMReviewActiveStatuses).
			First(&task).Error; err == nil {
			return &task, nil
		}
	}
	var task LLMReviewTask
	err = DB.Where("user_id = ? AND status IN ?", userId, LLMReviewActiveStatuses).
		Order("id desc").
		First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// HasActiveLLMReviewTask reports whether the user has an active task.
func HasActiveLLMReviewTask(userId int) (bool, error) {
	active, err := GetActiveLLMReviewTask(userId)
	if err != nil {
		return false, err
	}
	return active != nil, nil
}

// ClaimActiveReviewSlot atomically binds the user's active slot to taskID via
// a conditional UPDATE (WHERE active_task_id = 0), safe across all three
// databases. Exactly one concurrent claim per user succeeds.
func ClaimActiveReviewSlot(userId int, taskID int64) (bool, error) {
	if userId <= 0 || taskID <= 0 {
		return false, errors.New("invalid userId or taskID")
	}
	if _, err := GetOrCreateLLMReviewGrace(userId); err != nil {
		return false, err
	}
	now := common.GetTimestamp()
	result := DB.Model(&LLMReviewGrace{}).
		Where("user_id = ? AND active_task_id = 0", userId).
		Updates(map[string]any{
			"active_task_id": taskID,
			"updated_at":     now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ReleaseActiveReviewSlot clears the user's slot when the task leaves the
// active state. taskID==0 clears any non-zero slot (anomaly recovery).
func ReleaseActiveReviewSlot(userId int, taskID int64) error {
	if userId <= 0 {
		return nil
	}
	query := DB.Model(&LLMReviewGrace{}).Where("user_id = ?", userId)
	if taskID > 0 {
		query = query.Where("active_task_id = ?", taskID)
	} else {
		query = query.Where("active_task_id != 0")
	}
	return query.Updates(map[string]any{
		"active_task_id": 0,
		"updated_at":     common.GetTimestamp(),
	}).Error
}

// DeleteLLMReviewTask deletes a task and releases its slot.
func DeleteLLMReviewTask(id int64) error {
	var task LLMReviewTask
	if err := DB.Select("user_id").Where("id = ?", id).First(&task).Error; err != nil {
		return err
	}
	_ = ReleaseActiveReviewSlot(task.UserId, id)
	return DB.Delete(&LLMReviewTask{}, id).Error
}

// ErrLLMReviewTaskNoLongerActive means the merge target left the active
// state; the caller re-runs atomic enqueue to attach the event to a new task.
var ErrLLMReviewTaskNoLongerActive = errors.New("LLM review task is no longer active")

// MergeLLMReviewTask merges a trigger event into the active task using atomic
// CASE WHEN updates so concurrent merges never lose an update.
func MergeLLMReviewTask(task *LLMReviewTask, triggerType LLMReviewTriggerType, current, limit int) error {
	if task == nil {
		return errors.New("nil task")
	}
	now := common.GetTimestamp()
	isRPM := triggerType == LLMReviewTriggerRPM
	isInput := triggerType == LLMReviewTriggerInputToken
	isOutput := triggerType == LLMReviewTriggerOutputToken
	result := DB.Model(&LLMReviewTask{}).
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

// FindClaimableLLMReviewTasks returns pending tasks whose retry time has
// arrived, ordered by creation time.
func FindClaimableLLMReviewTasks(limit int) ([]*LLMReviewTask, error) {
	if limit <= 0 {
		limit = 20
	}
	now := common.GetTimestamp()
	var tasks []*LLMReviewTask
	err := DB.Where("status = ? AND (next_retry_at = 0 OR next_retry_at <= ?)",
		LLMReviewTaskPending, now).
		Order("id asc").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

// ClaimLLMReviewTask atomically claims a task (pending -> reviewing) so
// multiple instances never process it twice.
func ClaimLLMReviewTask(id int64) (*LLMReviewTask, bool, error) {
	now := common.GetTimestamp()
	result := DB.Model(&LLMReviewTask{}).
		Where("id = ? AND status = ?", id, LLMReviewTaskPending).
		Updates(map[string]any{
			"status":          LLMReviewTaskReviewing,
			"started_at":      now,
			"last_attempt_at": now,
			"updated_at":      now,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	var task LLMReviewTask
	if err := DB.Where("id = ?", id).First(&task).Error; err != nil {
		return nil, false, err
	}
	return &task, true, nil
}

// UpdateLLMReviewTaskPayload persists the current policy payload on a
// reviewing task before the reviewer call.
func UpdateLLMReviewTaskPayload(id int64, payload, policyID, promptVersion string) error {
	result := DB.Model(&LLMReviewTask{}).
		Where("id = ? AND status = ?", id, LLMReviewTaskReviewing).
		Updates(map[string]any{
			"payload":        payload,
			"policy_id":      policyID,
			"prompt_version": promptVersion,
			"updated_at":     common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("LLM review task payload update lost reviewing state")
	}
	return nil
}

// MarkLLMReviewTaskRetry records a failed attempt and schedules the fixed
// retry interval (reviewing -> pending with next_retry_at).
func MarkLLMReviewTaskRetry(taskID int64, attempts int, retryIntervalSeconds int) error {
	now := common.GetTimestamp()
	next := now + int64(retryIntervalSeconds)
	return DB.Model(&LLMReviewTask{}).
		Where("id = ? AND status = ?", taskID, LLMReviewTaskReviewing).
		Updates(map[string]any{
			"status":          LLMReviewTaskPending,
			"attempts":        attempts,
			"next_retry_at":   next,
			"last_attempt_at": now,
			"updated_at":      now,
		}).Error
}

// ReleaseLLMReviewTask puts a claimed task back to pending when the
// concurrency pool is full.
func ReleaseLLMReviewTask(taskID int64) error {
	return DB.Model(&LLMReviewTask{}).
		Where("id = ? AND status = ?", taskID, LLMReviewTaskReviewing).
		Updates(map[string]any{
			"status":     LLMReviewTaskPending,
			"updated_at": common.GetTimestamp(),
			"started_at": 0,
		}).Error
}

// RecoverStaleLLMReviewTasks resets reviewing tasks stuck past the stale
// threshold so a restarted worker can retry them.
func RecoverStaleLLMReviewTasks(staleThreshold time.Duration) error {
	if staleThreshold <= 0 {
		staleThreshold = 10 * time.Minute
	}
	cutoff := common.GetTimestamp() - int64(staleThreshold.Seconds())
	return DB.Model(&LLMReviewTask{}).
		Where("status = ? AND started_at > 0 AND started_at < ?", LLMReviewTaskReviewing, cutoff).
		Updates(map[string]any{
			"status":     LLMReviewTaskPending,
			"started_at": 0,
			"updated_at": common.GetTimestamp(),
		}).Error
}

// RecordLLMReviewAttempt persists one masked attempt record.
func RecordLLMReviewAttempt(attempt *LLMReviewAttempt) error {
	if attempt == nil {
		return errors.New("nil review attempt")
	}
	attempt.Response = common.MaskReviewCredentialText(common.MaskSensitiveInfo(attempt.Response))
	attempt.ParseError = common.MaskReviewCredentialText(common.MaskSensitiveInfo(attempt.ParseError))
	return DB.Create(attempt).Error
}

// FailLLMReviewTask marks retry exhaustion and releases the user slot.
func FailLLMReviewTask(id int64, reason string) error {
	reason = common.MaskReviewCredentialText(common.MaskSensitiveInfo(reason))
	var task LLMReviewTask
	if err := DB.Select("user_id", "status").Where("id = ?", id).First(&task).Error; err != nil {
		return err
	}
	_ = ReleaseActiveReviewSlot(task.UserId, id)
	return DB.Model(&LLMReviewTask{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":         LLMReviewTaskFailed,
			"failure_reason": reason,
			"completed_at":   common.GetTimestamp(),
			"updated_at":     common.GetTimestamp(),
		}).Error
}

// CompleteLLMReviewTask writes the verdict, evidence, masked raw response and
// schema results, then releases the user slot. The optional attempt is
// recorded in the same call path.
func CompleteLLMReviewTask(task *LLMReviewTask, attempt *LLMReviewAttempt) error {
	if task == nil {
		return errors.New("nil review task")
	}
	task.RawResponse = common.MaskReviewCredentialText(common.MaskSensitiveInfo(task.RawResponse))
	task.SchemaError = common.MaskReviewCredentialText(common.MaskSensitiveInfo(task.SchemaError))
	task.BanError = common.MaskReviewCredentialText(common.MaskSensitiveInfo(task.BanError))
	if attempt != nil {
		if err := RecordLLMReviewAttempt(attempt); err != nil {
			return err
		}
	}
	now := common.GetTimestamp()
	err := DB.Model(&LLMReviewTask{}).
		Where("id = ? AND status = ?", task.ID, LLMReviewTaskReviewing).
		Updates(map[string]any{
			"status":         task.Status,
			"verdict":        task.Verdict,
			"category":       task.Category,
			"confidence":     task.Confidence,
			"reason":         task.Reason,
			"evidence":       task.Evidence,
			"raw_response":   task.RawResponse,
			"schema_passed":  task.SchemaPassed,
			"schema_error":   task.SchemaError,
			"reviewer_model": task.ReviewerModel,
			"policy_id":      task.PolicyID,
			"prompt_version": task.PromptVersion,
			"schema_version": task.SchemaVersion,
			"banned":         task.Banned,
			"ban_message":    task.BanMessage,
			"ban_error":      task.BanError,
			"completed_at":   now,
			"updated_at":     now,
		}).Error
	if err != nil {
		return err
	}
	return ReleaseActiveReviewSlot(task.UserId, task.ID)
}

// MarkLLMReviewTaskSuperseded marks a task superseded by a later manual
// action and releases the slot.
func MarkLLMReviewTaskSuperseded(id int64, reason string) error {
	var task LLMReviewTask
	if err := DB.Select("user_id", "status").Where("id = ?", id).First(&task).Error; err != nil {
		return err
	}
	_ = ReleaseActiveReviewSlot(task.UserId, id)
	return DB.Model(&LLMReviewTask{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        LLMReviewTaskSuperseded,
			"superseded_by": reason,
			"skip_reason":   reason,
			"completed_at":  common.GetTimestamp(),
			"updated_at":    common.GetTimestamp(),
		}).Error
}

// MarkLLMReviewTaskSkipped marks a task skipped with a reason. Unsaved tasks
// (ID==0) are created in the skipped state without occupying the user slot.
func MarkLLMReviewTaskSkipped(task *LLMReviewTask, reason string) error {
	task.Status = LLMReviewTaskSkipped
	task.SkipReason = reason
	task.CompletedAt = common.GetTimestamp()
	task.UpdatedAt = common.GetTimestamp()
	if task.ID > 0 {
		_ = ReleaseActiveReviewSlot(task.UserId, task.ID)
		return DB.Model(&LLMReviewTask{}).Where("id = ?", task.ID).
			Updates(map[string]any{
				"status":       LLMReviewTaskSkipped,
				"skip_reason":  reason,
				"completed_at": task.CompletedAt,
				"updated_at":   task.UpdatedAt,
			}).Error
	}
	if task.ReviewID == "" {
		rid, err := GenerateLLMReviewID()
		if err != nil {
			return err
		}
		task.ReviewID = rid
	}
	return DB.Create(task).Error
}

// CancelPendingLLMReviewTasks supersedes all pending tasks of a user (manual
// permanent disable) and releases the slot.
func CancelPendingLLMReviewTasks(userId int, reason string) error {
	var active LLMReviewTask
	if err := DB.Where("user_id = ? AND status = ?", userId, LLMReviewTaskPending).
		Order("id asc").First(&active).Error; err == nil {
		_ = ReleaseActiveReviewSlot(userId, active.ID)
	}
	return DB.Model(&LLMReviewTask{}).
		Where("user_id = ? AND status = ?", userId, LLMReviewTaskPending).
		Updates(map[string]any{
			"status":        LLMReviewTaskSuperseded,
			"superseded_by": reason,
			"skip_reason":   reason,
			"completed_at":  common.GetTimestamp(),
			"updated_at":    common.GetTimestamp(),
		}).Error
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

// GetLLMReviewTask returns a task by numeric ID with hydrated metadata.
func GetLLMReviewTask(id int64) (*LLMReviewTask, error) {
	var task LLMReviewTask
	if err := DB.Where("id = ?", id).First(&task).Error; err != nil {
		return nil, err
	}
	HydrateLLMReviewTaskMetadata(&task)
	return &task, nil
}

// GetLLMReviewTaskByReviewID returns a task by its unguessable review number.
func GetLLMReviewTaskByReviewID(reviewID string) (*LLMReviewTask, error) {
	var task LLMReviewTask
	if err := DB.Where("review_id = ?", reviewID).First(&task).Error; err != nil {
		return nil, err
	}
	HydrateLLMReviewTaskMetadata(&task)
	return &task, nil
}

// ListLLMReviewTasks pages tasks with filters. Keyword matches username or
// model name with a cross-database-safe LIKE.
func ListLLMReviewTasks(page, pageSize int, status string, userId int, modelName string, triggerType string, category string, username string, keyword string, startTime, endTime int64) ([]*LLMReviewTask, int64, error) {
	query := DB.Model(&LLMReviewTask{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	if triggerType != "" {
		query = query.Where("trigger_type = ?", triggerType)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if username != "" {
		query = query.Where("username = ?", username)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(username LIKE ? OR model_name LIKE ?)", like, like)
	}
	if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tasks []*LLMReviewTask
	err := query.Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tasks).Error
	if err != nil {
		return nil, 0, err
	}
	for _, task := range tasks {
		HydrateLLMReviewTaskMetadata(task)
	}
	return tasks, total, nil
}

// ListLLMReviewAttempts returns a task's attempt records in order.
func ListLLMReviewAttempts(taskId int64) ([]*LLMReviewAttempt, error) {
	var attempts []*LLMReviewAttempt
	err := DB.Where("task_id = ?", taskId).Order("attempt_no asc").Find(&attempts).Error
	return attempts, err
}

// LLMReviewQueueSummary is the admin queue overview.
type LLMReviewQueueSummary struct {
	Pending              int64   `json:"pending"`
	Reviewing            int64   `json:"reviewing"`
	OldestWaitingSeconds int64   `json:"oldest_waiting_seconds"`
	RecentFailureRate    float64 `json:"recent_failure_rate"`
	MergedEvents         int64   `json:"merged_events"`
}

// GetLLMReviewQueueSummary aggregates queue health.
func GetLLMReviewQueueSummary() (LLMReviewQueueSummary, error) {
	var summary LLMReviewQueueSummary
	now := common.GetTimestamp()
	dayAgo := now - 86400
	if err := DB.Model(&LLMReviewTask{}).
		Where("status = ?", LLMReviewTaskPending).Count(&summary.Pending).Error; err != nil {
		return summary, err
	}
	if err := DB.Model(&LLMReviewTask{}).
		Where("status = ?", LLMReviewTaskReviewing).Count(&summary.Reviewing).Error; err != nil {
		return summary, err
	}
	var oldest int64
	if err := DB.Model(&LLMReviewTask{}).
		Where("status = ?", LLMReviewTaskPending).
		Select("COALESCE(MIN(created_at), 0)").Scan(&oldest).Error; err != nil {
		return summary, err
	}
	if oldest > 0 {
		summary.OldestWaitingSeconds = now - oldest
		if summary.OldestWaitingSeconds < 0 {
			summary.OldestWaitingSeconds = 0
		}
	}
	var completed24h int64
	var failed24h int64
	_ = DB.Model(&LLMReviewTask{}).
		Where("status IN ? AND updated_at >= ?", []string{
			string(LLMReviewTaskCompliant),
			string(LLMReviewTaskViolation),
			string(LLMReviewTaskUncertain),
			string(LLMReviewTaskFailed),
		}, dayAgo).Count(&completed24h).Error
	_ = DB.Model(&LLMReviewTask{}).
		Where("status = ? AND updated_at >= ?", LLMReviewTaskFailed, dayAgo).Count(&failed24h).Error
	if completed24h > 0 {
		summary.RecentFailureRate = float64(failed24h) / float64(completed24h)
	}
	if err := DB.Model(&LLMReviewTask{}).
		Select("COALESCE(SUM(merge_count), 0)").
		Scan(&summary.MergedEvents).Error; err != nil {
		return summary, err
	}
	return summary, nil
}

// RetryLLMReviewTask re-queues a failed/uncertain task and atomically claims
// the user slot. A concurrent active task for the same user is rejected.
func RetryLLMReviewTask(id int64) error {
	var task LLMReviewTask
	if err := DB.Select("user_id", "status").Where("id = ?", id).First(&task).Error; err != nil {
		return err
	}
	if task.Status != LLMReviewTaskFailed && task.Status != LLMReviewTaskUncertain {
		return errors.New("only failed or uncertain tasks can be retried")
	}
	active, err := GetActiveLLMReviewTask(task.UserId)
	if err != nil {
		return err
	}
	if active != nil && active.ID != id {
		return errors.New("user already has an active review task")
	}
	now := common.GetTimestamp()
	err = DB.Model(&LLMReviewTask{}).
		Where("id = ? AND status IN ?", id, []string{
			string(LLMReviewTaskFailed),
			string(LLMReviewTaskUncertain),
		}).
		Updates(map[string]any{
			"status":          LLMReviewTaskPending,
			"attempts":        0,
			"next_retry_at":   0,
			"started_at":      0,
			"last_attempt_at": 0,
			"completed_at":    0,
			"verdict":         "",
			"category":        "",
			"confidence":      0,
			"reason":          "",
			"evidence":        LLMReviewEvidence{},
			"raw_response":    "",
			"failure_reason":  "",
			"updated_at":      now,
		}).Error
	if err != nil {
		return err
	}
	claimed, err := ClaimActiveReviewSlot(task.UserId, id)
	if err != nil {
		return err
	}
	if !claimed {
		// A concurrent task claimed the slot; roll back to the prior state.
		_ = DB.Model(&LLMReviewTask{}).Where("id = ?", id).
			Update("status", task.Status).Error
		return errors.New("concurrent active task claimed the slot")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Grace state
// ---------------------------------------------------------------------------

// GetOrCreateLLMReviewGrace returns the user's grace row, creating it when
// absent. Concurrent creation races are resolved by re-reading the row
// (duplicate-key error codes differ across the three databases).
func GetOrCreateLLMReviewGrace(userId int) (*LLMReviewGrace, error) {
	var grace LLMReviewGrace
	err := DB.Where("user_id = ?", userId).First(&grace).Error
	if err == nil {
		return &grace, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	grace = LLMReviewGrace{UserId: userId}
	if err := DB.Create(&grace).Error; err != nil {
		var existing LLMReviewGrace
		if readErr := DB.Where("user_id = ?", userId).First(&existing).Error; readErr == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &grace, nil
}

// RecordLLMReviewCompliant increments the compliant counter and opens a grace
// window when the configured count is reached.
func RecordLLMReviewCompliant(userId int, now int64) error {
	cfg := operation_setting.GetLLMReviewSetting()
	maxCount := cfg.MaxCompliantCount
	if maxCount < 1 {
		maxCount = 3
	}
	graceHours := cfg.GracePeriodHours
	if graceHours < 1 {
		graceHours = 5
	}
	grace, err := GetOrCreateLLMReviewGrace(userId)
	if err != nil {
		return err
	}
	grace.CompliantCount++
	grace.LastCompliantAt = now
	if grace.CompliantCount >= maxCount {
		grace.GraceStartAt = now
		grace.GraceEndAt = now + int64(graceHours)*3600
		grace.CompliantCount = 0
	}
	return DB.Model(&LLMReviewGrace{}).Where("user_id = ?", userId).
		Updates(map[string]any{
			"compliant_count":   grace.CompliantCount,
			"grace_start_at":    grace.GraceStartAt,
			"grace_end_at":      grace.GraceEndAt,
			"last_compliant_at": grace.LastCompliantAt,
			"updated_at":        common.GetTimestamp(),
		}).Error
}

// CheckLLMReviewGrace reports whether the user is inside a grace period and
// resets expired counters.
func CheckLLMReviewGrace(userId int, now int64) (bool, string, error) {
	grace, err := GetOrCreateLLMReviewGrace(userId)
	if err != nil {
		return false, "", err
	}
	if grace.InGracePeriod(now) {
		return true, SkipReasonGracePeriod, nil
	}
	if grace.GraceEndAt > 0 {
		if err := DB.Model(&LLMReviewGrace{}).Where("user_id = ?", userId).
			Updates(map[string]any{
				"compliant_count": 0,
				"grace_start_at":  0,
				"grace_end_at":    0,
				"updated_at":      common.GetTimestamp(),
			}).Error; err != nil {
			return false, "", err
		}
	}
	return false, "", nil
}

// RecordLLMReviewManualBan records a manual permanent disable time, which
// supersedes older pending review results.
func RecordLLMReviewManualBan(userId int, now int64) error {
	grace, err := GetOrCreateLLMReviewGrace(userId)
	if err != nil {
		return err
	}
	grace.LastManualBanAt = now
	return DB.Model(&LLMReviewGrace{}).Where("user_id = ?", userId).
		Updates(map[string]any{
			"last_manual_ban_at": now,
			"updated_at":         common.GetTimestamp(),
		}).Error
}

// RecordLLMReviewManualUnban records a manual re-enable time and resets stale
// grace counters so new anomalies can create fresh tasks.
func RecordLLMReviewManualUnban(userId int, now int64) error {
	grace, err := GetOrCreateLLMReviewGrace(userId)
	if err != nil {
		return err
	}
	grace.LastManualUnbanAt = now
	grace.CompliantCount = 0
	grace.GraceStartAt = 0
	grace.GraceEndAt = 0
	return DB.Model(&LLMReviewGrace{}).Where("user_id = ?", userId).
		Updates(map[string]any{
			"last_manual_unban_at": now,
			"compliant_count":      0,
			"grace_start_at":       0,
			"grace_end_at":         0,
			"updated_at":           common.GetTimestamp(),
		}).Error
}

// GetLLMReviewGrace returns the user's grace row or nil.
func GetLLMReviewGrace(userId int) (*LLMReviewGrace, error) {
	var grace LLMReviewGrace
	err := DB.Where("user_id = ?", userId).First(&grace).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &grace, nil
}

// ---------------------------------------------------------------------------
// Calibration samples
// ---------------------------------------------------------------------------

// RecordLLMReviewCalibration persists one sample and recomputes the model's
// acceptance flag in the same transaction.
func RecordLLMReviewCalibration(sample *LLMReviewCalibration) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(sample).Error; err != nil {
			return err
		}
		stats, err := getLLMReviewCalibrationStats(tx, sample.ModelName, sample.EstimatorVersion)
		if err != nil {
			return err
		}
		passed := llmReviewCalibrationPassed(stats)
		sample.ModelPassed = passed
		return tx.Model(&LLMReviewCalibration{}).
			Where("id = ?", sample.ID).
			Update("model_passed", passed).Error
	})
}

// IsValidLLMReviewCalibrationSample reports whether a calibration sample fits
// the durable token/quota range used by persistence and eligibility stats.
func IsValidLLMReviewCalibrationSample(modelName string, estimate, actual, limit int) bool {
	return modelName != "" && estimate >= 0 && estimate <= common.MaxQuota && actual > 0 && actual <= common.MaxQuota && limit >= 0 && limit <= common.MaxQuota
}

// RecordLLMReviewCalibrationSample records an (estimate, actual) input token
// sample. Values outside the durable quota range are ignored; limit==0
// disables the near-threshold flag.
func RecordLLMReviewCalibrationSample(modelName string, estimate, actual, limit int, estimatorVersion string) error {
	if !IsValidLLMReviewCalibrationSample(modelName, estimate, actual, limit) {
		return nil
	}
	if estimatorVersion == "" {
		estimatorVersion = "estimator-v1"
	}
	relErr := math.Abs(float64(estimate)-float64(actual)) / float64(actual)
	nearThreshold := false
	if limit > 0 {
		nearThreshold = float64(actual) >= float64(limit)*0.9
	}
	sample := &LLMReviewCalibration{
		ModelName:        modelName,
		EstimateTokens:   estimate,
		ActualTokens:     actual,
		LimitValue:       limit,
		RelativeError:    relErr,
		NearThreshold:    nearThreshold,
		EstimatorVersion: estimatorVersion,
		SampleTime:       common.GetTimestamp(),
	}
	return RecordLLMReviewCalibration(sample)
}

// GetLLMReviewCalibrationStats returns the model's calibration statistics;
// callers fail open when the database is unavailable.
func GetLLMReviewCalibrationStats(modelName string, estimatorVersion string) (*LLMReviewCalibrationStatsResult, error) {
	if DB == nil {
		return nil, errors.New("database not initialized")
	}
	return getLLMReviewCalibrationStats(DB, modelName, estimatorVersion)
}

func getLLMReviewCalibrationStats(db *gorm.DB, modelName string, estimatorVersion string) (*LLMReviewCalibrationStatsResult, error) {
	query := db.Model(&LLMReviewCalibration{}).Where("model_name = ?", modelName)
	if estimatorVersion != "" {
		query = query.Where("estimator_version = ?", estimatorVersion)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var pass int64
	if err := query.Where("relative_error <= ?", 0.05).Count(&pass).Error; err != nil {
		return nil, err
	}
	// Near-threshold false rejects: the estimate exceeded the limit while the
	// actual input stayed within it, and the actual was close to the limit.
	var nearFalseReject int64
	sub := db.Model(&LLMReviewCalibration{}).
		Select("id").
		Where("model_name = ? AND limit_value > 0 AND near_threshold = ? AND estimate_tokens > limit_value AND actual_tokens <= limit_value", modelName, true)
	if estimatorVersion != "" {
		sub = sub.Where("estimator_version = ?", estimatorVersion)
	}
	if err := db.Model(&LLMReviewCalibration{}).
		Where("id IN (?)", sub).
		Count(&nearFalseReject).Error; err != nil {
		return nil, err
	}
	return &LLMReviewCalibrationStatsResult{
		ModelName:        modelName,
		EstimatorVersion: estimatorVersion,
		SampleCount:      int(total),
		PassCount:        int(pass),
		NearFalseReject:  int(nearFalseReject),
	}, nil
}

// LLMReviewCalibrationStatsResult is the aggregated calibration result.
type LLMReviewCalibrationStatsResult struct {
	ModelName        string
	EstimatorVersion string
	SampleCount      int
	PassCount        int
	NearFalseReject  int
}

// PassRate is the share of samples within 5% relative error.
func (s *LLMReviewCalibrationStatsResult) PassRate() float64 {
	if s.SampleCount <= 0 {
		return 0
	}
	return float64(s.PassCount) / float64(s.SampleCount)
}

// llmReviewCalibrationPassed implements the acceptance criteria: at least
// 1000 samples, >=95% within 5% relative error, zero near-threshold false
// rejects.
func llmReviewCalibrationPassed(stats *LLMReviewCalibrationStatsResult) bool {
	return stats != nil && stats.SampleCount >= 1000 && stats.PassRate() >= 0.95 && stats.NearFalseReject == 0
}

// IsLLMReviewPreflightEligible reports whether the estimator passed model
// acceptance. Missing data or errors return false and callers must fail open.
func IsLLMReviewPreflightEligible(modelName, estimatorVersion string) bool {
	stats, err := GetLLMReviewCalibrationStats(modelName, estimatorVersion)
	if err != nil {
		return false
	}
	return llmReviewCalibrationPassed(stats)
}

// ---------------------------------------------------------------------------
// Retention
// ---------------------------------------------------------------------------

// CleanupLLMReviewOldRecords removes expired tasks. Records tied to permanent
// disables or violations are never deleted.
func CleanupLLMReviewOldRecords(now int64, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	cutoff := now - int64(retentionDays)*86400
	result := DB.Where("created_at < ? AND banned = ? AND verdict != ?", cutoff, false, LLMReviewVerdictViolation).
		Delete(&LLMReviewTask{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// EnsureLLMReviewGraceTable migrates the grace table (test setup helper).
func EnsureLLMReviewGraceTable(db *gorm.DB) error {
	return db.AutoMigrate(&LLMReviewGrace{})
}
