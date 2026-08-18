package controller

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LLMReviewConfigResponse is the review configuration response. The API key is
// only ever returned as a tail-derived mask.
type LLMReviewConfigResponse struct {
	Enabled                     bool    `json:"enabled"`
	BaseURL                     string  `json:"base_url"`
	APIKey                      string  `json:"api_key"`
	ModelName                   string  `json:"model"`
	PolicyText                  string  `json:"policy_text"`
	TimeoutSeconds              int     `json:"timeout_seconds"`
	MaxAttempts                 int     `json:"max_attempts"`
	RetryIntervalSeconds        int     `json:"retry_interval_seconds"`
	WorkerConcurrency           int     `json:"worker_concurrency"`
	ConfidenceThreshold         float64 `json:"confidence_threshold"`
	CompliantLimit              int     `json:"compliant_limit"`
	ImmuneHours                 int     `json:"immune_hours"`
	RetentionDays               int     `json:"retention_days"`
	MaxOutputTokens             int     `json:"max_output_tokens"`
	AllowPrivateURL             bool    `json:"allow_private_url"`
	SchemaTested                bool    `json:"schema_tested"`
	StructuredOutputMode        string  `json:"structured_output_mode"`
	StructuredOutputTested      bool    `json:"structured_output_tested"`
	StructuredOutputTestedAt    int64   `json:"structured_output_tested_at"`
	StructuredOutputTestedModel string  `json:"structured_output_tested_model"`
	StructuredOutputVersion     string  `json:"structured_output_version"`
	PolicyConfigured            bool    `json:"policy_configured"`
	ReadinessReady              bool    `json:"readiness_ready"`
	ReadinessReason             string  `json:"readiness_reason"`
}

// LLMReviewConfigUpdateRequest updates the review configuration. An empty or
// masked api_key leaves the stored key untouched.
type LLMReviewConfigUpdateRequest struct {
	Enabled              *bool    `json:"enabled"`
	BaseURL              string   `json:"base_url"`
	APIKey               string   `json:"api_key"`
	ModelName            string   `json:"model"`
	PolicyText           *string  `json:"policy_text"`
	StructuredOutputMode *string  `json:"structured_output_mode"`
	TimeoutSeconds       *int     `json:"timeout_seconds"`
	MaxAttempts          *int     `json:"max_attempts"`
	RetryIntervalSeconds *int     `json:"retry_interval_seconds"`
	WorkerConcurrency    *int     `json:"worker_concurrency"`
	ConfidenceThreshold  *float64 `json:"confidence_threshold"`
	CompliantLimit       *int     `json:"compliant_limit"`
	ImmuneHours          *int     `json:"immune_hours"`
	RetentionDays        *int     `json:"retention_days"`
	MaxOutputTokens      *int     `json:"max_output_tokens"`
	AllowPrivateURL      *bool    `json:"allow_private_url"`
}

// llmReviewSchemaTestMu serializes capability probes with critical review
// configuration changes in the same process, preventing stale probe results
// from restoring invalidated capability state.
var llmReviewSchemaTestMu sync.Mutex

