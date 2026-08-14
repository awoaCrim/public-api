package opencodego

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestForcesStreamUsage(t *testing.T) {
	stream := true
	request := &dto.GeneralOpenAIRequest{
		Stream:        &stream,
		StreamOptions: &dto.StreamOptions{IncludeUsage: false},
	}
	info := &relaycommon.RelayInfo{
		IsStream: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          constant.ChannelTypeOpenCodeGo,
			SupportStreamOptions: true,
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)

	require.NoError(t, err)
	openAIRequest := requireOpenAIRequest(t, converted)
	require.NotNil(t, openAIRequest.StreamOptions)
	assert.True(t, openAIRequest.StreamOptions.IncludeUsage)
}

func TestConvertOpenAIRequestCreatesStreamUsageOptions(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{}
	info := &relaycommon.RelayInfo{
		IsStream: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          constant.ChannelTypeOpenCodeGo,
			SupportStreamOptions: true,
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)

	require.NoError(t, err)
	openAIRequest := requireOpenAIRequest(t, converted)
	require.NotNil(t, openAIRequest.StreamOptions)
	assert.True(t, openAIRequest.StreamOptions.IncludeUsage)
}

func TestConvertOpenAIRequestRejectsNilRequest(t *testing.T) {
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, nil)

	require.Error(t, err)
	assert.Nil(t, converted)
}

func TestConvertOpenAIRequestLeavesNonStreamingOptionsUnchanged(t *testing.T) {
	streamOptions := &dto.StreamOptions{IncludeUsage: false}
	request := &dto.GeneralOpenAIRequest{StreamOptions: streamOptions}
	info := &relaycommon.RelayInfo{
		IsStream: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          constant.ChannelTypeOpenCodeGo,
			SupportStreamOptions: true,
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)

	require.NoError(t, err)
	openAIRequest := requireOpenAIRequest(t, converted)
	assert.Same(t, streamOptions, openAIRequest.StreamOptions)
	assert.False(t, openAIRequest.StreamOptions.IncludeUsage)
}

func TestOpenCodeGoSupportsStreamOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set(string(constant.ContextKeyChannelType), constant.ChannelTypeOpenCodeGo)
	info := &relaycommon.RelayInfo{}
	info.InitChannelMeta(context)

	require.NotNil(t, info.ChannelMeta)
	assert.True(t, info.SupportStreamOptions)
}

func TestConvertClaudeRequestSanitizesPromptAndPreservesContent(t *testing.T) {
	stream := true
	billingHeader := "x-anthropic-billing-header: cc_version=2.1.177; cch=b8e89;"
	stableSystem := "Stable instructions cch=volatile-token; remain stable."
	request := &dto.ClaudeRequest{
		Model:  "claude-sonnet-4-5",
		Stream: &stream,
		System: []dto.ClaudeMediaMessage{
			{Type: "text", Text: &billingHeader},
			{Type: "text", Text: &stableSystem},
		},
		Messages: []dto.ClaudeMessage{
			{
				Role: "System",
				Content: []dto.ClaudeMediaMessage{
					{Type: "text", Text: stringPointer("Keep this reminder."), CacheControl: []byte(`{"type":"ephemeral"}`)},
				},
			},
			{
				Role: "user",
				Content: []dto.ClaudeMediaMessage{
					{Type: "text", Text: stringPointer("Keep this user message."), CacheControl: []byte(`{"type":"ephemeral"}`)},
				},
			},
		},
	}
	info := &relaycommon.RelayInfo{
		IsStream: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:          constant.ChannelTypeOpenCodeGo,
			SupportStreamOptions: true,
		},
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, request)

	require.NoError(t, err)
	openAIRequest := requireOpenAIRequest(t, converted)
	require.NotNil(t, openAIRequest.StreamOptions)
	assert.True(t, openAIRequest.StreamOptions.IncludeUsage)
	require.Len(t, request.ParseSystem(), 1)
	assert.Equal(t, "Stable instructions remain stable.", request.ParseSystem()[0].GetText())
	assert.Equal(t, "user", request.Messages[0].Role)
	reminderContent, err := request.Messages[0].ParseContent()
	require.NoError(t, err)
	require.Len(t, reminderContent, 1)
	assert.Equal(t, "Keep this reminder.", reminderContent[0].GetText())

	require.Len(t, openAIRequest.Messages, 3)
	assert.Equal(t, "system", openAIRequest.Messages[0].Role)
	assert.Equal(t, "Stable instructions remain stable.", openAIRequest.Messages[0].StringContent())
	assert.Equal(t, "user", openAIRequest.Messages[1].Role)
	assert.Equal(t, "Keep this reminder.", openAIRequest.Messages[1].ParseContent()[0].Text)
	assert.Equal(t, "Keep this user message.", openAIRequest.Messages[2].ParseContent()[0].Text)
	for _, message := range openAIRequest.Messages {
		for _, content := range message.ParseContent() {
			assert.Empty(t, content.CacheControl)
		}
	}

	requestAfterFirstConversion, err := common.Marshal(request)
	require.NoError(t, err)
	_, err = (&Adaptor{}).ConvertClaudeRequest(nil, info, request)
	require.NoError(t, err)
	requestAfterSecondConversion, err := common.Marshal(request)
	require.NoError(t, err)
	assert.JSONEq(t, string(requestAfterFirstConversion), string(requestAfterSecondConversion))
}

