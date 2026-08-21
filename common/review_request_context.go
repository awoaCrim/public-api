package common

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// LLMReviewRequestContext is the bounded, copied request evidence attached to
// an automatic LLM review. It is safe to use after the originating request has
// returned; it never contains a live gin.Context, request body, or header map.
type LLMReviewRequestContext struct {
	Summary string
	Body    string
	Headers map[string][]string
}

var reviewRequestLongOpaqueText = regexp.MustCompile(`[A-Za-z0-9+/=_-]{128,}`)

const (
	// Keep request-context parsing bounded while allowing ordinary large chat
	// requests to be parsed as complete JSON. This is a reviewer-capture limit,
	// not the relay request-body limit; larger valid bodies use a safe omission
	// marker instead of being treated as malformed input.
	reviewRequestBodyReadBytes   = 256 << 10
	reviewRequestBodyMaxBytes    = 8 << 10
	reviewRequestMaxDepth        = 6
	reviewRequestMaxMapEntries   = 40
	reviewRequestMaxArrayItems   = 24
	reviewRequestMaxStringRunes  = 512
	reviewRequestMaxHeaderCount  = 40
	reviewRequestMaxHeaderValues = 4
	reviewRequestMaxHeaderRunes  = 300
	reviewRequestMaxHeadersBytes = 6 << 10
)

// These aliases keep the bounds easy to assert in package-level regression
// tests without exposing implementation constants as public API.
const reviewRequestBodyMaxBytesForTest = reviewRequestBodyMaxBytes

// CaptureLLMReviewRequestContext synchronously copies bounded request evidence
// before an asynchronous review enqueue. Body storage is read through an
// independent reader, so downstream relay code can still consume it.
func CaptureLLMReviewRequestContext(c *gin.Context) LLMReviewRequestContext {
	captured := LLMReviewRequestContext{}
	if c == nil || c.Request == nil {
		return captured
	}

	captured.Headers = redactReviewRequestHeaders(c.Request.Header)
	body, truncated := readReviewRequestBody(c)
	contentType := c.Request.Header.Get("Content-Type")
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	// Only JSON gets the chat-oriented snippet extractor. Applying the JSON
	// fallback to multipart or binary bytes would copy file/media content into
	// request_snippet before the body-specific redaction runs.
	if isReviewJSONMediaType(mediaType) && !truncated {
		var value any
		if err := Unmarshal(body, &value); err == nil {
			captured.Summary = maskReviewRequestFreeText(ExtractLLMReviewSnippet(body))
		}
	}
	captured.Body = redactReviewRequestBody(body, contentType, truncated)
	return captured
}

func readReviewRequestBody(c *gin.Context) ([]byte, bool) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, false
	}
	storage, err := GetBodyStorage(c)
	if err != nil {
		return nil, false
	}
	reader, err := storage.NewReader()
	if err != nil {
		return nil, false
	}
	defer reader.Close()
	body, err := io.ReadAll(io.LimitReader(reader, reviewRequestBodyReadBytes+1))
	if err != nil {
		return nil, false
	}
	return body, len(body) > reviewRequestBodyReadBytes
}