// GetLLMReviewConfig returns the review configuration (root).
func GetLLMReviewConfig(c *gin.Context) {
	cfg := operation_setting.GetLLMReviewSetting()
	readiness := operation_setting.GetReviewReadiness(cfg)
	structuredTestedAt := cfg.StructuredOutputTestedAt
	structuredTestedModel := cfg.StructuredOutputTestedModel
	structuredVersion := cfg.StructuredOutputVersion
	if !cfg.StructuredOutputTested && cfg.SchemaTested {
		// Older installations only persisted the strict capability fields.
		structuredTestedAt = cfg.SchemaTestedAt
		structuredTestedModel = cfg.SchemaTestedModel
		structuredVersion = cfg.SchemaVersion
	}
	maskedKey := ""
	if cfg.APIKeyEncrypted != "" {
		if plain, err := service.DecryptLLMReviewAPIKey(cfg.APIKeyEncrypted); err == nil {
			maskedKey = operation_setting.MaskAPIKey(plain)
		}
	}
	resp := LLMReviewConfigResponse{
		Enabled:                     cfg.Enabled,
		BaseURL:                     cfg.BaseURL,
		APIKey:                      maskedKey,
		ModelName:                   cfg.ModelName,
		PolicyText:                  cfg.PolicyText,
		TimeoutSeconds:              cfg.TimeoutSeconds,
		MaxAttempts:                 cfg.MaxAttempts,
		RetryIntervalSeconds:        cfg.RetryIntervalSeconds,
		WorkerConcurrency:           cfg.WorkerConcurrency,
		ConfidenceThreshold:         cfg.AutoBanConfidence,
		CompliantLimit:              cfg.MaxCompliantCount,
		ImmuneHours:                 cfg.GracePeriodHours,
		RetentionDays:               cfg.LogRetentionDays,
		MaxOutputTokens:             cfg.MaxOutputTokens,
		AllowPrivateURL:             cfg.AllowPrivateAddress,
		SchemaTested:                cfg.SchemaTested,
		StructuredOutputMode:        operation_setting.EffectiveStructuredOutputMode(cfg),
		StructuredOutputTested:      cfg.StructuredOutputTested || cfg.SchemaTested,
		StructuredOutputTestedAt:    structuredTestedAt,
		StructuredOutputTestedModel: structuredTestedModel,
		StructuredOutputVersion:     structuredVersion,
		PolicyConfigured:            readiness.PolicyConfigured,
		ReadinessReady:              readiness.Ready,
		ReadinessReason:             readiness.Reason,
	}
	common.ApiSuccess(c, resp)
}