func TestConvertClaudeRequestClearsBillingOnlyStringSystem(t *testing.T) {
	request := &dto.ClaudeRequest{Model: "claude-sonnet-4-5"}
	request.SetStringSystem("x-anthropic-billing-header: cc_version=2.1.177; cch=b8e89;")

	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, &relaycommon.RelayInfo{}, request)

	require.NoError(t, err)
	assert.Nil(t, request.System)
	for _, message := range requireOpenAIRequest(t, converted).Messages {
		assert.NotEqual(t, "system", message.Role)
	}
}

func TestConvertClaudeRequestKeepsSameLineContentBesideBillingHeader(t *testing.T) {
	request := &dto.ClaudeRequest{Model: "claude-sonnet-4-5"}
	request.SetStringSystem("x-anthropic-billing-header: cc_version=2.1.177; cch=b8e89; Keep these instructions.")

	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, &relaycommon.RelayInfo{}, request)

	require.NoError(t, err)
	assert.Equal(t, "Keep these instructions.", request.GetStringSystem())
	openAIRequest := requireOpenAIRequest(t, converted)
	require.Len(t, openAIRequest.Messages, 1)
	assert.Equal(t, "Keep these instructions.", openAIRequest.Messages[0].StringContent())

	requestAfterFirstConversion, err := common.Marshal(request)
	require.NoError(t, err)
	_, err = (&Adaptor{}).ConvertClaudeRequest(nil, &relaycommon.RelayInfo{}, request)
	require.NoError(t, err)
	requestAfterSecondConversion, err := common.Marshal(request)
	require.NoError(t, err)
	assert.JSONEq(t, string(requestAfterFirstConversion), string(requestAfterSecondConversion))
}

func TestRemoveBillingHeaderLinesPreservesSameLineContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "content after header on the same line",
			input: "x-anthropic-billing-header: cc_version=2.1.177; cch=b8e89; Keep these instructions.",
			want:  "Keep these instructions.",
		},
		{
			name:  "text before header on the same line",
			input: "Instructions first. x-anthropic-billing-header: cc_version=2.1.177; cch=b8e89; stay focused.",
			want:  "Instructions first. stay focused.",
		},
		{
			name:  "header on its own line is fully removed",
			input: "x-anthropic-billing-header: cc_version=2.1.177; cch=b8e89;\nKeep these instructions.",
			want:  "Keep these instructions.",
		},
		{
			name:  "line without header is preserved",
			input: "Keep these instructions.",
			want:  "Keep these instructions.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cleaned := removeBillingHeaderLines(test.input)

			assert.Equal(t, test.want, cleaned)
			assert.NotContains(t, cleaned, "x-anthropic-billing-header")
			assert.NotContains(t, cleaned, "cch=")
		})
	}
}

func TestConvertClaudeRequestPreservesNormalTextBesideBillingHeader(t *testing.T) {
	request := &dto.ClaudeRequest{Model: "claude-sonnet-4-5"}
	request.SetStringSystem("x-anthropic-billing-header: cc_version=2.1.177; cch=b8e89;\nKeep these instructions.")

	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, &relaycommon.RelayInfo{}, request)

	require.NoError(t, err)
	assert.Equal(t, "Keep these instructions.", request.GetStringSystem())
	openAIRequest := requireOpenAIRequest(t, converted)
	require.Len(t, openAIRequest.Messages, 1)
	assert.Equal(t, "Keep these instructions.", openAIRequest.Messages[0].StringContent())
}

