package model

import (
	"database/sql/driver"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// LLMReviewTaskStatus is the review task state machine.
type LLMReviewTaskStatus string

const (
	LLMReviewTaskPending    LLMReviewTaskStatus = "pending"
	LLMReviewTaskReviewing  LLMReviewTaskStatus = "reviewing"
	LLMReviewTaskCompliant  LLMReviewTaskStatus = "compliant"
	LLMReviewTaskViolation  LLMReviewTaskStatus = "violation"
	LLMReviewTaskUncertain  LLMReviewTaskStatus = "uncertain"
	LLMReviewTaskSkipped    LLMReviewTaskStatus = "skipped"
	LLMReviewTaskFailed     LLMReviewTaskStatus = "failed"
	LLMReviewTaskSuperseded LLMReviewTaskStatus = "superseded"
)

// LLMReviewTriggerType is the trigger category.
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

// LLMReviewVerdict is the reviewer verdict.
type LLMReviewVerdict string

const (
	LLMReviewVerdictViolation LLMReviewVerdict = "violation"
	LLMReviewVerdictCompliant LLMReviewVerdict = "compliant"
	LLMReviewVerdictUncertain LLMReviewVerdict = "uncertain"
)

// LLMReviewCategory is the violation category.
type LLMReviewCategory string

const (
	LLMReviewCategoryNone LLMReviewCategory = "none"
)

// Skip / supersede reasons (kept separate from status for the admin view).
const (
	SkipReasonReviewDisabled = "review_disabled" // master switch off
	SkipReasonGracePeriod    = "grace_period"
	SkipReasonManualBan      = "manual_ban_override"   // later manual permanent disable
	SkipReasonManualUnban    = "manual_unban_override" // later manual re-enable
	SkipReasonDisabledUser   = "skipped_disabled"      // user already permanently disabled
)

// LLMReviewEvidence is the structured evidence list, persisted as JSON text
// for SQLite/MySQL/PostgreSQL compatibility.
type LLMReviewEvidence []string

// LLMReviewRawResponse is the masked raw reviewer response (text column).
type LLMReviewRawResponse string

// LLMReviewPayload is the sanitized payload snapshot submitted to the
// reviewer LLM. It never contains usernames, emails, API keys, auth headers,
// cookies, raw IPs or full request headers; the client IP appears only as an
// irreversible hash.
type LLMReviewPayload struct {
	PolicyID       string               `json:"policy_id"`
	PolicyText     string               `json:"policy_text"`
	PromptVersion  string               `json:"prompt_version"`
	SchemaVersion  string               `json:"schema_version"`
	RequestSnippet string               `json:"request_snippet"`
	TriggerType    LLMReviewTriggerType `json:"trigger_type"`
	Stage          LLMReviewStage       `json:"stage"`
	ModelName      string               `json:"model_name"`
	Endpoint       string               `json:"endpoint"`
	IsStream       bool                 `json:"is_stream"`
	CurrentValue   int                  `json:"current_value"`
	LimitValue     int                  `json:"limit_value"`
	RecentTriggers int                  `json:"recent_triggers"`
	EstimateInput  int                  `json:"estimate_input,omitempty"`
	ActualInput    int                  `json:"actual_input,omitempty"`
	ActualOutput   int                  `json:"actual_output,omitempty"`
	UserAgent      string               `json:"user_agent,omitempty"`
	ClientIPHash   string               `json:"client_ip_hash,omitempty"`
	AccountsOnIP   int                  `json:"accounts_on_ip,omitempty"`
	Aggregate      *LLMReviewAggregate  `json:"aggregate,omitempty"`
	Features       map[string]bool      `json:"features,omitempty"`
	Verdict        *LLMReviewVerdict    `json:"verdict,omitempty"`
	Categories     []LLMReviewCategory  `json:"categories,omitempty"`
	Confidence     float64              `json:"confidence,omitempty"`
	Reason         string               `json:"reason,omitempty"`
	Evidence       LLMReviewEvidence    `json:"evidence,omitempty"`
	Custom         map[string]any       `json:"custom,omitempty"`
}

// LLMReviewAggregate is a bounded recent-behavior aggregate; historical
// request bodies are never included.
type LLMReviewAggregate struct {
	RequestIntervals []int `json:"request_intervals,omitempty"` // seconds
	RPM              int   `json:"rpm,omitempty"`
	InputTokens      int   `json:"input_tokens,omitempty"`
	OutputTokens     int   `json:"output_tokens,omitempty"`
}

// LLMReviewTask is one persisted review task.
type LLMReviewTask struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`

	// ReviewID is unguessable (rev-<24 random chars>) so tasks cannot be
	// enumerated through the public API.
	ReviewID string `json:"review_id" gorm:"type:varchar(64);uniqueIndex"`

	UserId      int    `json:"user_id" gorm:"index"`
	Username    string `json:"username" gorm:"type:varchar(64);index"`
	DisplayName string `json:"display_name" gorm:"-"`

	ModelName         string `json:"model_name" gorm:"type:varchar(191);index"`
	ChannelId         int    `json:"channel_id" gorm:"index"`
	ChannelName       string `json:"channel_name" gorm:"type:varchar(128)"`
	ChannelAssignment string `json:"channel_assignment" gorm:"-"`
	RecentChannelId   int    `json:"recent_channel_id" gorm:"-"`
	RecentChannelName string `json:"recent_channel_name" gorm:"-"`
	RecentChannelAt   int64  `json:"recent_channel_at" gorm:"-"`
	Endpoint          string `json:"endpoint" gorm:"type:varchar(512)"`

	// MaskedIP stores only a partial mask; raw IPs are never persisted.
	MaskedIP string `json:"ip_masked" gorm:"type:varchar(64)"`

	TriggerType LLMReviewTriggerType `json:"trigger_type" gorm:"type:varchar(32);index"`
	Stage       LLMReviewStage       `json:"stage" gorm:"type:varchar(32);index"`

	CurrentValue  int  `json:"current_value"`
	LimitValue    int  `json:"limit_value"`
	EstimateInput int  `json:"estimate_input"`
	ActualInput   int  `json:"actual_input"`
	ActualOutput  int  `json:"actual_output"`
	IsStream      bool `json:"is_stream"`

	// RequestSnippet is the sanitized request summary; Payload is the exact
	// sanitized payload submitted to the reviewer.
	RequestSnippet string `json:"request_snippet" gorm:"type:text"`
	Payload        string `json:"payload" gorm:"type:text"`

	Status        LLMReviewTaskStatus `json:"status" gorm:"type:varchar(32);index"`
	Attempts      int                 `json:"attempts"`
	NextRetryAt   int64               `json:"next_retry_at" gorm:"bigint;index"`
	LastAttemptAt int64               `json:"last_attempt_at" gorm:"bigint"`
	StartedAt     int64               `json:"started_at" gorm:"bigint"`
	CompletedAt   int64               `json:"completed_at" gorm:"bigint"`
	ReviewerModel string              `json:"reviewer_model" gorm:"type:varchar(191)"`
	PolicyID      string              `json:"policy_id" gorm:"type:varchar(64)"`
	PromptVersion string              `json:"prompt_version" gorm:"type:varchar(64)"`
	SchemaVersion string              `json:"schema_version" gorm:"type:varchar(64)"`
	SchemaPassed  bool                `json:"schema_passed"`
	SchemaError   string              `json:"schema_error" gorm:"type:text"`

	Verdict    LLMReviewVerdict  `json:"verdict" gorm:"type:varchar(32)"`
	Category   LLMReviewCategory `json:"category" gorm:"type:varchar(64)"`
	Confidence float64           `json:"confidence" gorm:"type:double precision"`
	Reason     string            `json:"reason" gorm:"type:text"`
	Evidence   LLMReviewEvidence `json:"evidence" gorm:"type:text"`

	// RawResponse is the masked raw reviewer response (no API keys or full
	// headers).
	RawResponse   string `json:"raw_response" gorm:"type:text"`
	FailureReason string `json:"failure_reason" gorm:"type:text"`

	Banned       bool   `json:"banned"`
	BanMessage   string `json:"ban_message" gorm:"type:text"`
	BanError     string `json:"ban_error" gorm:"type:text"`
	SupersededBy string `json:"superseded_by" gorm:"type:varchar(64)"`
	SkipReason   string `json:"skip_reason" gorm:"type:varchar(128)"`

	// Merge event statistics. The trigger-type set is kept in three boolean
	// columns updated atomically with CASE WHEN so concurrent merges never
	// lose an update across SQLite/MySQL/PostgreSQL.
	MergeCount         int   `json:"merge_count"`
	TriggerRPM         bool  `json:"-"`
	TriggerInputToken  bool  `json:"-"`
	TriggerOutputToken bool  `json:"-"`
	MaxCurrentValue    int   `json:"max_current_value"`
	LastTriggerAt      int64 `json:"last_trigger_at" gorm:"bigint"`

	CreatedAt int64 `json:"created_at" gorm:"bigint;index"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

// TriggerTypesValue derives the comma-joined trigger-type set from the
// boolean columns.
func (t *LLMReviewTask) TriggerTypesValue() string {
	var parts []string
	if t.TriggerRPM {
		parts = append(parts, "rpm")
	}
	if t.TriggerInputToken {
		parts = append(parts, "input_token")
	}
	if t.TriggerOutputToken {
		parts = append(parts, "output_token")
	}
	return strings.Join(parts, ",")
}

// BeforeCreate fills timestamps.
func (t *LLMReviewTask) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	if t.UpdatedAt == 0 {
		t.UpdatedAt = now
	}
	return nil
}