func redactReviewRequestBody(body []byte, contentType string, truncated bool) string {
	if len(body) == 0 {
		return ""
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if strings.HasPrefix(mediaType, "image/") || strings.HasPrefix(mediaType, "audio/") ||
		strings.HasPrefix(mediaType, "video/") || mediaType == "application/octet-stream" {
		return "[media body omitted]"
	}
	if isReviewJSONMediaType(mediaType) {
		if truncated {
			return "[json body omitted: exceeds review capture limit]"
		}
		var value any
		if err := Unmarshal(body, &value); err == nil {
			return marshalBoundedReviewRequestValue(value)
		}
		return "[json body omitted: invalid]"
	}
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		if truncated {
			return "[multipart body omitted: exceeds review capture limit]"
		}
		if value, ok := parseBoundedReviewMultipart(body, contentType); ok {
			return marshalBoundedReviewRequestValue(value)
		}
		return "[multipart body omitted: invalid]"
	}
	if strings.HasPrefix(strings.ToLower(contentType), "application/x-www-form-urlencoded") {
		if values, err := url.ParseQuery(string(body)); err == nil {
			value := make(map[string]any, len(values))
			for key, items := range values {
				value[key] = items
			}
			return marshalBoundedReviewRequestValue(value)
		}
	}

	text := maskReviewRequestFreeText(sanitizeReviewRequestText(MaskSensitiveInfo(string(body))))
	if isReviewRequestMediaValue(strings.TrimSpace(text)) {
		return "[omitted media]"
	}
	if truncated || len(body) > reviewRequestBodyMaxBytes {
		if len(text) < reviewRequestBodyMaxBytes-len("...[truncated]") {
			return text + "...[truncated]"
		}
		return truncateReviewRequestBytesWithSuffix(text, reviewRequestBodyMaxBytes, "...[truncated]")
	}
	return truncateReviewRequestBytes(text, reviewRequestBodyMaxBytes)
}

func isReviewJSONMediaType(mediaType string) bool {
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func parseBoundedReviewMultipart(body []byte, contentType string) (map[string]any, bool) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		return nil, false
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	form, err := reader.ReadForm(reviewRequestBodyReadBytes)
	if err != nil {
		return nil, false
	}
	defer form.RemoveAll()

	value := map[string]any{}
	fields := map[string]any{}
	for key, items := range form.Value {
		if len(items) > reviewRequestMaxHeaderValues {
			items = items[:reviewRequestMaxHeaderValues]
		}
		copied := make([]string, 0, len(items))
		for _, item := range items {
			copied = append(copied, item)
		}
		fields[key] = copied
	}
	if len(fields) > reviewRequestMaxMapEntries {
		fields = limitReviewMap(fields, reviewRequestMaxMapEntries)
	}
	if len(fields) > 0 {
		value["fields"] = fields
	}

	files := map[string]any{}
	for key, headers := range form.File {
		if len(files) >= reviewRequestMaxMapEntries {
			break
		}
		items := make([]any, 0, reviewRequestMaxHeaderValues)
		for i, header := range headers {
			if i >= reviewRequestMaxHeaderValues {
				break
			}
			items = append(items, map[string]any{
				"name":         key,
				"filename":     header.Filename,
				"content_type": header.Header.Get("Content-Type"),
				"size":         header.Size,
			})
		}
		files["part_"+strconv.Itoa(len(files))] = items
	}
	if len(files) > 0 {
		value["files"] = files
	}
	return value, true
}

func marshalBoundedReviewRequestValue(value any) string {
	redacted := redactReviewRequestValue(value, 0)
	data, err := Marshal(redacted)
	if err != nil {
		return ""
	}
	if len(data) <= reviewRequestBodyMaxBytes {
		return string(data)
	}
	return truncateReviewRequestBytes(string(data), reviewRequestBodyMaxBytes)
}

func redactReviewRequestValue(value any, depth int) any {
	if depth >= reviewRequestMaxDepth {
		return "[omitted nested value]"
	}
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, minInt(len(keys), reviewRequestMaxMapEntries))
		for _, key := range keys {
			if len(out) >= reviewRequestMaxMapEntries {
				break
			}
			lower := strings.ToLower(key)
			if isReviewRequestSensitiveKey(lower) {
				out[key] = "***"
				continue
			}
			if isReviewRequestMediaKey(lower) {
				out[key] = "[omitted media]"
				continue
			}
			out[key] = redactReviewRequestValue(typed[key], depth+1)
		}
		return out
	case []any:
		limit := len(typed)
		if limit > reviewRequestMaxArrayItems {
			limit = reviewRequestMaxArrayItems
		}
		out := make([]any, 0, limit)
		for _, item := range typed[:limit] {
			out = append(out, redactReviewRequestValue(item, depth+1))
		}
		if len(typed) > limit {
			out = append(out, "[truncated]")
		}
		return out
	case string:
		if isReviewRequestMediaValue(typed) {
			return "[omitted media]"
		}
		return truncateRunes(maskReviewRequestFreeText(MaskSensitiveInfo(typed)), reviewRequestMaxStringRunes)
	default:
		return typed
	}
}

