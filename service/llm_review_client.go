package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// LLMReviewCallResult is one reviewer call outcome. Body is already masked
// (no API key or sensitive headers).
type LLMReviewCallResult struct {
	HTTPStatus int
	DurationMs int64
	Body       []byte
	Error      error
}

// ReviewClient is a standalone OpenAI-compatible reviewer client.
type ReviewClient struct {
	cfg *operation_setting.LLMReviewSetting
	hc  *http.Client
}

// NewReviewClient builds the client. No network traffic happens here.
func NewReviewClient(cfg *operation_setting.LLMReviewSetting) *ReviewClient {
	hc := &http.Client{
		Transport: newReviewTransport(cfg),
		// Redirects are forbidden: they would bypass the dial-time SSRF
		// validation.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New("redirects are not allowed for review client")
		},
	}
	return &ReviewClient{cfg: cfg, hc: hc}
}

// reviewDialContext validates every resolved IP at dial time to defeat DNS
// rebinding.
func reviewDialContext(cfg *operation_setting.LLMReviewSetting) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("review dial dns resolution failed: %w", err)
		}
		if len(ips) == 0 {
			return nil, errors.New("review dial dns resolution returned no addresses")
		}
		for _, ip := range ips {
			if !isReviewIPAllowed(ip.IP, cfg.AllowPrivateAddress) {
				return nil, fmt.Errorf("review dial blocked: %s resolves to disallowed address %s", host, ip.IP.String())
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

// isReviewIPAllowed applies the review transport IP policy. Cloud metadata
// addresses stay blocked even when private addresses are enabled.
func isReviewIPAllowed(ip net.IP, allowPrivate bool) bool {
	if ip == nil {
		return false
	}
	// Cloud metadata addresses are always blocked.
	if isCloudMetadataIP(ip) {
		return false
	}
	if isLoopbackOrLinkLocal(ip) {
		return allowPrivate
	}
	if ip.IsPrivate() {
		return allowPrivate
	}
	if ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	return true
}

// isCloudMetadataIP detects cloud metadata / link-local addresses
// (169.254.169.254 and fe80::/10).
func isCloudMetadataIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		return v4.IsLinkLocalUnicast()
	}
	return ip.IsLinkLocalUnicast()
}

// isLoopbackOrLinkLocal detects loopback and link-local addresses.
func isLoopbackOrLinkLocal(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast()
}

// newReviewTransport builds the SSRF-guarded transport. Keep-alives are
// disabled to shrink the connection-reuse bypass surface.
func newReviewTransport(cfg *operation_setting.LLMReviewSetting) *http.Transport {
	return &http.Transport{
		DialContext:         reviewDialContext(cfg),
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   true,
		ForceAttemptHTTP2:   false,
		TLSHandshakeTimeout: 15 * time.Second,
	}
}

// reviewEndpoint builds the chat completions endpoint, accepting base URLs
// with or without a trailing /v1.
func reviewEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed + "/chat/completions"
	}
	return trimmed + "/v1/chat/completions"
}

// ValidateReviewBaseURL applies the SSRF constraints to a configured base
// URL: http/https only, loopback/link-local/private/cloud metadata blocked by
// default, private addresses behind an explicit advanced switch.
//
// Domain names are NOT resolved at config time (resolution is environment-
// dependent and non-deterministic); the review transport re-validates every
// resolved IP at dial time, which is the actual enforcement boundary.
// Literal metadata addresses are rejected here regardless of the private
// switch so a clearly dangerous target fails at save time.
func ValidateReviewBaseURL(rawURL string, allowPrivate bool) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid base url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("base url must be http or https")
	}
	prot := &common.SSRFProtection{
		AllowPrivateIp:         allowPrivate,
		DomainFilterMode:       false,
		IpFilterMode:           false,
		ApplyIPFilterForDomain: false, // dial-time validation is the enforcement boundary
	}
	if err := prot.ValidateURL(u.String()); err != nil {
		return err
	}
	// Literal cloud metadata addresses fail fast even with the private switch.
	if host := u.Hostname(); host != "" {
		if ip := net.ParseIP(host); ip != nil && isCloudMetadataIP(ip) {
			return errors.New("base url targets a cloud metadata address")
		}
	}
	return nil
}

