package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math/bits"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/corona10/goimagehash"
	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
	"github.com/samber/lo"
	"golang.org/x/sync/singleflight"
)

// Security bounds for vision interception. Every image entering the pipeline
// (data URI or downloaded URL) must fit inside these limits before any decode
// or upstream call.
const (
	MinPhashThreshold     = 0
	MaxPhashThreshold     = 64
	MaxBase64ImageBytes   = 20 * 1024 * 1024 // encoded data URI cap
	maxDecodedImageBytes  = 15 * 1024 * 1024 // decoded byte cap
	maxImageDimension     = 8192             // per-side pixel cap
	maxImagePixels        = 20 * 1000 * 1000 // total pixel cap (~20MP)
	maxImageDownloadBytes = 15 * 1024 * 1024 // URL download cap
	MaxImagesPerRequest   = 16               // total extracted image cap per request
	imageDownloadTimeout  = 30 * time.Second
)

// DefaultPromptTemplate is used when a legacy or newly-created Vision setting
// does not contain a prompt. It deliberately asks for evidence-preserving
// descriptions instead of encouraging unsupported inference.
const DefaultPromptTemplate = `Describe the provided image as accurately and comprehensively as possible, based strictly on visible evidence.

Preserve all information that may be useful to another AI, including subjects, objects, appearance, actions, colors, materials, spatial relationships, layout, foreground/background, lighting, perspective, composition, UI elements, symbols, diagrams, charts, and visible text.

Transcribe readable text exactly. Mark unreadable text as [illegible].

Do not guess, invent, or infer unsupported identities, locations, relationships, intentions, events, brands, or hidden details. Explicitly mark uncertain observations as uncertain.

Prioritize factual accuracy, spatial relationships, and information preservation over elegant prose or brevity.`

// NormalizePromptTemplate returns the configured prompt, or the canonical
// default for blank values. Non-blank custom prompts are preserved verbatim.
func NormalizePromptTemplate(prompt string) string {
	if strings.TrimSpace(prompt) == "" {
		return DefaultPromptTemplate
	}
	return prompt
}

// IsValidPhashThreshold reports whether a threshold is within the 64-bit
// perceptual-hash distance domain.
func IsValidPhashThreshold(threshold int) bool {
	return threshold >= MinPhashThreshold && threshold <= MaxPhashThreshold
}

// NormalizePhashThreshold disables perceptual grouping for malformed legacy
// values so they cannot accidentally merge unrelated images.
func NormalizePhashThreshold(threshold int) int {
	if !IsValidPhashThreshold(threshold) {
		return MinPhashThreshold
	}
	return threshold
}

// imageDescCache is the cross-request LRU (TTL 10m, 1000 entries). Keys
// include user/model/prompt identity, so descriptions never leak across
// users, models or prompts.
var imageDescCache = hot.NewHotCache[string, string](hot.LRU, 1000).
	WithTTL(10 * time.Minute).Build()

// requestCacheEntry deduplicates repeated images inside one request. The
// inner map is mutex-guarded because parallel group analysis shares one
// entry.
type requestCacheEntry struct {
	mu sync.Mutex
	m  map[string]string
}

var requestCache = hot.NewHotCache[string, *requestCacheEntry](hot.LRU, 5000).
	WithTTL(5 * time.Minute).Build()

// requestCacheInitMu prevents two concurrent misses from creating competing
// entries.
var requestCacheInitMu sync.Mutex

// sfGroup merges concurrent identical sub-calls (cache stampede guard).
var sfGroup singleflight.Group

// phashEntry records one fuzzy-cache entry with full isolation identity.
type phashEntry struct {
	userId     int
	model      string
	promptHash string
	hash       uint64
	desc       string
	ts         int64
}

const phashCacheTTL = 10 * time.Minute

type phashRingCache struct {
	mu      sync.RWMutex
	entries []phashEntry
	count   int
	max     int
}

func newPhashRingCache(max int) *phashRingCache {
	return &phashRingCache{entries: make([]phashEntry, max), max: max}
}

func (c *phashRingCache) lookup(userId int, model, promptHash string, hash uint64, threshold int) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n := c.count
	if n > c.max {
		n = c.max
	}
	now := time.Now().Unix()
	for i := 0; i < n; i++ {
		e := c.entries[i]
		if now-e.ts > int64(phashCacheTTL.Seconds()) {
			continue
		}
		if e.userId == userId && e.model == model && e.promptHash == promptHash && bits.OnesCount64(e.hash^hash) <= threshold {
			return e.desc, true
		}
	}
	return "", false
}

