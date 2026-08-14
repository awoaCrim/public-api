package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"
)

// LLMReviewTriggerType is the review trigger category.
type LLMReviewTriggerType string

const (
	LLMReviewTriggerRPM         LLMReviewTriggerType = "rpm"
	LLMReviewTriggerInputToken  LLMReviewTriggerType = "input_token"
	LLMReviewTriggerOutputToken LLMReviewTriggerType = "output_token"
)

// LLMReviewStage is the trigger stage.
type LLMReviewStage string

const (
	LLMReviewStagePreflight  LLMReviewStage = "preflight"
	LLMReviewStagePostflight LLMReviewStage = "postflight"
)

// LLMReviewTrigger is the minimal sanitized trigger event. It never carries
// usernames, emails, API keys, full headers or raw IPs.
type LLMReviewTrigger struct {
	UserId         int
	ModelName      string
	ChannelId      int
	Endpoint       string
	IsStream       bool
	TriggerType    LLMReviewTriggerType
	Stage          LLMReviewStage
	CurrentValue   int
	LimitValue     int
	EstimateInput  int
	ActualInput    int
	ActualOutput   int
	RequestSnippet string
	// ClientIP is used only to compute the irreversible payload hash and the
	// partial mask; it is never persisted raw or sent to the reviewer.
	ClientIP string
}

// EnqueueLLMReview persists a trigger event for the review worker. It must
// never block the caller's 429 or normal response; callers invoke it from a
// goroutine with values extracted synchronously from gin.Context.
//
// Behavior:
//   - root users are exempt and create no tasks (including skipped ones);
//   - disabled review records a skipped task (skip_reason=review_disabled);
//   - permanently disabled users record a skipped task (skip_reason=
//     skipped_disabled) for audit without invoking the reviewer;
//   - users in a grace period record a skipped task (skip_reason=
//     grace_period);
//   - otherwise the event is atomically enqueued or merged into the user's
//     active task (database slot CAS, cross-instance safe).
func EnqueueLLMReview(ctx context.Context, trigger LLMReviewTrigger) error {
	now := common.GetTimestamp()
	cfg := operation_setting.GetLLMReviewSetting()

	// 0. Root exemption.
	if model.IsRootUser(trigger.UserId) {
		return nil
	}

	// Stable account/channel names at enqueue time; display names hydrate at
	// read time from the user profile.
	username := ""
	channelName := ""
	if user, err := model.GetUserById(trigger.UserId, false); err == nil && user != nil {
		username = user.Username
	}
	if trigger.ChannelId > 0 {
		if channel, err := model.GetChannelById(trigger.ChannelId, false); err == nil && channel != nil {
			channelName = channel.Name
		}
	}

	baseTask := &model.LLMReviewTask{
		UserId:         trigger.UserId,
		Username:       username,
		ModelName:      trigger.ModelName,
		ChannelId:      trigger.ChannelId,
		ChannelName:    channelName,
		Endpoint:       trigger.Endpoint,
		IsStream:       trigger.IsStream,
		TriggerType:    model.LLMReviewTriggerType(trigger.TriggerType),
		Stage:          model.LLMReviewStage(trigger.Stage),
		CurrentValue:   trigger.CurrentValue,
		LimitValue:     trigger.LimitValue,
		EstimateInput:  trigger.EstimateInput,
		ActualInput:    trigger.ActualInput,
		ActualOutput:   trigger.ActualOutput,
		RequestSnippet: trigger.RequestSnippet,
		Payload:        buildPayloadSnapshot(trigger, cfg),
		MaskedIP:       common.MaskIP(trigger.ClientIP),
	}

	// 1. Review disabled: record a skipped audit task, no reviewer calls.
	if !cfg.Enabled {
		if err := model.MarkLLMReviewTaskSkipped(baseTask, model.SkipReasonReviewDisabled); err != nil {
			common.SysLog(fmt.Sprintf("EnqueueLLMReview: record skipped (review_disabled) failed: %v", err))
		}
		return nil
	}

	// 2. Permanently disabled users: audit-only skipped record.
	if banned, _ := model.IsUserPermanentlyBanned(trigger.UserId); banned {
		if err := model.MarkLLMReviewTaskSkipped(baseTask, model.SkipReasonDisabledUser); err != nil {
			common.SysLog(fmt.Sprintf("EnqueueLLMReview: record skipped (skipped_disabled) failed: %v", err))
		}
		return nil
	}

	// 3. Grace period: record skipped, no reviewer call.
	if inGrace, _, err := model.CheckLLMReviewGrace(trigger.UserId, now); err == nil && inGrace {
		if err := model.MarkLLMReviewTaskSkipped(baseTask, model.SkipReasonGracePeriod); err != nil {
			common.SysLog(fmt.Sprintf("EnqueueLLMReview: record skipped (grace_period) failed: %v", err))
		}
		return nil
	}

	// 4. Atomic enqueue with merge fallback. A task that completes between
	// the enqueue result and the merge retries once so the event is never
	// silently dropped.
	task := &model.LLMReviewTask{
		UserId:             trigger.UserId,
		Username:           username,
		ModelName:          trigger.ModelName,
		ChannelId:          trigger.ChannelId,
		ChannelName:        channelName,
		Endpoint:           trigger.Endpoint,
		IsStream:           trigger.IsStream,
		TriggerType:        model.LLMReviewTriggerType(trigger.TriggerType),
		Stage:              model.LLMReviewStage(trigger.Stage),
		CurrentValue:       trigger.CurrentValue,
		LimitValue:         trigger.LimitValue,
		EstimateInput:      trigger.EstimateInput,
		ActualInput:        trigger.ActualInput,
		ActualOutput:       trigger.ActualOutput,
		RequestSnippet:     trigger.RequestSnippet,
		Payload:            buildPayloadSnapshot(trigger, cfg),
		MaskedIP:           common.MaskIP(trigger.ClientIP),
		TriggerRPM:         trigger.TriggerType == LLMReviewTriggerRPM,
		TriggerInputToken:  trigger.TriggerType == LLMReviewTriggerInputToken,
		TriggerOutputToken: trigger.TriggerType == LLMReviewTriggerOutputToken,
		MaxCurrentValue:    trigger.CurrentValue,
		LastTriggerAt:      now,
	}

	for attempt := 0; attempt < 2; attempt++ {
		candidate := *task
		candidate.ID = 0
		candidate.ReviewID = ""
		candidate.CreatedAt = 0
		candidate.UpdatedAt = 0
		activeTask, created, err := model.EnqueueLLMReviewTask(trigger.UserId, &candidate)
		if err != nil {
			common.SysLog(fmt.Sprintf("EnqueueLLMReview: enqueue failed: %v", err))
			return err
		}
		if created {
			common.SysLog(fmt.Sprintf("EnqueueLLMReview: task created reviewId=%s type=%s stage=%s userId=%d",
				activeTask.ReviewID, trigger.TriggerType, trigger.Stage, trigger.UserId))
			return nil
		}
		if activeTask == nil {
			return errors.New("active LLM review task missing after enqueue")
		}
		err = model.MergeLLMReviewTask(activeTask, model.LLMReviewTriggerType(trigger.TriggerType), trigger.CurrentValue, trigger.LimitValue)
		if err == nil {
			return nil
		}
		if !errors.Is(err, model.ErrLLMReviewTaskNoLongerActive) {
			common.SysLog(fmt.Sprintf("EnqueueLLMReview: merge failed for task %d: %v", activeTask.ID, err))
			return err
		}
	}
	return model.ErrLLMReviewTaskNoLongerActive
}