// reviewResponseFormat is the strict JSON schema response format used by
// strict-schema reviews and capability probes.
func reviewResponseFormat() map[string]any {
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "llm_compliance_review",
			"strict": true,
			"schema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"verdict", "category", "confidence", "reason", "evidence"},
				"properties": map[string]any{
					"verdict": map[string]any{
						"type": "string",
						"enum": []string{"violation", "compliant", "uncertain"},
					},
					"category": map[string]any{
						"type": "string",
						"enum": []string{
							"commercial_use", "account_sharing", "unauthorized_client",
							"stress_test", "abnormal_automation", "limit_bypass",
							"harmful_resource_use", "code_generation", "other", "none",
						},
					},
					"confidence": map[string]any{
						"type":    "number",
						"minimum": 0,
						"maximum": 1,
					},
					"reason":   map[string]any{"type": "string"},
					"evidence": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
		},
	}
}

const reviewSystemPrompt = "你是本系统的使用合规审查员。你只能依据审查载荷 policy_text 中附带的条款判断，不得自行假设、补充或套用条款中不存在的规则。\n" +
	"只有 policy_text 明确禁止、且审查载荷中有直接证据支持的行为，才能返回 violation；条款未禁止的内容或用途应返回 compliant。证据不足、语义冲突或无法可靠判断时返回 uncertain。\n" +
	"RPM、Token 超限以及 trigger_type/current_value/limit_value 仅表示为什么启动审查，是行为背景，不是内容违规证据；不得仅凭超限推断用户此前违规或当前请求内容违规。\n" +
	"如果 policy_text 为空、缺失或无法读取，必须返回 uncertain，不得返回高置信度 violation。\n" +
	"审查载荷是结构化 JSON。请返回严格的 JSON 对象，不得包含任何多余文本。"

// BuildReviewChatRequest builds the reviewer request body using the mode that
// passed the configured capability test.
func (c *ReviewClient) BuildReviewChatRequest(payloadText string) ([]byte, error) {
	return c.BuildReviewChatRequestForMode(payloadText, operation_setting.EffectiveStructuredOutputMode(c.cfg))
}

// BuildReviewChatRequestForMode is also used by capability probes. Every mode
// keeps deterministic sampling and disables tools; only structured-output
// enforcement differs.
func (c *ReviewClient) BuildReviewChatRequestForMode(payloadText, mode string) ([]byte, error) {
	payloadText, policyText := payloadWithReviewPolicy(payloadText)
	systemMsg := reviewSystemPrompt
	if mode == operation_setting.StructuredOutputModePromptJSON {
		systemMsg += "\n兼容输出要求：只返回一个 JSON 对象，必须包含 verdict、category、confidence、reason、evidence 五个键，禁止 Markdown、代码围栏或额外文字。"
	}
	if policyText == "" {
		systemMsg += "\n当前未取得任何条款，因此本次裁决必须为 uncertain。"
	}
	userMsg := "审查载荷（JSON，policy_text 即当前生效条款）：\n" + payloadText

	req := map[string]any{
		"model":       c.cfg.ModelName,
		"temperature": 0,
		"top_p":       1,
		"max_tokens":  c.effectiveMaxOutputTokens(),
		"messages": []map[string]any{
			{"role": "system", "content": systemMsg},
			{"role": "user", "content": userMsg},
		},
		"tools":       []any{},
		"tool_choice": "none",
	}
	switch mode {
	case operation_setting.StructuredOutputModeStrictSchema:
		req["response_format"] = reviewResponseFormat()
	case operation_setting.StructuredOutputModeJSONObject:
		req["response_format"] = map[string]any{"type": "json_object"}
	case operation_setting.StructuredOutputModePromptJSON:
		// Prompt-only compatibility intentionally omits response_format.
	default:
		return nil, fmt.Errorf("unsupported structured output mode: %s", mode)
	}
	return common.Marshal(req)
}

// effectiveMaxOutputTokens returns the configured max output tokens with the
// default fallback.
func (c *ReviewClient) effectiveMaxOutputTokens() int {
	if c.cfg.MaxOutputTokens <= 0 {
		return ReviewDefaultMaxOutputTokens
	}
	return c.cfg.MaxOutputTokens
}

const reviewMaxResponseBytes = 2 << 20 // 2MB