// UpdateLLMReviewConfig updates the review configuration (root, dedicated
// endpoint). Critical config changes reset the structured-output capability
// state, and enabling requires a fully configured service whose current
// critical config passed a structured-output capability test.
func UpdateLLMReviewConfig(c *gin.Context) {
	var req LLMReviewConfigUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, "common.invalid_params")
		return
	}
	if !llmReviewSchemaTestMu.TryLock() {
		common.ApiErrorMsg(c, "structured output test already in progress")
		return
	}
	defer llmReviewSchemaTestMu.Unlock()

	cfg := operation_setting.GetLLMReviewSetting()

	updates := map[string]string{}
	schemaInvalidated := false
	candidate := *cfg
	if req.Enabled != nil {
		candidate.Enabled = *req.Enabled
	}

	if req.AllowPrivateURL != nil {
		candidate.AllowPrivateAddress = *req.AllowPrivateURL
		if candidate.AllowPrivateAddress != cfg.AllowPrivateAddress {
			updates["llm_review_setting.allow_private_url"] = common.Interface2String(*req.AllowPrivateURL)
			schemaInvalidated = true
		}
	}
	if req.BaseURL != "" {
		if err := service.ValidateReviewBaseURL(req.BaseURL, candidate.AllowPrivateAddress); err != nil {
			common.ApiErrorMsg(c, "invalid base_url: "+sanitizeReviewError(err))
			return
		}
		candidate.BaseURL = req.BaseURL
		if candidate.BaseURL != cfg.BaseURL {
			updates["llm_review_setting.base_url"] = req.BaseURL
			schemaInvalidated = true
		}
	}
	if req.APIKey != "" && !isMaskedAPIKey(req.APIKey) {
		candidate.APIKeyEncrypted = req.APIKey
		updates["llm_review_setting.api_key"] = req.APIKey
		schemaInvalidated = true
	}
	if req.ModelName != "" {
		candidate.ModelName = req.ModelName
		if candidate.ModelName != cfg.ModelName {
			updates["llm_review_setting.model"] = req.ModelName
			schemaInvalidated = true
		}
	}
	if req.PolicyText != nil {
		candidate.PolicyText = *req.PolicyText
		updates["llm_review_setting.policy_text"] = *req.PolicyText
	}
	if req.StructuredOutputMode != nil {
		mode := *req.StructuredOutputMode
		if mode != operation_setting.StructuredOutputModeStrictSchema && mode != operation_setting.StructuredOutputModeJSONObject && mode != operation_setting.StructuredOutputModePromptJSON {
			common.ApiErrorMsg(c, "unsupported structured output mode")
			return
		}
		candidate.StructuredOutputMode = mode
		if mode != operation_setting.EffectiveStructuredOutputMode(cfg) {
			updates["llm_review_setting.structured_output_mode"] = mode
			schemaInvalidated = true
		} else if cfg.StructuredOutputMode == "" {
			updates["llm_review_setting.structured_output_mode"] = mode
		}
	}
	if req.TimeoutSeconds != nil {
		candidate.TimeoutSeconds = *req.TimeoutSeconds
		updates["llm_review_setting.timeout_seconds"] = strconv.Itoa(*req.TimeoutSeconds)
	}
	if req.MaxAttempts != nil {
		if !operation_setting.IsValidMaxAttempts(*req.MaxAttempts) {
			common.ApiErrorMsg(c, "max_attempts must be >= 1")
			return
		}
		candidate.MaxAttempts = *req.MaxAttempts
		updates["llm_review_setting.max_attempts"] = strconv.Itoa(*req.MaxAttempts)
	}
	if req.RetryIntervalSeconds != nil {
		candidate.RetryIntervalSeconds = *req.RetryIntervalSeconds
		updates["llm_review_setting.retry_interval_seconds"] = strconv.Itoa(*req.RetryIntervalSeconds)
	}
	if req.WorkerConcurrency != nil {
		candidate.WorkerConcurrency = operation_setting.ClampWorkerConcurrency(*req.WorkerConcurrency)
		updates["llm_review_setting.worker_concurrency"] = strconv.Itoa(candidate.WorkerConcurrency)
	}
	if req.ConfidenceThreshold != nil {
		candidate.AutoBanConfidence = *req.ConfidenceThreshold
		updates["llm_review_setting.confidence_threshold"] = common.Interface2String(*req.ConfidenceThreshold)
	}
	if req.CompliantLimit != nil {
		candidate.MaxCompliantCount = *req.CompliantLimit
		updates["llm_review_setting.compliant_limit"] = strconv.Itoa(*req.CompliantLimit)
	}
	if req.ImmuneHours != nil {
		candidate.GracePeriodHours = *req.ImmuneHours
		updates["llm_review_setting.immune_hours"] = strconv.Itoa(*req.ImmuneHours)
	}
	if req.RetentionDays != nil {
		candidate.LogRetentionDays = *req.RetentionDays
		updates["llm_review_setting.retention_days"] = strconv.Itoa(*req.RetentionDays)
	}
	if req.MaxOutputTokens != nil {
		candidate.MaxOutputTokens = *req.MaxOutputTokens
		updates["llm_review_setting.max_output_tokens"] = strconv.Itoa(*req.MaxOutputTokens)
	}

	// Enable validation is intentionally ordered from basic configuration to
	// policy to capability so the administrator gets the most actionable setup
	// error, while an explicit empty policy is still distinguishable from an
	// omitted field.
	if req.Enabled != nil && *req.Enabled {
		if !operation_setting.IsReviewConfigured(&candidate) {
			common.ApiErrorMsg(c, "review service is not fully configured: base_url and model are required")
			return
		}
		if !operation_setting.IsPolicyConfigured(&candidate) {
			common.ApiErrorMsg(c, "policy text is required while review is enabled")
			return
		}
		if schemaInvalidated {
			common.ApiErrorMsg(c, "review service is not ready: structured-output capability test is required after configuration changes")
			return
		}
		if !operation_setting.CanEnableReview(&candidate) {
			readiness := operation_setting.GetReviewReadiness(&candidate)
			common.ApiErrorMsg(c, "review service is not ready: "+readiness.Reason)
			return
		}
	}
	if req.Enabled != nil {
		updates["llm_review_setting.enabled"] = common.Interface2String(*req.Enabled)
	}
	if candidate.Enabled && !operation_setting.IsPolicyConfigured(&candidate) {
		common.ApiErrorMsg(c, "policy text is required while review is enabled")
		return
	}

	if len(updates) == 0 {
		common.ApiSuccess(c, nil)
		return
	}
	if schemaInvalidated {
		updates["llm_review_setting.schema_tested"] = "false"
		updates["llm_review_setting.schema_tested_at"] = "0"
		updates["llm_review_setting.schema_tested_model"] = ""
		updates["llm_review_setting.schema_version"] = ""
		updates["llm_review_setting.structured_output_tested"] = "false"
		updates["llm_review_setting.structured_output_tested_at"] = "0"
		updates["llm_review_setting.structured_output_tested_model"] = ""
		updates["llm_review_setting.structured_output_version"] = ""
		updates["llm_review_setting.test_error"] = ""
	}

	if err := service.SaveReviewSetting(updates); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": "llm_review_setting",
	})
	common.ApiSuccess(c, nil)
}

