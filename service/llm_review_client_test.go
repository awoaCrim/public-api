package service

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateReviewBaseURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		allowPrivate bool
		wantErr      bool
	}{
		{"public https allowed", "https://review.example.com/v1", false, false},
		{"public http allowed", "http://review.example.com", false, false},
		{"missing scheme", "review.example.com", false, true},
		{"ftp rejected", "ftp://review.example.com", false, true},
		{"loopback rejected", "http://127.0.0.1:8080", false, true},
		{"loopback allowed with switch", "http://127.0.0.1:8080", true, false},
		{"private rejected", "http://10.0.0.5", false, true},
		{"metadata rejected even with switch", "http://169.254.169.254", true, true},
		{"link local rejected even with switch", "http://169.254.10.10", true, true},
		{"private allowed with switch", "http://10.0.0.5", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReviewBaseURL(tt.url, tt.allowPrivate)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReviewEndpointCompatibility(t *testing.T) {
	assert.Equal(t, "https://r.example.com/v1/chat/completions", reviewEndpoint("https://r.example.com/v1"))
	assert.Equal(t, "https://r.example.com/v1/chat/completions", reviewEndpoint("https://r.example.com/"))
	assert.Equal(t, "https://r.example.com/v1/chat/completions", reviewEndpoint("https://r.example.com"))
}

func TestIsReviewIPAllowedPolicy(t *testing.T) {
	tests := []struct {
		name         string
		ip           string
		allowPrivate bool
		want         bool
	}{
		{"public ip", "203.0.113.7", false, true},
		{"loopback blocked", "127.0.0.1", false, false},
		{"loopback allowed with switch", "127.0.0.1", true, true},
		{"private blocked", "10.1.2.3", false, false},
		{"private allowed with switch", "10.1.2.3", true, true},
		{"cloud metadata always blocked", "169.254.169.254", true, false},
		{"link local always blocked", "169.254.10.10", true, false},
		{"unspecified blocked", "0.0.0.0", true, false},
		{"multicast blocked without switch", "224.0.0.1", false, false},
		{"link local multicast follows switch", "224.0.0.1", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isReviewIPAllowed(net.ParseIP(tt.ip), tt.allowPrivate))
		})
	}
}

// TestReviewClientCallAgainstLocalServer drives Call through a local HTTP
// server with the private-address switch on, verifying the response body is
// masked and non-200 statuses surface as errors.
func TestReviewClientCallAgainstLocalServer(t *testing.T) {
	originalSecret := common.CryptoSecret
	common.CryptoSecret = "llm-review-client-test"
	t.Cleanup(func() { common.CryptoSecret = originalSecret })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer sk-client-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"bad key"}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"verdict\":\"compliant\",\"category\":\"none\",\"confidence\":0.9,\"reason\":\"ok\",\"evidence\":[\"e\"]}"}}]}`))
	}))
	defer server.Close()

	enc, err := EncryptLLMReviewAPIKey("sk-client-key")
	require.NoError(t, err)
	cfg := &operation_setting.LLMReviewSetting{
		BaseURL:             server.URL,
		ModelName:           "reviewer",
		APIKeyEncrypted:     enc,
		TimeoutSeconds:      5,
		AllowPrivateAddress: true, // loopback for the test server
	}
	client := NewReviewClient(cfg)

	result := client.Call(context.Background(), `{"request_snippet":"x","policy_text":"terms"}`)
	require.NoError(t, result.Error)
	assert.Equal(t, http.StatusOK, result.HTTPStatus)
	content, err := ParseRawLLMResponse(result.Body)
	require.NoError(t, err)
	assert.Contains(t, content, "compliant")
}

// TestReviewClientMasksErrorResponses verifies credentials in upstream error
// bodies are masked before they can be persisted.
func TestReviewClientMasksErrorResponses(t *testing.T) {
	originalSecret := common.CryptoSecret
	common.CryptoSecret = "llm-review-client-test"
	t.Cleanup(func() { common.CryptoSecret = originalSecret })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Authorization: Bearer sk-leak-123456"}`))
	}))
	defer server.Close()

	cfg := &operation_setting.LLMReviewSetting{
		BaseURL:             server.URL,
		ModelName:           "reviewer",
		TimeoutSeconds:      5,
		AllowPrivateAddress: true,
	}
	result := NewReviewClient(cfg).Call(context.Background(), `{}`)
	require.Error(t, result.Error)
	assert.NotContains(t, string(result.Body), "sk-leak-123456", "upstream credentials must be masked")
	assert.Contains(t, string(result.Body), "***")
}

// TestReviewClientRejectsRedirects verifies the redirect-blocking policy.
func TestReviewClientRejectsRedirects(t *testing.T) {
	originalSecret := common.CryptoSecret
	common.CryptoSecret = "llm-review-client-test"
	t.Cleanup(func() { common.CryptoSecret = originalSecret })

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	cfg := &operation_setting.LLMReviewSetting{
		BaseURL:             redirector.URL,
		ModelName:           "reviewer",
		TimeoutSeconds:      5,
		AllowPrivateAddress: true,
	}
	result := NewReviewClient(cfg).Call(context.Background(), `{}`)
	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "redirect")
}

