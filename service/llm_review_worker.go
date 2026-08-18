package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	// llmReviewWorkerPollInterval is the fixed worker poll cadence.
	llmReviewWorkerPollInterval = 5 * time.Second
	// llmReviewStaleThreshold recovers reviewing tasks stuck longer than
	// this back to pending (crashed worker / lost lease).
	llmReviewStaleThreshold = 10 * time.Minute
	// llmReviewCleanupInterval runs retention cleanup once an hour.
	llmReviewCleanupInterval = 3600
)

var (
	llmReviewWorkerOnce  sync.Once
	llmReviewWorkerStop  atomic.Bool
	llmReviewWorkerWg    sync.WaitGroup
	llmReviewSem         chan struct{}
	llmReviewSemMu       sync.Mutex
	llmReviewLastCleanup int64
)

// reviewAutoBanUser is the auto-disable seam. Production disables the user
// permanently through the shared entry point; tests replace it to observe the
// call without the full user lifecycle.
var reviewAutoBanUser = func(task *model.LLMReviewTask, message string) error {
	return model.DisableUserPermanently(task.UserId, "")
}

// ensureReviewSem rebuilds the concurrency semaphore when the configured
// concurrency changes at runtime. In-flight tasks keep slots on the old
// semaphore; the new value applies to the next claim round.
func ensureReviewSem(concurrency int) {
	llmReviewSemMu.Lock()
	defer llmReviewSemMu.Unlock()
	if llmReviewSem != nil && cap(llmReviewSem) == concurrency {
		return
	}
	llmReviewSem = make(chan struct{}, concurrency)
}

// StartLLMReviewWorker starts the background review worker on the master node
// only. Disabled review leaves the loop idle (no claims, no reviewer calls).
func StartLLMReviewWorker() {
	llmReviewWorkerOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		common.RelayCtxGo(context.Background(), func() {
			logger.LogInfo(context.Background(), "llm review worker started")
			ticker := time.NewTicker(llmReviewWorkerPollInterval)
			defer ticker.Stop()
			runLLMReviewWorkerPass()
			for range ticker.C {
				if llmReviewWorkerStop.Load() {
					return
				}
				runLLMReviewWorkerPass()
			}
		})
	})
}

// StopLLMReviewWorker stops claiming new tasks and waits for in-flight work.
// Unfinished reviewing tasks are recovered by RecoverStaleLLMReviewTasks on
// the next start.
func StopLLMReviewWorker() {
	llmReviewWorkerStop.Store(true)
	llmReviewWorkerWg.Wait()
}

// runLLMReviewWorkerPass runs one claim round: retention cleanup, stale
// recovery, then claim and dispatch due tasks within the concurrency limit.
func runLLMReviewWorkerPass() {
	cfg := operation_setting.GetLLMReviewSetting()
	if !cfg.Enabled || !operation_setting.GetReviewReadiness(cfg).Ready {
		return
	}
	if now := common.GetTimestamp(); now-llmReviewLastCleanup >= llmReviewCleanupInterval {
		llmReviewLastCleanup = now
		cfg := operation_setting.GetLLMReviewSetting()
		if cfg.LogRetentionDays > 0 {
			if n, err := model.CleanupLLMReviewOldRecords(now, cfg.LogRetentionDays); err != nil {
				logger.LogWarn(context.Background(), fmt.Sprintf("llm review retention cleanup failed: %v", err))
			} else if n > 0 {
				logger.LogInfo(context.Background(), fmt.Sprintf("llm review retention cleanup removed %d records", n))
			}
		}
	}
	if err := model.RecoverStaleLLMReviewTasks(llmReviewStaleThreshold); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("llm review stale recovery failed: %v", err))
	}
	concurrency := operation_setting.ClampWorkerConcurrency(cfg.WorkerConcurrency)
	ensureReviewSem(concurrency)

	tasks, err := model.FindClaimableLLMReviewTasks(concurrency)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("llm review claim query failed: %v", err))
		return
	}
	for _, task := range tasks {
		llmReviewSemMu.Lock()
		sem := llmReviewSem
		select {
		case sem <- struct{}{}:
			llmReviewSemMu.Unlock()
		default:
			llmReviewSemMu.Unlock()
			return
		}

		claimed, ok, err := model.ClaimLLMReviewTask(task.ID)
		if err != nil {
			<-sem
			logger.LogWarn(context.Background(), fmt.Sprintf("llm review claim failed: %v", err))
			continue
		}
		if !ok {
			<-sem
			continue
		}
		llmReviewWorkerWg.Add(1)
		dispatchTask := claimed
		common.RelayCtxGo(context.Background(), func() {
			defer func() { <-sem }()
			defer llmReviewWorkerWg.Done()
			processLLMReviewTask(dispatchTask)
		})
	}
}