// BeforeSave keeps updated_at current under concurrent updates.
func (t *LLMReviewTask) BeforeSave(_ *gorm.DB) error {
	t.UpdatedAt = common.GetTimestamp()
	return nil
}

// Active reports whether the task is pending or reviewing.
func (t *LLMReviewTask) Active() bool {
	return t.Status == LLMReviewTaskPending || t.Status == LLMReviewTaskReviewing
}

// LLMReviewAttempt is one reviewer call record.
type LLMReviewAttempt struct {
	ID         int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskId     int64  `json:"task_id" gorm:"index"`
	AttemptNo  int    `json:"attempt_no"`
	RequestAt  int64  `json:"request_at" gorm:"bigint"`
	DurationMs int64  `json:"duration_ms"`
	HTTPStatus int    `json:"http_status"`
	Response   string `json:"response" gorm:"type:text"` // masked
	ParseError string `json:"parse_error" gorm:"type:text"`
	Retryable  bool   `json:"retryable"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index"`
}

// BeforeCreate fills timestamps.
func (a *LLMReviewAttempt) BeforeCreate(_ *gorm.DB) error {
	if a.CreatedAt == 0 {
		a.CreatedAt = common.GetTimestamp()
	}
	return nil
}

// LLMReviewGrace is the per-user grace state plus manual ban/unban timestamps
// used to supersede stale review results.
type LLMReviewGrace struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	// UserId unique index: one row per user.
	UserId int `json:"user_id" gorm:"uniqueIndex"`

	// CompliantCount counts compliant verdicts (reset when the grace period
	// starts or expires).
	CompliantCount int `json:"compliant_count"`
	// GraceStartAt / GraceEndAt define the grace window (Unix seconds);
	// GraceEndAt==0 means not in a grace period.
	GraceStartAt int64 `json:"grace_start_at" gorm:"bigint"`
	GraceEndAt   int64 `json:"grace_end_at" gorm:"bigint"`
	// LastCompliantAt is the most recent compliant verdict time.
	LastCompliantAt int64 `json:"last_compliant_at" gorm:"bigint"`

	// LastManualBanAt: a later manual permanent disable supersedes pending
	// review results.
	LastManualBanAt int64 `json:"last_manual_ban_at" gorm:"bigint"`
	// LastManualUnbanAt: a later manual re-enable prevents re-auto-disable.
	LastManualUnbanAt int64 `json:"last_manual_unban_at" gorm:"bigint"`

	// ActiveTaskId is the current active task slot (0 = none). Claiming is a
	// conditional UPDATE (WHERE user_id=? AND active_task_id=0), which is
	// atomic across SQLite/MySQL/PostgreSQL.
	ActiveTaskId int64 `json:"active_task_id" gorm:"bigint;index"`

	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