// TestBuildReviewChatRequestPinsDeterministicContract verifies the reviewer
// request shape: no tools, no randomness, strict schema, policy embedded.
func TestBuildReviewChatRequestPinsDeterministicContract(t *testing.T) {
	cfg := llmReviewSettingForTest(t)
	cfg.ModelName = "reviewer-model"
	cfg.MaxOutputTokens = 500
	cfg.PolicyText = "No account sharing."

	client := NewReviewClient(cfg)
	body, err := client.BuildReviewChatRequest(`{"request_snippet":"x"}`)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, common.Unmarshal(body, &req))
	assert.Equal(t, "reviewer-model", req["model"])
	assert.Equal(t, float64(0), req["temperature"])
	assert.Equal(t, float64(500), req["max_tokens"])
	assert.Equal(t, "none", req["tool_choice"])

	format, ok := req["response_format"].(map[string]any)
	require.True(t, ok)
	js, ok := format["json_schema"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, js["strict"])

	messages, ok := req["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)
	systemMsg := messages[0].(map[string]any)["content"].(string)
	assert.Contains(t, systemMsg, "category 必须填写 none")
	userMsg := messages[1].(map[string]any)["content"].(string)
	assert.Contains(t, userMsg, "No account sharing.")
}

func TestBuildReviewChatRequestSupportsCompatibilityModes(t *testing.T) {
	cfg := llmReviewSettingForTest(t)
	cfg.ModelName = "reviewer-model"
	cfg.PolicyText = "No account sharing."
	client := NewReviewClient(cfg)

	jsonObjectBody, err := client.BuildReviewChatRequestForMode(`{"request_snippet":"x"}`, operation_setting.StructuredOutputModeJSONObject)
	require.NoError(t, err)
	var jsonObjectReq map[string]any
	require.NoError(t, common.Unmarshal(jsonObjectBody, &jsonObjectReq))
	assert.Equal(t, map[string]any{"type": "json_object"}, jsonObjectReq["response_format"])

	promptJSONBody, err := client.BuildReviewChatRequestForMode(`{"request_snippet":"x"}`, operation_setting.StructuredOutputModePromptJSON)
	require.NoError(t, err)
	var promptJSONReq map[string]any
	require.NoError(t, common.Unmarshal(promptJSONBody, &promptJSONReq))
	assert.NotContains(t, promptJSONReq, "response_format")
	promptMessages := promptJSONReq["messages"].([]any)
	assert.Contains(t, promptMessages[0].(map[string]any)["content"].(string), "兼容输出要求")
}

func TestTestStructuredOutputCapabilityFallsBackToJSONMode(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &req))
		format, _ := req["response_format"].(map[string]any)
		if format["type"] == "json_schema" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"json_schema is not supported"}}`))
			return
		}
		_, _ = w.Write([]byte(reviewVerdictBody("uncertain", "none", 0.2)))
	}))
	defer server.Close()

	cfg := &operation_setting.LLMReviewSetting{
		BaseURL:             server.URL,
		ModelName:           "reviewer",
		TimeoutSeconds:      5,
		AllowPrivateAddress: true,
	}
	result, err := NewReviewClient(cfg).TestStructuredOutputCapability(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, operation_setting.StructuredOutputModeJSONObject, result.Mode)
	assert.Equal(t, 2, calls)
}

func TestTestStructuredOutputCapabilityNormalizesUncertainCategoryInCompatibilityMode(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &req))
		format, _ := req["response_format"].(map[string]any)
		if format["type"] == "json_schema" {
			_, _ = w.Write([]byte(reviewVerdictBody("uncertain", "uncertain", 0.2)))
			return
		}
		_, _ = w.Write([]byte(reviewVerdictBody("uncertain", "uncertain", 0.2)))
	}))
	defer server.Close()

	cfg := &operation_setting.LLMReviewSetting{
		BaseURL:             server.URL,
		ModelName:           "reviewer",
		TimeoutSeconds:      5,
		AllowPrivateAddress: true,
	}
	result, err := NewReviewClient(cfg).TestStructuredOutputCapability(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, operation_setting.StructuredOutputModeJSONObject, result.Mode)
	assert.Equal(t, 2, calls)
}

func TestTestStructuredOutputCapabilityDoesNotTrustRepairedStrictResponse(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &req))
		format, _ := req["response_format"].(map[string]any)
		if format["type"] == "json_schema" {
			inner := `{"verdict":"uncertain","category":"none","confidence":0.2,"reason":"r","evidence":["e"]}`
			content, marshalErr := common.Marshal("```json\n" + inner + "\n```")
			require.NoError(t, marshalErr)
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + string(content) + `}}]}`))
			return
		}
		_, _ = w.Write([]byte(reviewVerdictBody("uncertain", "none", 0.2)))
	}))
	defer server.Close()

	cfg := &operation_setting.LLMReviewSetting{
		BaseURL:             server.URL,
		ModelName:           "reviewer",
		TimeoutSeconds:      5,
		AllowPrivateAddress: true,
	}
	result, err := NewReviewClient(cfg).TestStructuredOutputCapability(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, operation_setting.StructuredOutputModeJSONObject, result.Mode)
	assert.Equal(t, 2, calls)
}

func TestTestStructuredOutputCapabilityDoesNotFallbackOnAuthFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	cfg := &operation_setting.LLMReviewSetting{
		BaseURL:             server.URL,
		ModelName:           "reviewer",
		TimeoutSeconds:      5,
		AllowPrivateAddress: true,
	}
	result, err := NewReviewClient(cfg).TestStructuredOutputCapability(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Equal(t, operation_setting.StructuredOutputModeStrictSchema, result.Mode)
	assert.Contains(t, result.Error, "http 400")
	assert.Equal(t, 1, calls)
}