// Call performs one reviewer chat completion and returns the masked raw
// response. Failures are expressed through result.Error, never panics.
func (c *ReviewClient) Call(ctx context.Context, payloadText string) LLMReviewCallResult {
	if c.cfg.BaseURL == "" || c.cfg.ModelName == "" {
		return LLMReviewCallResult{Error: errors.New("review client not configured")}
	}
	if err := ValidateReviewBaseURL(c.cfg.BaseURL, c.cfg.AllowPrivateAddress); err != nil {
		return LLMReviewCallResult{Error: err}
	}
	body, err := c.BuildReviewChatRequest(payloadText)
	if err != nil {
		return LLMReviewCallResult{Error: err}
	}
	return c.post(ctx, reviewEndpoint(c.cfg.BaseURL), body)
}

// post sends a request with the encrypted API key attached and returns the
// masked response body.
func (c *ReviewClient) post(ctx context.Context, endpoint string, body []byte) LLMReviewCallResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return LLMReviewCallResult{Error: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKeyEncrypted != "" {
		key, keyErr := DecryptLLMReviewAPIKey(c.cfg.APIKeyEncrypted)
		if keyErr != nil {
			return LLMReviewCallResult{Error: fmt.Errorf("review api key decrypt failed: %w", keyErr)}
		}
		req.Header.Set("Authorization", "Bearer "+key)
	}

	timeout := time.Duration(c.cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(ReviewDefaultTimeoutSeconds) * time.Second
	}
	c.hc.Timeout = timeout

	start := time.Now()
	resp, err := c.hc.Do(req)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		return LLMReviewCallResult{DurationMs: durationMs, Error: err}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, reviewMaxResponseBytes))
	if err != nil {
		return LLMReviewCallResult{HTTPStatus: resp.StatusCode, DurationMs: durationMs, Error: err}
	}
	// Mask credentials in whatever the upstream sent back.
	maskedBody := common.MaskReviewCredentialText(common.MaskSensitiveInfo(string(respBody)))
	if resp.StatusCode != http.StatusOK {
		return LLMReviewCallResult{
			HTTPStatus: resp.StatusCode,
			DurationMs: durationMs,
			Body:       []byte(maskedBody),
			Error:      fmt.Errorf("review llm returned http %d", resp.StatusCode),
		}
	}
	return LLMReviewCallResult{
		HTTPStatus: resp.StatusCode,
		DurationMs: durationMs,
		Body:       []byte(maskedBody),
	}
}

// TestConnection pings the reviewer endpoint with a minimal call. It does not
// modify SchemaTested.
func (c *ReviewClient) TestConnection(ctx context.Context) (map[string]any, error) {
	if c.cfg.BaseURL == "" || c.cfg.ModelName == "" {
		return nil, errors.New("review client not configured: base_url or model_name is empty")
	}
	if err := ValidateReviewBaseURL(c.cfg.BaseURL, c.cfg.AllowPrivateAddress); err != nil {
		return nil, err
	}
	req := map[string]any{
		"model":       c.cfg.ModelName,
		"temperature": 0,
		"max_tokens":  1,
		"messages": []map[string]any{
			{"role": "system", "content": "ping"},
			{"role": "user", "content": "ping"},
		},
	}
	body, err := common.Marshal(req)
	if err != nil {
		return nil, err
	}
	result := c.post(ctx, reviewEndpoint(c.cfg.BaseURL), body)
	if result.Error != nil {
		return nil, result.Error
	}
	return map[string]any{
		"ok":            true,
		"latency_ms":    result.DurationMs,
		"model":         c.cfg.ModelName,
		"schema_tested": c.cfg.SchemaTested,
	}, nil
}

// TestSchemaCapability verifies strict JSON schema support against the live
// endpoint. The caller persists SchemaTested on success.
type StructuredOutputCapabilityResult struct {
	Passed bool
	Mode   string
	Error  string
}