func (c *phashRingCache) store(userId int, model, promptHash string, hash uint64, desc string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	idx := c.count % c.max
	c.entries[idx] = phashEntry{userId: userId, model: model, promptHash: promptHash, hash: hash, desc: desc, ts: time.Now().Unix()}
	c.count++
}

var phashCache = newPhashRingCache(500)

// ImageBlock is one extracted image reference.
type ImageBlock struct {
	MessageIdx int
	ContentIdx int
	NestedPath string // e.g. "messages.0.content.2.content.0"; empty = top-level
	ImageURL   string // HTTP(S) URL or data: URI
	MediaType  string
}

// DecodeBase64Image decodes a data: URI into an image with bounded decoding.
func DecodeBase64Image(dataURI string) (image.Image, error) {
	if !strings.HasPrefix(dataURI, "data:") {
		return nil, fmt.Errorf("not a data URI")
	}
	commaIdx := strings.IndexByte(dataURI, ',')
	if commaIdx < 0 {
		return nil, fmt.Errorf("invalid data URI: missing comma")
	}
	b64Data := dataURI[commaIdx+1:]
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(b64Data))
	limited := io.LimitReader(decoder, maxDecodedImageBytes+1)

	cfg, _, err := image.DecodeConfig(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read image header from base64: %w", err)
	}
	if err := validateImageSize(cfg); err != nil {
		return nil, err
	}

	decoder2 := base64.NewDecoder(base64.StdEncoding, strings.NewReader(b64Data))
	img, _, err := image.Decode(io.LimitReader(decoder2, maxDecodedImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image from base64: %w", err)
	}
	return img, nil
}

// DownloadImageForPhash safely downloads a URL image for pHash computation:
// SSRF validation, bounded content-length, bounded streaming read, then size
// validation and decode.
func DownloadImageForPhash(imageURL string) (image.Image, error) {
	if err := ValidateImageURL(imageURL); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), imageDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build image request: %w", err)
	}
	resp, err := service.GetSSRFProtectedHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image download returned status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxImageDownloadBytes {
		return nil, fmt.Errorf("image download exceeds size limit (%d bytes)", maxImageDownloadBytes)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read image body: %w", err)
	}
	if len(data) > maxImageDownloadBytes {
		return nil, fmt.Errorf("image download exceeds size limit (%d bytes)", maxImageDownloadBytes)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to read image header from URL: %w", err)
	}
	if err := validateImageSize(cfg); err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image from URL: %w", err)
	}
	return img, nil
}

// ValidateImageURL enforces SSRF constraints on a downloaded image URL using
// the system fetch settings.
func ValidateImageURL(imageURL string) error {
	if err := service.ValidateSSRFProtectedFetchURL(imageURL); err != nil {
		return fmt.Errorf("image url blocked: %w", err)
	}
	return nil
}

func validateImageSize(cfg image.Config) error {
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("invalid image dimensions: %dx%d", cfg.Width, cfg.Height)
	}
	if cfg.Width > maxImageDimension || cfg.Height > maxImageDimension {
		return fmt.Errorf("image dimensions %dx%d exceed limit %d", cfg.Width, cfg.Height, maxImageDimension)
	}
	if cfg.Width*cfg.Height > maxImagePixels {
		return fmt.Errorf("image pixel count %d exceeds limit %d", cfg.Width*cfg.Height, maxImagePixels)
	}
	return nil
}

// ComputePhash computes the perceptual hash of an image.
func ComputePhash(img image.Image) (uint64, error) {
	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return 0, fmt.Errorf("failed to compute perceptual hash: %w", err)
	}
	return hash.GetHash(), nil
}

// HammingDistance returns the hamming distance of two hashes.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// hashPrompt returns a stable prompt identity for cache isolation.
func hashPrompt(prompt string) string {
	return common.GenerateHMAC(prompt)
}

// buildCacheKey derives the LRU cache key including user/model/prompt and the
// image identity so entries never cross users, models or prompts.
func buildCacheKey(userId int, imageURL string, config dto.UserVisionSetting) string {
	prompt := NormalizePromptTemplate(config.PromptTemplate)
	return fmt.Sprintf("%d|%s|%s|%s", userId, config.VisionModel, hashPrompt(prompt), imageURL)
}

func storeInRequestCache(requestID, imageURL, desc string) {
	if requestID == "" {
		return
	}
	requestCacheInitMu.Lock()
	entry, found, _ := requestCache.Get(requestID)
	if !found || entry == nil {
		entry = &requestCacheEntry{m: map[string]string{}}
		requestCache.Set(requestID, entry)
	}
	requestCacheInitMu.Unlock()
	entry.mu.Lock()
	entry.m[imageURL] = desc
	entry.mu.Unlock()
}