// isMaskedAPIKey reports whether the submitted key is already a mask, which
// means "keep the stored key".
func isMaskedAPIKey(key string) bool {
	trimmed := strings.TrimSpace(key)
	return trimmed == "****" || (strings.Contains(trimmed, "****") && !strings.ContainsAny(trimmed, "\r\n\t "))
}

// reviewCandidateRequest carries unsaved candidate config for connection /
// schema capability tests.
type reviewCandidateRequest struct {
	BaseURL              string  `json:"base_url"`
	APIKey               string  `json:"api_key"`
	ModelName            string  `json:"model"`
	StructuredOutputMode *string `json:"structured_output_mode"`
	TimeoutSeconds       *int    `json:"timeout_seconds"`
	AllowPrivateURL      *bool   `json:"allow_private_url"`
}

// applyReviewCandidate overlays the candidate request onto the stored config
// for in-memory test use. A candidate API key is encrypted for the in-memory
// struct only and never persisted here.
func applyReviewCandidate(cfg *operation_setting.LLMReviewSetting, req reviewCandidateRequest) (*operation_setting.LLMReviewSetting, error) {
	tmp := *cfg
	criticalConfigChanged := false
	if req.BaseURL != "" {
		criticalConfigChanged = criticalConfigChanged || tmp.BaseURL != req.BaseURL
		tmp.BaseURL = req.BaseURL
	}
	if req.ModelName != "" {
		criticalConfigChanged = criticalConfigChanged || tmp.ModelName != req.ModelName
		tmp.ModelName = req.ModelName
	}
	if req.StructuredOutputMode != nil {
		criticalConfigChanged = criticalConfigChanged || tmp.StructuredOutputMode != *req.StructuredOutputMode
		tmp.StructuredOutputMode = *req.StructuredOutputMode
	}
	if req.APIKey != "" && !isMaskedAPIKey(req.APIKey) {
		enc, err := service.EncryptLLMReviewAPIKey(req.APIKey)
		if err != nil {
			return nil, err
		}
		tmp.APIKeyEncrypted = enc
		criticalConfigChanged = true
	}
	if req.TimeoutSeconds != nil {
		tmp.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.AllowPrivateURL != nil {
		criticalConfigChanged = criticalConfigChanged || tmp.AllowPrivateAddress != *req.AllowPrivateURL
		tmp.AllowPrivateAddress = *req.AllowPrivateURL
	}
	if criticalConfigChanged {
		tmp.SchemaTested = false
		tmp.SchemaTestedAt = 0
		tmp.SchemaTestedModel = ""
		tmp.SchemaVersion = ""
		tmp.StructuredOutputTested = false
		tmp.StructuredOutputTestedAt = 0
		tmp.StructuredOutputTestedModel = ""
		tmp.StructuredOutputVersion = ""
		tmp.TestError = ""
	}
	return &tmp, nil
}

// TestLLMReviewConnection tests connectivity (root). Temporary candidate keys
// stay in memory and are never persisted or logged.
func TestLLMReviewConnection(c *gin.Context) {
	var req reviewCandidateRequest
	_ = common.DecodeJson(c.Request.Body, &req)

	cfg := operation_setting.GetLLMReviewSetting()
	tmp, err := applyReviewCandidate(cfg, req)
	if err != nil {
		common.ApiErrorMsg(c, "failed to process api key: "+sanitizeReviewError(err))
		return
	}
	if err := service.ValidateReviewBaseURL(tmp.BaseURL, tmp.AllowPrivateAddress); err != nil {
		common.ApiErrorMsg(c, "test connection failed: "+sanitizeReviewError(err))
		return
	}
	client := service.NewReviewClient(tmp)
	result, err := client.TestConnection(context.Background())
	if err != nil {
		common.ApiErrorMsg(c, "test connection failed: "+sanitizeReviewError(err))
		return
	}
	common.ApiSuccess(c, result)
}

// TestLLMReviewSchema probes structured-output modes in order (root). A
// passing test persists the tested critical config and selected mode; a failing
// test persists the masked error.
func TestLLMReviewSchema(c *gin.Context) {
	if !llmReviewSchemaTestMu.TryLock() {
		common.ApiErrorMsg(c, "structured output test already in progress")
		return
	}
	defer llmReviewSchemaTestMu.Unlock()

	var req reviewCandidateRequest
	_ = common.DecodeJson(c.Request.Body, &req)

	cfg := operation_setting.GetLLMReviewSetting()
	tmp, err := applyReviewCandidate(cfg, req)
	if err != nil {
		common.ApiErrorMsg(c, "failed to process api key: "+sanitizeReviewError(err))
		return
	}
	if !operation_setting.IsReviewConfigured(tmp) {
		common.ApiErrorMsg(c, "review service is not fully configured: base_url and model are required")
		return
	}
	if err := service.ValidateReviewBaseURL(tmp.BaseURL, tmp.AllowPrivateAddress); err != nil {
		message := sanitizeReviewError(err)
		if saveErr := service.SaveReviewSchemaTestFailure(message); saveErr != nil {
			common.ApiError(c, saveErr)
			return
		}
		common.ApiErrorMsg(c, "schema test failed: "+message)
		return
	}
	client := service.NewReviewClient(tmp)
	capability, err := client.TestStructuredOutputCapability(context.Background())
	if err != nil {
		message := sanitizeReviewError(err)
		if saveErr := service.SaveReviewSchemaTestFailure(message); saveErr != nil {
			common.ApiError(c, saveErr)
			return
		}
		common.ApiErrorMsg(c, "structured output test failed: "+message)
		return
	}
	if !capability.Passed {
		message := capability.Error
		if strings.TrimSpace(message) == "" {
			message = "no supported structured-output mode"
		}
		message = sanitizeReviewError(errors.New(message))
		if saveErr := service.SaveReviewSchemaTestFailure(message); saveErr != nil {
			common.ApiError(c, saveErr)
			return
		}
		common.ApiErrorMsg(c, "structured output test not passed: "+message)
		return
	}

	// Persist the tested critical config so "current critical config passed"
	// holds after the test.
	updates := map[string]string{}
	if tmp.BaseURL != cfg.BaseURL {
		updates["llm_review_setting.base_url"] = tmp.BaseURL
	}
	if tmp.ModelName != cfg.ModelName {
		updates["llm_review_setting.model"] = tmp.ModelName
	}
	if req.APIKey != "" && !isMaskedAPIKey(req.APIKey) {
		updates["llm_review_setting.api_key"] = req.APIKey
	}
	if tmp.AllowPrivateAddress != cfg.AllowPrivateAddress {
		updates["llm_review_setting.allow_private_url"] = common.Interface2String(tmp.AllowPrivateAddress)
	}
	if tmp.StructuredOutputMode != "" && tmp.StructuredOutputMode != cfg.StructuredOutputMode {
		updates["llm_review_setting.structured_output_mode"] = tmp.StructuredOutputMode
	}
	if len(updates) > 0 {
		if err := service.SaveReviewSetting(updates); err != nil {
			common.ApiError(c, err)
			return
		}
	}

	operation_setting.MarkStructuredOutputTested(capability.Mode, tmp.ModelName, service.ReviewSchemaVersion)
	current := operation_setting.GetLLMReviewSetting()
	updates = map[string]string{
		"llm_review_setting.structured_output_mode":         capability.Mode,
		"llm_review_setting.structured_output_tested":       "true",
		"llm_review_setting.structured_output_tested_at":    strconv.FormatInt(current.StructuredOutputTestedAt, 10),
		"llm_review_setting.structured_output_tested_model": tmp.ModelName,
		"llm_review_setting.structured_output_version":      current.StructuredOutputVersion,
		"llm_review_setting.test_error":                     "",
	}
	if capability.Mode == operation_setting.StructuredOutputModeStrictSchema {
		updates["llm_review_setting.schema_tested"] = "true"
		updates["llm_review_setting.schema_tested_at"] = strconv.FormatInt(current.SchemaTestedAt, 10)
		updates["llm_review_setting.schema_tested_model"] = tmp.ModelName
		updates["llm_review_setting.schema_version"] = current.SchemaVersion
	} else {
		updates["llm_review_setting.schema_tested"] = "false"
		updates["llm_review_setting.schema_tested_at"] = "0"
		updates["llm_review_setting.schema_tested_model"] = ""
		updates["llm_review_setting.schema_version"] = ""
	}
	if err := service.SaveReviewSetting(updates); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"ok":                       true,
		"schema_tested":            capability.Mode == operation_setting.StructuredOutputModeStrictSchema,
		"structured_output_tested": true,
		"structured_output_mode":   capability.Mode,
		"model":                    tmp.ModelName,
	})
}