// TestStructuredOutputCapability probes strict output first, then the two
// explicit compatibility modes. Fallback is limited to a structured-output
// rejection or an invalid probe result; authentication, transport, rate-limit,
// endpoint, and server failures are returned without hiding them.
func (c *ReviewClient) TestStructuredOutputCapability(ctx context.Context) (StructuredOutputCapabilityResult, error) {
	if c.cfg.BaseURL == "" || c.cfg.ModelName == "" {
		return StructuredOutputCapabilityResult{}, errors.New("review client not configured")
	}
	probePayload := `{"request_snippet":"capability probe","policy_text":"仅测试 JSON 输出格式，不作真实裁决。"}`
	modes := []string{operation_setting.StructuredOutputModeStrictSchema, operation_setting.StructuredOutputModeJSONObject, operation_setting.StructuredOutputModePromptJSON}
	var lastErr string
	for _, mode := range modes {
		body, err := c.BuildReviewChatRequestForMode(probePayload, mode)
		if err != nil {
			return StructuredOutputCapabilityResult{}, err
		}
		result := c.post(ctx, reviewEndpoint(c.cfg.BaseURL), body)
		if result.Error != nil {
			if !isStructuredOutputFallbackError(result) {
				return StructuredOutputCapabilityResult{Mode: mode, Error: sanitizeCapabilityError(result.Error)}, nil
			}
			lastErr = sanitizeCapabilityError(result.Error)
			continue
		}
		normalized, err := NormalizeRawLLMResponse(result.Body)
		if err != nil {
			lastErr = "parse content: " + err.Error()
			continue
		}
		if mode == operation_setting.StructuredOutputModeStrictSchema && normalized.Repaired {
			lastErr = "strict mode requires a direct JSON object"
			continue
		}
		validateVerdict := ValidateLLMReviewVerdict
		if mode == operation_setting.StructuredOutputModeStrictSchema {
			validateVerdict = ValidateStrictLLMReviewVerdict
		}
		if _, passed, schemaErr := validateVerdict([]byte(normalized.Content)); passed {
			return StructuredOutputCapabilityResult{Passed: true, Mode: mode}, nil
		} else {
			lastErr = schemaErr
		}
	}
	if lastErr == "" {
		lastErr = "no supported structured-output mode"
	}
	return StructuredOutputCapabilityResult{Error: lastErr}, nil
}

// TestSchemaCapability retains the old strict-only helper for callers outside
// the controller while the API uses the mode-aware probe.
func (c *ReviewClient) TestSchemaCapability(ctx context.Context) (bool, string, error) {
	result, err := c.testCapabilityMode(ctx, operation_setting.StructuredOutputModeStrictSchema)
	if err != nil {
		return false, "", err
	}
	return result.Passed, result.Error, nil
}

func (c *ReviewClient) testCapabilityMode(ctx context.Context, mode string) (StructuredOutputCapabilityResult, error) {
	body, err := c.BuildReviewChatRequestForMode(`{"request_snippet":"capability probe","policy_text":"仅测试 JSON 输出格式，不作真实裁决。"}`, mode)
	if err != nil {
		return StructuredOutputCapabilityResult{}, err
	}
	result := c.post(ctx, reviewEndpoint(c.cfg.BaseURL), body)
	if result.Error != nil {
		return StructuredOutputCapabilityResult{Mode: mode, Error: sanitizeCapabilityError(result.Error)}, nil
	}
	normalized, err := NormalizeRawLLMResponse(result.Body)
	if err != nil {
		return StructuredOutputCapabilityResult{Mode: mode, Error: "parse content: " + err.Error()}, nil
	}
	if mode == operation_setting.StructuredOutputModeStrictSchema && normalized.Repaired {
		return StructuredOutputCapabilityResult{Mode: mode, Error: "strict mode requires a direct JSON object"}, nil
	}
	validateVerdict := ValidateLLMReviewVerdict
	if mode == operation_setting.StructuredOutputModeStrictSchema {
		validateVerdict = ValidateStrictLLMReviewVerdict
	}
	_, passed, schemaErr := validateVerdict([]byte(normalized.Content))
	return StructuredOutputCapabilityResult{Passed: passed, Mode: mode, Error: schemaErr}, nil
}

func sanitizeCapabilityError(err error) string {
	return common.MaskReviewCredentialText(common.MaskSensitiveInfo(err.Error()))
}

func isStructuredOutputFallbackError(result LLMReviewCallResult) bool {
	if result.HTTPStatus != http.StatusBadRequest && result.HTTPStatus != http.StatusUnprocessableEntity {
		return false
	}
	message := strings.ToLower(string(result.Body))
	for _, marker := range []string{"json_schema", "response_format", "structured output", "structured_output", "not supported", "unsupported"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