// LookupCachedDescription only consults L4 (pHash) and L2 (LRU) caches; it
// never makes an upstream call. Used for historical images.
func LookupCachedDescription(userId int, imageURL string, config dto.UserVisionSetting, phash *uint64) (string, bool) {
	if phash != nil && config.PhashThreshold > 0 {
		promptHash := hashPrompt(NormalizePromptTemplate(config.PromptTemplate))
		if desc, found := phashCache.lookup(userId, config.VisionModel, promptHash, *phash, config.PhashThreshold); found {
			return desc, true
		}
	}
	cacheKey := buildCacheKey(userId, imageURL, config)
	if desc, found, _ := imageDescCache.Get(cacheKey); found {
		return desc, true
	}
	return "", false
}

// AnalyzeImage obtains a text description of one image through the vision
// model. c is the parent request context (used for channel selection and
// billing identity); the description is billed against the same token/group
// as the parent request. Returns (description, cacheHit, error). Failures
// must surface to the caller — no silent placeholder.
func AnalyzeImage(c *gin.Context, ctx context.Context, config dto.UserVisionSetting, imageURL string, requestID string, phash *uint64) (desc string, cached bool, err error) {
	if imageURL == "" {
		return "", false, fmt.Errorf("empty image URL")
	}

	config.PromptTemplate = NormalizePromptTemplate(config.PromptTemplate)
	userId := c.GetInt("id")

	// L4: cross-request fuzzy pHash cache.
	promptHash := hashPrompt(config.PromptTemplate)
	if phash != nil && config.PhashThreshold > 0 {
		if desc, found := phashCache.lookup(userId, config.VisionModel, promptHash, *phash, config.PhashThreshold); found {
			storeInRequestCache(requestID, imageURL, desc)
			return desc, true, nil
		}
	}

	// L1: per-request URL dedup.
	if requestID != "" {
		if entry, found, _ := requestCache.Get(requestID); found && entry != nil {
			entry.mu.Lock()
			desc, exists := entry.m[imageURL]
			entry.mu.Unlock()
			if exists {
				return desc, true, nil
			}
		}
	}

	// L2: cross-request LRU with full identity key.
	cacheKey := buildCacheKey(userId, imageURL, config)
	if desc, found, _ := imageDescCache.Get(cacheKey); found {
		storeInRequestCache(requestID, imageURL, desc)
		return desc, true, nil
	}

	if strings.HasPrefix(imageURL, "data:") {
		if len(imageURL) > MaxBase64ImageBytes {
			return "", false, fmt.Errorf("base64 image exceeds size limit (%d bytes)", MaxBase64ImageBytes)
		}
	}
	if strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
		if err := ValidateImageURL(imageURL); err != nil {
			return "", false, err
		}
	}

	// Context id injected into the prompt prevents upstream prompt caching
	// from serving a stale description for a different conversation.
	promptText := config.PromptTemplate
	if requestID != "" {
		promptText = promptText + "\n[context_id:" + requestID + "]"
	}

	// L3: singleflight merges concurrent identical calls (billing-safe: key
	// includes user/model/prompt identity).
	callResult, err, _ := sfGroup.Do(cacheKey, func() (interface{}, error) {
		return analyzeImageWithRelay(c, ctx, config, imageURL, promptText)
	})
	if err != nil {
		return "", false, err
	}
	desc = callResult.(string)
	imageDescCache.Set(cacheKey, desc)
	storeInRequestCache(requestID, imageURL, desc)
	if phash != nil && config.PhashThreshold > 0 {
		phashCache.store(userId, config.VisionModel, promptHash, *phash, desc)
	}
	return desc, false, nil
}