func TestConvertClaudeRequestPreservesMixedBillingBlockTextAndFields(t *testing.T) {
	mixedSystemText := "x-anthropic-billing-header: cc_version=2.1.177; cch=b8e89;\nKeep these instructions."
	cacheControl := []byte(`{"type":"ephemeral"}`)
	request := &dto.ClaudeRequest{
		Model: "claude-sonnet-4-5",
		System: []dto.ClaudeMediaMessage{
			{
				Type:         "text",
				Text:         &mixedSystemText,
				CacheControl: cacheControl,
			},
		},
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, &relaycommon.RelayInfo{}, request)

	require.NoError(t, err)
	systemBlocks := request.ParseSystem()
	require.Len(t, systemBlocks, 1)
	assert.Equal(t, "Keep these instructions.", systemBlocks[0].GetText())
	assert.Equal(t, cacheControl, []byte(systemBlocks[0].CacheControl))
	openAIRequest := requireOpenAIRequest(t, converted)
	require.Len(t, openAIRequest.Messages, 1)
	assert.Equal(t, "Keep these instructions.", openAIRequest.Messages[0].StringContent())

	requestAfterFirstConversion, err := common.Marshal(request)
	require.NoError(t, err)
	_, err = (&Adaptor{}).ConvertClaudeRequest(nil, &relaycommon.RelayInfo{}, request)
	require.NoError(t, err)
	requestAfterSecondConversion, err := common.Marshal(request)
	require.NoError(t, err)
	assert.JSONEq(t, string(requestAfterFirstConversion), string(requestAfterSecondConversion))
}

func TestRemoveVolatileCCHPreservesTextAfterPunctuation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "comma", input: "Keep cch=abc,normal text", expected: "Keep ,normal text"},
		{name: "right parenthesis", input: "Keep cch=abc)normal text", expected: "Keep )normal text"},
		{name: "right bracket", input: "Keep cch=abc]normal text", expected: "Keep ]normal text"},
		{name: "right brace", input: "Keep cch=abc}normal text", expected: "Keep }normal text"},
		{name: "period", input: "Keep cch=abc.normal text", expected: "Keep .normal text"},
		{name: "legacy semicolon", input: "Keep cch=abc; next", expected: "Keep next"},
		{name: "colon", input: "Keep cch=abc:normal text", expected: "Keep :normal text"},
		{name: "exclamation", input: "Keep cch=abc!important", expected: "Keep !important"},
		{name: "question", input: "Keep cch=abc?what now", expected: "Keep ?what now"},
		{name: "double quote", input: "Keep cch=abc\"quoted\"", expected: "Keep \"quoted\""},
		{name: "single quote", input: "It's cch=abc'fine'", expected: "It's 'fine'"},
		{name: "hyphenated token", input: "Keep cch=abc-def; next", expected: "Keep next"},
		{name: "slash", input: "Keep cch=abc/x", expected: "Keep /x"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cleaned := removeVolatileCCH(test.input)

			assert.Equal(t, test.expected, cleaned)
			assert.Equal(t, cleaned, removeVolatileCCH(cleaned))
		})
	}
}

func TestConvertClaudeRequestPreservesNormalStringSystem(t *testing.T) {
	request := &dto.ClaudeRequest{Model: "claude-sonnet-4-5"}
	request.SetStringSystem("You are a careful coding assistant.")

	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, &relaycommon.RelayInfo{}, request)

	require.NoError(t, err)
	assert.Equal(t, "You are a careful coding assistant.", request.GetStringSystem())
	openAIRequest := requireOpenAIRequest(t, converted)
	require.Len(t, openAIRequest.Messages, 1)
	assert.Equal(t, "You are a careful coding assistant.", openAIRequest.Messages[0].StringContent())
}

func TestEmbeddedOpenAIAdaptorUsesOpenCodeGoURLAndHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	context.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeChatCompletions,
		RequestURLPath: "/v1/chat/completions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeOpenCodeGo,
			ChannelBaseUrl: "https://opencode.ai",
			ApiKey:         "opencode-test-key",
		},
	}
	adaptor := &Adaptor{}

	adaptor.Init(info)
	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	requestHeaders := http.Header{}
	err = adaptor.SetupRequestHeader(context, &requestHeaders, info)

	require.NoError(t, err)
	assert.Equal(t, constant.ChannelTypeOpenCodeGo, adaptor.ChannelType)
	assert.Equal(t, "https://opencode.ai/v1/chat/completions", requestURL)
	assert.Equal(t, "Bearer opencode-test-key", requestHeaders.Get("Authorization"))
	assert.Equal(t, "application/json", requestHeaders.Get("Content-Type"))
}

func requireOpenAIRequest(t *testing.T, converted any) *dto.GeneralOpenAIRequest {
	t.Helper()
	openAIRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	return openAIRequest
}

func stringPointer(value string) *string {
	return &value
}