// BeforeSave keeps updated_at current.
func (g *LLMReviewGrace) BeforeSave(_ *gorm.DB) error {
	g.UpdatedAt = common.GetTimestamp()
	return nil
}

// InGracePeriod reports whether now falls inside the grace window.
func (g *LLMReviewGrace) InGracePeriod(now int64) bool {
	return g.GraceEndAt > 0 && now < g.GraceEndAt
}

// LLMReviewCalibration is one input-token estimator calibration sample.
type LLMReviewCalibration struct {
	ID               int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	ModelName        string  `json:"model_name" gorm:"type:varchar(191);index"`
	EstimateTokens   int     `json:"estimate_tokens"`
	ActualTokens     int     `json:"actual_tokens"`
	LimitValue       int     `json:"limit_value"` // per-request input limit at sample time
	RelativeError    float64 `json:"relative_error" gorm:"type:double precision"`
	NearThreshold    bool    `json:"near_threshold"`
	EstimatorVersion string  `json:"estimator_version" gorm:"type:varchar(64)"`
	SampleTime       int64   `json:"sample_time" gorm:"bigint;index"`
	ModelPassed      bool    `json:"model_passed"`
}

// BeforeCreate fills timestamps.
func (c *LLMReviewCalibration) BeforeCreate(_ *gorm.DB) error {
	if c.SampleTime == 0 {
		c.SampleTime = common.GetTimestamp()
	}
	return nil
}

// Scan/Value persist LLMReviewEvidence as JSON text (all three databases).
func (e *LLMReviewEvidence) Scan(value any) error {
	if value == nil {
		*e = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*e = nil
			return nil
		}
		return common.Unmarshal(v, e)
	case string:
		if v == "" {
			*e = nil
			return nil
		}
		return common.UnmarshalJsonStr(v, e)
	default:
		return errors.New("unsupported evidence type")
	}
}

// Value serializes LLMReviewEvidence as JSON text.
func (e LLMReviewEvidence) Value() (driver.Value, error) {
	if len(e) == 0 {
		return nil, nil
	}
	data, err := common.Marshal(e)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}