// processLLMReviewTask processes one claimed task through the retry policy and
// verdict application.
func processLLMReviewTask(task *model.LLMReviewTask) {
	ctx := context.Background()
	cfg := operation_setting.GetLLMReviewSetting()
	// Manual actions after task creation supersede the review, even when the
	// service has since been disabled or its capability test has expired.
	if reason := checkManualOverride(task); reason != "" {
		_ = model.MarkLLMReviewTaskSuperseded(task.ID, reason)
		return
	}
	// Already permanently disabled users are never sent to the reviewer.
	if banned, _ := model.IsUserPermanentlyBanned(task.UserId); banned {
		_ = model.MarkLLMReviewTaskSuperseded(task.ID, model.SkipReasonManualBan)
		return
	}

	currentPayload, policyText := payloadWithCurrentReviewPolicy(task.Payload)
	if currentPayload != task.Payload {
		if err := model.UpdateLLMReviewTaskPayload(task.ID, currentPayload, ReviewPolicyID, ReviewPromptVersion); err != nil {
			_ = model.FailLLMReviewTask(task.ID, "failed to persist current policy")
			return
		}
		task.Payload = currentPayload
	}
	// Legacy tasks created before the policy prerequisite are completed
	// explicitly as uncertain instead of being sent to an unavailable reviewer.
	if policyText == "" {
		completeLLMReviewWithoutPolicy(task, cfg)
		return
	}

	readiness := operation_setting.GetReviewReadiness(cfg)
	if !cfg.Enabled {
		task.FailureReason = "review service is disabled"
		_ = model.MarkLLMReviewTaskSkipped(task, model.SkipReasonReviewDisabled)
		return
	}
	if !readiness.Ready {
		task.FailureReason = readiness.Reason
		_ = model.MarkLLMReviewTaskSkipped(task, model.SkipReasonReviewUnavailable)
		return
	}

	client := NewReviewClient(cfg)
	task.OutputMode = readiness.Mode
	maxAttempts := cfg.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = ReviewDefaultMaxAttempts
	}
	retryInterval := cfg.RetryIntervalSeconds
	if retryInterval < 1 {
		retryInterval = ReviewDefaultRetryIntervalSec
	}

	attemptNo := task.Attempts + 1 // 1-based
	for ; attemptNo <= maxAttempts; attemptNo++ {
		result := client.Call(ctx, task.Payload)

		attempt := &model.LLMReviewAttempt{
			TaskId:     task.ID,
			AttemptNo:  attemptNo,
			RequestAt:  common.GetTimestamp(),
			DurationMs: result.DurationMs,
			HTTPStatus: result.HTTPStatus,
			Response:   common.MaskReviewCredentialText(common.MaskSensitiveInfo(string(result.Body))),
			Retryable:  isRetryableLLMReviewCall(result),
		}
		if result.Error != nil {
			attempt.ParseError = common.MaskReviewCredentialText(common.MaskSensitiveInfo(result.Error.Error()))
			_ = model.RecordLLMReviewAttempt(attempt)
			if attempt.Retryable && attemptNo < maxAttempts {
				_ = model.MarkLLMReviewTaskRetry(task.ID, attemptNo, retryInterval)
				return
			}
			_ = model.FailLLMReviewTask(task.ID, attempt.ParseError)
			return
		}

		normalized, err := NormalizeRawLLMResponse(result.Body)
		if err != nil {
			attempt.ParseError = common.MaskReviewCredentialText(common.MaskSensitiveInfo("parse content: " + err.Error()))
			attempt.Retryable = true
			_ = model.RecordLLMReviewAttempt(attempt)
			if attemptNo < maxAttempts {
				_ = model.MarkLLMReviewTaskRetry(task.ID, attemptNo, retryInterval)
				return
			}
			_ = model.FailLLMReviewTask(task.ID, "unable to normalize llm response: "+err.Error())
			return
		}

		validateVerdict := ValidateLLMReviewVerdict
		if task.OutputMode == operation_setting.StructuredOutputModeStrictSchema {
			validateVerdict = ValidateStrictLLMReviewVerdict
		}
		verdict, schemaPassed, schemaErr := validateVerdict([]byte(normalized.Content))
		attempt.ParseError = schemaErr
		attempt.Retryable = !schemaPassed
		_ = model.RecordLLMReviewAttempt(attempt)
		if !schemaPassed {
			if attemptNo < maxAttempts {
				_ = model.MarkLLMReviewTaskRetry(task.ID, attemptNo, retryInterval)
				return
			}
			_ = model.FailLLMReviewTask(task.ID, "schema validation failed: "+schemaErr)
			return
		}

		applyLLMReviewVerdict(task, verdict, attempt, cfg, !normalized.Repaired)
		return
	}
}