// analyzeImageWithRelay runs the actual vision sub-call through the relay
// pipeline: channel selection, model mapping, price estimation, pre-consume,
// upstream request and post-consume settlement, all inheriting the parent
// token/group identity.
func analyzeImageWithRelay(c *gin.Context, ctx context.Context, config dto.UserVisionSetting, imageURL, promptText string) (string, error) {
	tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	retryParam := &service.RetryParam{
		Ctx:         c,
		TokenGroup:  tokenGroup,
		ModelName:   config.VisionModel,
		RequestPath: "/v1/chat/completions",
	}
	channel, _, err := service.CacheGetRandomSatisfiedChannel(retryParam)
	if err != nil {
		return "", fmt.Errorf("failed to find channel for vision model '%s': %w", config.VisionModel, err)
	}
	if channel == nil {
		return "", fmt.Errorf("no available channel for vision model '%s'", config.VisionModel)
	}

	// Build an isolated child context carrying the parent identity and the
	// selected channel context, so the sub-call bills against the same token
	// and group and respects the same channel settings.
	visionCtx := newVisionSubContext(c, ctx)
	if apiErr := setupVisionChannelContext(visionCtx, channel, config.VisionModel); apiErr != nil {
		return "", fmt.Errorf("failed to setup vision channel context: %s", apiErr.Error())
	}

	request := &dto.GeneralOpenAIRequest{
		Model:     config.VisionModel,
		Stream:    lo.ToPtr(false),
		Messages:  []dto.Message{{Role: "user"}},
		MaxTokens: lo.ToPtr(uint(4096)),
	}
	request.Messages[0].SetMediaContent([]dto.MediaContent{
		{Type: dto.ContentTypeText, Text: promptText},
		{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: imageURL, Detail: "high"}},
	})

	info, err := relaycommon.GenRelayInfo(visionCtx, types.RelayFormatOpenAI, request, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate relay info for vision: %w", err)
	}
	info.InitChannelMeta(visionCtx)

	if err := helper.ModelMappedHelper(visionCtx, info, request); err != nil {
		return "", fmt.Errorf("failed to map model for vision: %w", err)
	}

	// Independent billing lifecycle: estimate price, pre-consume (respecting
	// billing preference incl. subscriptions), and refund automatically if
	// the upstream call fails before settlement.
	meta := &types.TokenCountMeta{MaxTokens: int(lo.FromPtrOr(request.MaxTokens, uint(0)))}
	estTokens := service.CountTextToken(promptText, config.VisionModel)
	info.SetEstimatePromptTokens(estTokens)
	priceData, err := helper.ModelPriceHelper(visionCtx, info, estTokens, meta)
	if err != nil {
		return "", fmt.Errorf("failed to calculate vision model price: %w", err)
	}
	info.PriceData = priceData
	if !priceData.FreeModel {
		if apiErr := service.PreConsumeBilling(visionCtx, priceData.QuotaToPreConsume, info); apiErr != nil {
			return "", fmt.Errorf("pre-consume billing failed for vision model '%s': %s", config.VisionModel, apiErr.Error())
		}
		defer func() {
			if info.Billing != nil && info.Billing.NeedsRefund() {
				info.Billing.Refund(visionCtx)
			}
		}()
	}

	adaptor := relay.GetAdaptor(info.ApiType)
	if adaptor == nil {
		return "", fmt.Errorf("invalid api type %d for vision channel #%d", info.ApiType, channel.Id)
	}
	adaptor.Init(info)

	convertedRequest, err := adaptor.ConvertOpenAIRequest(visionCtx, info, request)
	if err != nil {
		return "", fmt.Errorf("failed to convert vision request: %w", err)
	}
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal vision request: %w", err)
	}
	body, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return "", fmt.Errorf("failed to build vision request body: %w", err)
	}
	defer closer.Close()

	resp, err := adaptor.DoRequest(visionCtx, info, body)
	if err != nil {
		return "", fmt.Errorf("vision model request failed: %w", err)
	}
	httpResp, ok := resp.(*http.Response)
	if !ok || httpResp == nil {
		return "", fmt.Errorf("vision adaptor returned an unexpected response type")
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vision model returned http %d", httpResp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read vision response: %w", err)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *dto.Usage `json:"usage"`
	}
	if err := common.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse vision response: %w", err)
	}
	description, err := visionContentText(parsed.Choices)
	if err != nil {
		return "", err
	}

	usage := parsed.Usage
	if usage == nil {
		usage = &dto.Usage{PromptTokens: estTokens, CompletionTokens: 0, TotalTokens: estTokens}
	}
	// Settle like a normal text request: quota, consume log and audit all
	// flow through the standard post-consume path.
	service.PostTextConsumeQuota(visionCtx, info, usage, nil)

	return description, nil
}

// visionContentText extracts choices[0].message.content text (string or
// structured array form).
func visionContentText(choices []struct {
	Message struct {
		Content any `json:"content"`
	} `json:"message"`
}) (string, error) {
	if len(choices) == 0 {
		return "", fmt.Errorf("vision response has no choices")
	}
	switch content := choices[0].Message.Content.(type) {
	case string:
		if content == "" {
			return "", fmt.Errorf("vision response content is empty")
		}
		return content, nil
	case []any:
		var text string
		for _, item := range content {
			if m, ok := item.(map[string]any); ok && m["type"] == "text" {
				if t, ok := m["text"].(string); ok {
					text += t
				}
			}
		}
		if text == "" {
			return "", fmt.Errorf("vision response content is empty")
		}
		return text, nil
	default:
		return "", fmt.Errorf("unsupported vision response content format")
	}
}