// GetLLMReviewSchemaStatus returns the capability test status (root).
func GetLLMReviewSchemaStatus(c *gin.Context) {
	cfg := operation_setting.GetLLMReviewSetting()
	readiness := operation_setting.GetReviewReadiness(cfg)
	structuredTestedAt := cfg.StructuredOutputTestedAt
	structuredTestedModel := cfg.StructuredOutputTestedModel
	structuredVersion := cfg.StructuredOutputVersion
	if !cfg.StructuredOutputTested && cfg.SchemaTested {
		structuredTestedAt = cfg.SchemaTestedAt
		structuredTestedModel = cfg.SchemaTestedModel
		structuredVersion = cfg.SchemaVersion
	}
	common.ApiSuccess(c, gin.H{
		"status":                         operation_setting.SchemaTestStatus(cfg),
		"tested":                         cfg.SchemaTested,
		"supports_strict_json_schema":    cfg.SchemaTested,
		"tested_at":                      cfg.SchemaTestedAt,
		"tested_model":                   cfg.SchemaTestedModel,
		"schema_version":                 cfg.SchemaVersion,
		"structured_output_mode":         readiness.Mode,
		"structured_output_tested":       readiness.CapabilityTested,
		"structured_output_tested_at":    structuredTestedAt,
		"structured_output_tested_model": structuredTestedModel,
		"structured_output_version":      structuredVersion,
		"policy_configured":              readiness.PolicyConfigured,
		"ready":                          readiness.Ready,
		"readiness_reason":               readiness.Reason,
		"error":                          cfg.TestError,
	})
}