// completeLLMReviewWithoutPolicy finishes a task as uncertain when no policy
// text is configured, so the reviewer never guesses site rules.
func completeLLMReviewWithoutPolicy(task *model.LLMReviewTask, cfg *operation_setting.LLMReviewSetting) {
	task.Status = model.LLMReviewTaskUncertain
	task.Verdict = model.LLMReviewVerdictUncertain
	task.Category = model.LLMReviewCategoryNone
	task.Confidence = 0
	task.Reason = "未配置使用条款，无法依据站点规则完成审查，已转人工复核。"
	task.Evidence = model.LLMReviewEvidence{"请管理员在系统设置中填写 Policy Text（使用条款）后重新提交审查。"}
	task.ReviewerModel = cfg.ModelName
	task.PolicyID = ReviewPolicyID
	task.PromptVersion = ReviewPromptVersion
	task.SchemaVersion = ReviewSchemaVersion
	task.SchemaPassed = false
	task.SchemaError = "missing policy"
	if err := model.CompleteLLMReviewTask(task, nil); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("llm review complete missing-policy task failed: %v", err))
	}
}

// isRetryableLLMReviewCall: network errors/timeouts and HTTP 429/5xx retry;
// other 4xx responses are configuration errors and do not retry.
func isRetryableLLMReviewCall(result LLMReviewCallResult) bool {
	if result.Error == nil {
		return false
	}
	if result.HTTPStatus == 0 {
		return true // network error / timeout
	}
	if result.HTTPStatus == httpStatusTooManyRequests || result.HTTPStatus >= 500 {
		return true
	}
	return false
}

const httpStatusTooManyRequests = 429

// checkManualOverride compares task creation time with manual ban/unban
// times. Returns the supersede reason, or "" for no override.
func checkManualOverride(task *model.LLMReviewTask) string {
	grace, err := model.GetLLMReviewGrace(task.UserId)
	if err != nil || grace == nil {
		return ""
	}
	if grace.LastManualBanAt > task.CreatedAt {
		return model.SkipReasonManualBan
	}
	if grace.LastManualUnbanAt > task.CreatedAt {
		return model.SkipReasonManualUnban
	}
	return ""
}

// applyLLMReviewVerdict applies a schema-valid verdict. Before an automatic
// permanent disable the manual ban/unban timestamps are re-checked to close
// the race with admin actions during the reviewer call.
func applyLLMReviewVerdict(task *model.LLMReviewTask, verdict *ReviewVerdictResponse, attempt *model.LLMReviewAttempt, cfg *operation_setting.LLMReviewSetting, trustedRaw bool) {
	now := common.GetTimestamp()

	task.Verdict = model.LLMReviewVerdict(verdict.Verdict)
	task.Category = model.LLMReviewCategory(verdict.Category)
	task.Confidence = verdict.Confidence
	task.Reason = verdict.Reason
	task.Evidence = model.LLMReviewEvidence(verdict.Evidence)
	task.ReviewerModel = cfg.ModelName
	task.PolicyID = ReviewPolicyID
	task.PromptVersion = ReviewPromptVersion
	task.SchemaVersion = ReviewSchemaVersion
	task.SchemaPassed = true
	task.RawResponse = common.MaskReviewCredentialText(common.MaskSensitiveInfo(attempt.Response))

	switch verdict.Verdict {
	case "compliant":
		task.Status = model.LLMReviewTaskCompliant
		if err := model.RecordLLMReviewCompliant(task.UserId, now); err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("llm review record compliant failed: %v", err))
		}
	case "violation":
		task.Status = model.LLMReviewTaskViolation
		// Re-check manual actions before auto-disable (admin may have acted
		// during the reviewer call).
		if reason := checkManualOverride(task); reason != "" {
			task.Status = model.LLMReviewTaskSuperseded
			task.SupersededBy = reason
			task.SkipReason = reason
			break
		}
		if ShouldAutoBanWithTrust(verdict, true, cfg, task.OutputMode, trustedRaw) && !model.IsRootUser(task.UserId) {
			banMessage := fmt.Sprintf(
				"账号因违反本站使用条款已被永久禁用。如认为存在误判，请联系管理员并提供审查编号 %s。",
				task.ReviewID)
			if err := reviewAutoBanUser(task, banMessage); err != nil {
				task.BanError = common.MaskReviewCredentialText(common.MaskSensitiveInfo(err.Error()))
				logger.LogWarn(context.Background(), fmt.Sprintf("llm review auto ban failed for user %d: %v", task.UserId, err))
			} else {
				task.Banned = true
				task.BanMessage = banMessage
			}
		}
	case "uncertain":
		task.Status = model.LLMReviewTaskUncertain
	default:
		task.Status = model.LLMReviewTaskUncertain
	}

	if err := model.CompleteLLMReviewTask(task, nil); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("llm review complete task failed: %v", err))
	}
}