// setupVisionChannelContext populates the child context with the selected
// channel's full context so the sub-call respects channel settings, model
// mapping, multi-key rotation and auto-ban exactly like a normal relay
// request. This mirrors middleware.SetupContextForSelectedChannel but lives
// here to avoid a middleware <-> vision import cycle.
func setupVisionChannelContext(c *gin.Context, channel *model.Channel, modelName string) *types.NewAPIError {
	c.Set("original_model", modelName)
	if channel == nil {
		return types.NewError(fmt.Errorf("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	common.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
	common.SetContextKey(c, constant.ContextKeyChannelName, channel.Name)
	common.SetContextKey(c, constant.ContextKeyChannelType, channel.Type)
	common.SetContextKey(c, constant.ContextKeyChannelCreateTime, channel.CreatedTime)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, channel.GetSetting())
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, channel.GetOtherSettings())
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, channel.GetParamOverride())
	common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, channel.GetHeaderOverride())
	if nil != channel.OpenAIOrganization && *channel.OpenAIOrganization != "" {
		common.SetContextKey(c, constant.ContextKeyChannelOrganization, *channel.OpenAIOrganization)
	}
	common.SetContextKey(c, constant.ContextKeyChannelAutoBan, channel.GetAutoBan())
	common.SetContextKey(c, constant.ContextKeyChannelModelMapping, channel.GetModelMapping())
	common.SetContextKey(c, constant.ContextKeyChannelStatusCodeMapping, channel.GetStatusCodeMapping())

	key, index, apiErr := channel.GetNextEnabledKey()
	if apiErr != nil {
		return apiErr
	}
	if channel.ChannelInfo.IsMultiKey {
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
		common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, index)
	} else {
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, false)
	}
	common.SetContextKey(c, constant.ContextKeyChannelKey, key)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, channel.GetBaseURL())

	// A model pinned to a fixed endpoint may only be served through that
	// endpoint; anything else is rejected outright.
	if err := model.CheckChannelModelFixedEndpoint(channel.Id, modelName, channel.GetBaseURL()); err != nil {
		return types.NewError(err, types.ErrorCodeFixedEndpointMismatch, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusForbidden))
	}
	return nil
}

// newVisionSubContext builds an isolated gin context carrying the parent's
// token/group/user identity, so the vision sub-call bills like the parent
// request without mutating the parent context.
func newVisionSubContext(parent *gin.Context, ctx context.Context) *gin.Context {
	w := &visionResponseWriter{header: make(http.Header)}
	visionCtx, _ := gin.CreateTestContext(w)
	visionCtx.Request = parent.Request.Clone(ctx)

	userId := parent.GetInt("id")
	visionCtx.Set("id", userId)
	if userId > 0 {
		if userCache, err := model.GetUserCache(userId); err == nil && userCache != nil {
			userCache.WriteContext(visionCtx)
		}
	}
	for _, key := range []constant.ContextKey{
		constant.ContextKeyTokenId,
		constant.ContextKeyTokenKey,
		constant.ContextKeyTokenUnlimited,
		constant.ContextKeyTokenGroup,
		constant.ContextKeyTokenCrossGroupRetry,
		constant.ContextKeyTokenSpecificChannelId,
		constant.ContextKeyTokenModelLimitEnabled,
		constant.ContextKeyTokenModelLimit,
		constant.ContextKeyUsingGroup,
		constant.ContextKeyAutoGroup,
		constant.ContextKeyAutoGroupIndex,
		constant.ContextKeyAutoGroupRetryIndex,
		constant.ContextKeyUserQuota,
		constant.ContextKeyUserStatus,
		constant.ContextKeyUserEmail,
		constant.ContextKeyUserName,
		constant.ContextKeyUserSetting,
		constant.ContextKeyUserGroup,
	} {
		if v, exists := common.GetContextKey(parent, key); exists {
			common.SetContextKey(visionCtx, key, v)
		}
	}
	return visionCtx
}

// visionResponseWriter is a discard response writer for the isolated sub-call
// context; the sub-call never writes to the client.
type visionResponseWriter struct {
	header http.Header
}

func (w *visionResponseWriter) Header() http.Header         { return w.header }
func (w *visionResponseWriter) WriteHeader(statusCode int)  {}
func (w *visionResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