// ClearLLMReviewAPIKey clears the stored review API key (root) and resets the
// capability state.
func ClearLLMReviewAPIKey(c *gin.Context) {
	if !llmReviewSchemaTestMu.TryLock() {
		common.ApiErrorMsg(c, "structured output test already in progress")
		return
	}
	defer llmReviewSchemaTestMu.Unlock()

	updates := map[string]string{
		"llm_review_setting.api_key":                        "",
		"llm_review_setting.schema_tested":                  "false",
		"llm_review_setting.schema_tested_at":               "0",
		"llm_review_setting.schema_tested_model":            "",
		"llm_review_setting.schema_version":                 "",
		"llm_review_setting.structured_output_tested":       "false",
		"llm_review_setting.structured_output_tested_at":    "0",
		"llm_review_setting.structured_output_tested_model": "",
		"llm_review_setting.structured_output_version":      "",
		"llm_review_setting.test_error":                     "",
	}
	if err := service.SaveReviewSetting(updates); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": "llm_review_setting.api_key",
	})
	common.ApiSuccess(c, nil)
}

// ListLLMReviewTasks pages review tasks. Permission enforced by the router.
func ListLLMReviewTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	status := c.Query("status")
	modelName := c.Query("model_name")
	triggerType := c.Query("trigger_type")
	category := c.Query("category")
	username := c.Query("username")
	keyword := c.Query("keyword")
	userId, _ := strconv.Atoi(c.Query("user_id"))
	startTime, _ := strconv.ParseInt(c.Query("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("end_time"), 10, 64)

	tasks, total, err := model.ListLLMReviewTasks(page, pageSize, status, userId, modelName, triggerType, category, username, keyword, startTime, endTime)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, llmReviewTaskDetail(task, nil))
	}
	common.ApiSuccess(c, gin.H{
		"data":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// llmReviewTaskDetail assembles the frontend contract. attempts==nil renders
// list fields only.
func llmReviewTaskDetail(task *model.LLMReviewTask, attempts []*model.LLMReviewAttempt) gin.H {
	h := gin.H{
		"id":                     task.ID,
		"review_no":              task.ReviewID,
		"user_id":                task.UserId,
		"username":               task.Username,
		"display_name":           task.DisplayName,
		"status":                 task.Status,
		"trigger_type":           task.TriggerType,
		"trigger_types":          task.TriggerTypesValue(),
		"trigger_stage":          task.Stage,
		"model_name":             task.ModelName,
		"channel_id":             task.ChannelId,
		"channel_name":           task.ChannelName,
		"channel_assignment":     task.ChannelAssignment,
		"recent_channel_id":      task.RecentChannelId,
		"recent_channel_name":    task.RecentChannelName,
		"recent_channel_at":      task.RecentChannelAt,
		"api_endpoint":           task.Endpoint,
		"current_value":          task.CurrentValue,
		"limit_value":            task.LimitValue,
		"estimated_input_tokens": task.EstimateInput,
		"actual_input_tokens":    task.ActualInput,
		"actual_output_tokens":   task.ActualOutput,
		"verdict":                task.Verdict,
		"category":               task.Category,
		"confidence":             task.Confidence,
		"short_reason":           task.Reason,
		"banned":                 task.Banned,
		"ban_error":              task.BanError,
		"merged_event_count":     task.MergeCount,
		"ip_masked":              task.MaskedIP,
		"next_retry_at":          task.NextRetryAt,
		"created_at":             task.CreatedAt,
		"started_at":             task.StartedAt,
		"finished_at":            task.CompletedAt,
	}
	if attempts == nil {
		return h
	}
	h["request_summary"] = task.RequestSnippet
	h["review_payload"] = task.Payload
	h["evidence"] = task.Evidence
	h["raw_response"] = task.RawResponse
	h["skip_reason"] = task.SkipReason
	h["failure_reason"] = task.FailureReason
	h["schema_valid"] = task.SchemaPassed
	h["schema_error"] = task.SchemaError
	h["review_model"] = task.ReviewerModel
	h["output_mode"] = task.OutputMode
	h["policy_id"] = task.PolicyID
	h["prompt_template_version"] = task.PromptVersion
	h["schema_version"] = task.SchemaVersion
	h["human_override"] = task.SupersededBy
	h["retryable"] = model.IsLLMReviewTaskRetryable(task)
	h["attempts"] = attempts
	return h
}

// GetLLMReviewTaskDetail returns full task details with attempts. The router
// enforces the llm_review.read permission.
func GetLLMReviewTaskDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, "common.invalid_params")
		return
	}
	task, err := model.GetLLMReviewTask(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "task not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	attempts, err := model.ListLLMReviewAttempts(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "llm_review.read", map[string]interface{}{"task_id": id})
	common.ApiSuccess(c, llmReviewTaskDetail(task, attempts))
}