func redactReviewRequestHeaders(headers map[string][]string) map[string][]string {
	if len(headers) == 0 {
		return nil
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string][]string, minInt(len(keys), reviewRequestMaxHeaderCount))
	for _, key := range keys {
		if len(out) >= reviewRequestMaxHeaderCount {
			break
		}
		values := headers[key]
		if len(values) > reviewRequestMaxHeaderValues {
			values = values[:reviewRequestMaxHeaderValues]
		}
		copied := make([]string, 0, len(values))
		for _, value := range values {
			if isReviewRequestSensitiveHeader(key) {
				copied = append(copied, "***")
				continue
			}
			masked := maskReviewRequestFreeText(MaskSensitiveInfo(value))
			if isReviewRequestMediaValue(strings.TrimSpace(masked)) {
				copied = append(copied, "[omitted media]")
				continue
			}
			copied = append(copied, truncateRunes(masked, reviewRequestMaxHeaderRunes))
		}
		out[key] = copied
	}
	data, err := Marshal(out)
	if err == nil && len(data) > reviewRequestMaxHeadersBytes {
		// Keep the deterministic prefix of the map by removing the last keys.
		for len(out) > 0 {
			data, _ = Marshal(out)
			if len(data) <= reviewRequestMaxHeadersBytes {
				break
			}
			last := ""
			for key := range out {
				if key > last {
					last = key
				}
			}
			delete(out, last)
		}
	}
	return out
}

func isReviewRequestSensitiveKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	compact := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, lower)
	for _, part := range []string{
		"authorization", "proxyauthorization", "cookie", "apikey", "accesstoken", "refreshtoken",
		"token", "password", "passwd", "secret", "signature", "privatekey", "jwt", "sessiontoken",
		"clientsecret",
	} {
		if strings.Contains(compact, part) {
			return true
		}
	}
	return compact == "auth" || compact == "token" || compact == "key" || compact == "sign"
}

func isReviewRequestSensitiveHeader(key string) bool {
	lower := strings.ToLower(key)
	if isReviewRequestSensitiveKey(lower) {
		return true
	}
	return strings.Contains(lower, "websocket") || strings.Contains(lower, "sec-websocket-protocol")
}

func isReviewRequestMediaKey(key string) bool {
	if key == "files" || key == "filename" || key == "file_name" || key == "content_type" || key == "size" {
		return false
	}
	for _, part := range []string{"image", "audio", "video", "file", "media", "base64", "b64", "bytes"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func isReviewRequestMediaValue(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(lower, ";base64,") || strings.HasPrefix(lower, "data:") {
		return true
	}
	if utf8.RuneCountInString(value) < 128 || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	valid := 0
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("+/=_-", r) {
			valid++
		}
	}
	return valid*100 >= len([]rune(value))*95
}

func limitReviewMap(value map[string]any, max int) map[string]any {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]any, minInt(len(keys), max))
	for _, key := range keys {
		if len(out) >= max {
			break
		}
		out[key] = value[key]
	}
	return out
}

func truncateReviewRequestBytes(value string, maxBytes int) string {
	return truncateReviewRequestBytesWithSuffix(value, maxBytes, "...")
}

func truncateReviewRequestBytesWithSuffix(value string, maxBytes int, suffix string) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	if len(suffix) >= maxBytes {
		return value[:maxBytes]
	}
	available := maxBytes - len(suffix)
	for available > 0 && !utf8.RuneStart(value[available]) {
		available--
	}
	return value[:available] + suffix
}

func maskReviewRequestFreeText(value string) string {
	value = MaskReviewCredentialText(value)
	return reviewRequestLongOpaqueText.ReplaceAllString(value, "[omitted media]")
}

func sanitizeReviewRequestText(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			builder.WriteRune(r)
		case unicode.IsControl(r):
			builder.WriteByte(' ')
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
