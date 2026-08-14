package opencodego

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) GetChannelName() string {
	return "OpenCode Go"
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if info != nil && info.ChannelMeta != nil && info.SupportStreamOptions && info.IsStream {
		if request.StreamOptions == nil {
			request.StreamOptions = &dto.StreamOptions{}
		}
		request.StreamOptions.IncludeUsage = true
	}
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	sanitizeClaudeRequest(request)
	result, err := service.ConvertRequest(c, info, types.RelayFormatOpenAI, request)
	if err != nil {
		return nil, err
	}
	openAIRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, errors.New("Claude request conversion did not return an OpenAI request")
	}
	for messageIndex := range openAIRequest.Messages {
		message := &openAIRequest.Messages[messageIndex]
		if message.IsStringContent() {
			continue
		}
		contents := message.ParseContent()
		removedCacheControl := false
		for contentIndex := range contents {
			if len(contents[contentIndex].CacheControl) == 0 {
				continue
			}
			contents[contentIndex].CacheControl = nil
			removedCacheControl = true
		}
		if removedCacheControl {
			message.SetMediaContent(contents)
		}
	}
	return a.ConvertOpenAIRequest(c, info, openAIRequest)
}

func sanitizeClaudeRequest(request *dto.ClaudeRequest) {
	if request.System != nil {
		if request.IsStringSystem() {
			systemText := removeBillingHeaderLines(request.GetStringSystem())
			if systemText == "" {
				request.System = nil
			} else {
				request.SetStringSystem(removeVolatileCCH(systemText))
			}
		} else {
			systemBlocks := request.ParseSystem()
			filteredSystemBlocks := make([]dto.ClaudeMediaMessage, 0, len(systemBlocks))
			for systemIndex := range systemBlocks {
				systemBlock := systemBlocks[systemIndex]
				if systemBlock.Text != nil {
					cleanedSystemText := removeBillingHeaderLines(systemBlock.GetText())
					cleanedSystemText = removeVolatileCCH(cleanedSystemText)
					if cleanedSystemText == "" {
						continue
					}
					systemBlock.SetText(cleanedSystemText)
				}
				filteredSystemBlocks = append(filteredSystemBlocks, systemBlock)
			}
			if len(filteredSystemBlocks) == 0 {
				request.System = nil
			} else {
				request.System = filteredSystemBlocks
			}
		}
	}

	for messageIndex := range request.Messages {
		if strings.EqualFold(request.Messages[messageIndex].Role, "system") {
			request.Messages[messageIndex].Role = "user"
		}
	}
}

func removeBillingHeaderLines(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	retainedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		markerIndex := strings.Index(line, "x-anthropic-billing-header")
		if markerIndex < 0 {
			retainedLines = append(retainedLines, line)
			continue
		}
		headerEnd := billingHeaderRegionEnd(line, markerIndex)
		retainedLines = append(retainedLines, line[:markerIndex]+line[headerEnd:])
	}
	return strings.TrimSpace(strings.Join(retainedLines, "\n"))
}

// billingHeaderRegionEnd returns the index just past the billing header fields
// in the region starting at markerIndex. The header always ends with its "cch="
// field followed by "; " (see the design doc), so real text after that on the
// same line must be kept. If no "cch=" field is found, the whole region to the
// end of the line is treated as header.
func billingHeaderRegionEnd(line string, markerIndex int) int {
	region := line[markerIndex:]
	const cchMarker = "cch="
	cchIndex := strings.Index(region, cchMarker)
	if cchIndex < 0 {
		return len(line)
	}
	segmentEnd := cchIndex + len(cchMarker)
	for segmentEnd < len(region) && isCCHTokenChar(region[segmentEnd]) {
		segmentEnd++
	}
	if segmentEnd < len(region) && region[segmentEnd] == ';' {
		segmentEnd++
	}
	for segmentEnd < len(region) && (region[segmentEnd] == ' ' || region[segmentEnd] == '\t') {
		segmentEnd++
	}
	return markerIndex + segmentEnd
}

// isCCHTokenChar reports whether b is a legal character inside a "cch=" token:
// hex digits per the header format, plus the letters and hyphen used by real
// client values. Any other byte (punctuation, whitespace, non-ASCII) ends the
// token.
func isCCHTokenChar(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '-' || b == '_'
}

func removeVolatileCCH(text string) string {
	const marker = "cch="
	for {
		markerIndex := strings.Index(text, marker)
		if markerIndex < 0 {
			return strings.TrimSpace(text)
		}
		segmentEnd := markerIndex + len(marker)
		for segmentEnd < len(text) && isCCHTokenChar(text[segmentEnd]) {
			segmentEnd++
		}
		if segmentEnd < len(text) && text[segmentEnd] == ';' {
			segmentEnd++
		}
		for segmentEnd < len(text) && (text[segmentEnd] == ' ' || text[segmentEnd] == '\t') {
			segmentEnd++
		}
		text = text[:markerIndex] + text[segmentEnd:]
	}
}