// RetryLLMReviewTask re-queues a failed/uncertain task or a recoverable
// skipped task. The router enforces the llm_review.read permission.
func RetryLLMReviewTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, "common.invalid_params")
		return
	}
	if err := model.RetryLLMReviewTask(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "llm_review.retry", map[string]interface{}{
		"task_id": id,
	})
	common.ApiSuccess(c, nil)
}

// GetLLMReviewQueueSummary returns the queue overview.
func GetLLMReviewQueueSummary(c *gin.Context) {
	summary, err := model.GetLLMReviewQueueSummary()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

// GetLLMReviewGrace returns the user's grace state.
func GetLLMReviewGrace(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Query("user_id"))
	if userId <= 0 {
		common.ApiErrorI18n(c, "common.invalid_params")
		return
	}
	grace, err := model.GetLLMReviewGrace(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, grace)
}

// CreateLLMReviewTaskRequest is the manual task creation payload.
type CreateLLMReviewTaskRequest struct {
	UserId         int    `json:"user_id" binding:"required"`
	ModelName      string `json:"model_name"`
	ChannelId      int    `json:"channel_id"`
	Endpoint       string `json:"endpoint"`
	IsStream       bool   `json:"is_stream"`
	TriggerType    string `json:"trigger_type" binding:"required"`
	Stage          string `json:"stage"`
	CurrentValue   int    `json:"current_value"`
	LimitValue     int    `json:"limit_value"`
	EstimateInput  int    `json:"estimate_input"`
	ActualInput    int    `json:"actual_input"`
	ActualOutput   int    `json:"actual_output"`
	RequestSnippet string `json:"request_snippet"`
}

