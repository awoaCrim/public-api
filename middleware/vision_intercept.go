package middleware

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service/vision"

	"github.com/gin-gonic/gin"
)

// visionInterceptPaths are the request paths the vision interception
// middleware handles.
var visionInterceptPaths = map[string]bool{
	"/v1/chat/completions": true,
	"/v1/messages":         true,
	"/v1/responses":        true,
	"/chat/completions":    true,
	"/messages":            true,
}

type visionImageInterceptor func(*gin.Context, map[string]any, []vision.ImageEntry, dto.UserVisionSetting, string) error

// VisionIntercept replaces images with text descriptions when the user has
// vision interception enabled and the requested model matches the configured
// suffix. Failures are logged and the request continues unimpeded (fail-open);
// the flag "vision_intercepted" marks that interception took place.
func VisionIntercept() func(c *gin.Context) {
	return visionIntercept(vision.InterceptImages)
}

func visionIntercept(interceptImages visionImageInterceptor) func(c *gin.Context) {
	return func(c *gin.Context) {
		if !visionInterceptPaths[c.Request.URL.Path] {
			c.Next()
			return
		}
		userId := c.GetInt("id")
		userSetting, ok := c.Get(string(constant.ContextKeyUserSetting))
		if !ok {
			c.Next()
			return
		}
		setting, ok := userSetting.(dto.UserSetting)
		if !ok || setting.Vision == nil || !setting.Vision.Enabled || strings.TrimSpace(setting.Vision.VisionSuffix) == "" {
			c.Next()
			return
		}
		if strings.TrimSpace(setting.Vision.VisionModel) == "" {
			logger.LogWarn(c, fmt.Sprintf("[vision_intercept] user %d has vision interception enabled but no vision model configured; skipping", userId))
			c.Next()
			return
		}

		body, err := common.GetRequestBody(c)
		if err != nil {
			logger.LogWarn(c, fmt.Sprintf("[vision_intercept] failed to read request body: %s", err.Error()))
			c.Next()
			return
		}
		rawBody, err := readSeekerAll(body)
		if err != nil {
			logger.LogWarn(c, fmt.Sprintf("[vision_intercept] failed to read request body: %s", err.Error()))
			c.Next()
			return
		}
		if len(rawBody) == 0 {
			c.Next()
			return
		}

		var root map[string]any
		if err := common.Unmarshal(rawBody, &root); err != nil {
			logger.LogWarn(c, fmt.Sprintf("[vision_intercept] failed to parse request JSON: %s", err.Error()))
			c.Next()
			return
		}

		modelName, _ := root["model"].(string)
		if !strings.HasSuffix(modelName, setting.Vision.VisionSuffix) {
			c.Next()
			return
		}

		images, err := vision.ExtractImages(root)
		if err != nil {
			logger.LogWarn(c, fmt.Sprintf("[vision_intercept] %s", err.Error()))
			c.Next()
			return
		}
		if len(images) == 0 {
			c.Next()
			return
		}

		// The suffix is a Vision interception trigger only. Keep the client's
		// model value intact so downstream model mapping and billing continue to
		// see exactly the model that was requested.

		requestID := c.GetString(common.RequestIdKey)
		if err := interceptImages(c, root, images, *setting.Vision, requestID); err != nil {
			logger.LogWarn(c, fmt.Sprintf("[vision_intercept] failed to intercept images: %s", err.Error()))
			c.Next()
			return
		}

		newBody, err := common.Marshal(root)
		if err != nil {
			logger.LogWarn(c, fmt.Sprintf("[vision_intercept] failed to re-marshal request body: %s", err.Error()))
			c.Next()
			return
		}

		// Replace both the raw request body and the cached body storage so
		// downstream handlers (distributor, relay) see the rewritten body.
		c.Request.Body = io.NopCloser(bytes.NewReader(newBody))
		c.Request.ContentLength = int64(len(newBody))
		c.Set(common.KeyBodyStorage, nil)
		c.Set(common.KeyRequestBody, newBody)
		c.Set("vision_intercepted", true)

		c.Next()
	}
}

// readSeekerAll drains an io.Seeker whose concrete storage may or may not
// also implement io.Reader (e.g. disk-backed body storage).
func readSeekerAll(s io.Seeker) ([]byte, error) {
	if bs, ok := s.(common.BodyStorage); ok {
		return bs.Bytes()
	}
	if _, err := s.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if r, ok := s.(io.Reader); ok {
		return io.ReadAll(r)
	}
	return nil, fmt.Errorf("body storage does not support reading")
}