// CreateLLMReviewTask manually enqueues a review task. The router enforces
// the llm_review.read permission.
func CreateLLMReviewTask(c *gin.Context) {
	var req CreateLLMReviewTaskRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, "common.invalid_params")
		return
	}
	trigger := service.LLMReviewTrigger{
		UserId:         req.UserId,
		ModelName:      req.ModelName,
		ChannelId:      req.ChannelId,
		Endpoint:       req.Endpoint,
		IsStream:       req.IsStream,
		TriggerType:    service.LLMReviewTriggerType(req.TriggerType),
		Stage:          service.LLMReviewStage(req.Stage),
		CurrentValue:   req.CurrentValue,
		LimitValue:     req.LimitValue,
		EstimateInput:  req.EstimateInput,
		ActualInput:    req.ActualInput,
		ActualOutput:   req.ActualOutput,
		RequestSnippet: req.RequestSnippet,
	}
	if err := service.EnqueueLLMReview(context.Background(), trigger); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "llm_review.create", map[string]interface{}{
		"user_id": req.UserId,
	})
	common.ApiSuccess(c, nil)
}

// sanitizeReviewError masks credentials in reviewer-facing errors before they
// reach API responses.
func sanitizeReviewError(err error) string {
	if err == nil {
		return ""
	}
	return common.MaskReviewCredentialText(common.MaskSensitiveInfo(err.Error()))
}
