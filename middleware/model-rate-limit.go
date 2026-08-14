package middleware

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/common/limiter"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/tidwall/gjson"
)

const (
	ModelRequestRateLimitCountMark        = "MRRL"
	ModelRequestRateLimitSuccessCountMark = "MRRLS"
	modelRateLimitTimeFormat              = "2006-01-02T15:04:05.000Z"
	rpmReservedMemberKey                  = "rpm_reserved_member"
	rpmMemoryReservationKey               = "rpm_memory_reservation"
)

const RPMReleaseSlotKey = "rpm_release_slot"

func MarkRPMFailure(c *gin.Context) {
	if c != nil {
		c.Set(RPMReleaseSlotKey, true)
	}
}

var rpmCheckAndReserveLua = redis.NewScript(`
local keyType = redis.call('TYPE', KEYS[1])
if type(keyType) == 'table' then keyType = keyType.ok end
if keyType ~= 'none' and keyType ~= 'zset' then
  redis.call('DEL', KEYS[1])
end
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[2] - ARGV[3])
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
local count = redis.call('ZCARD', KEYS[1])
if count > tonumber(ARGV[4]) then
  redis.call('ZREM', KEYS[1], ARGV[1])
  local oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
  local oldestScore = ARGV[2]
  if #oldest > 0 then oldestScore = oldest[2] end
  return {0, count - 1, oldestScore}
end
redis.call('PEXPIRE', KEYS[1], ARGV[3] + 60000)
return {1, count, 0}
`)

var rpmRecordCounter atomic.Int64
var rpmProcessPrefix = fmt.Sprintf("%s-%d", common.NodeName, os.Getpid())

func redisReplyIntegerForRPM(value interface{}) int64 {
	integer, err := redisReplyInteger(value)
	if err != nil {
		return 0
	}
	return integer
}

func extractModelName(c *gin.Context) string {
	if modelName := c.GetString("original_model"); modelName != "" {
		return modelName
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		storage, err := common.GetBodyStorage(c)
		if err == nil {
			body, readErr := storage.Bytes()
			if readErr == nil {
				_, _ = storage.Seek(0, io.SeekStart)
				return gjson.GetBytes(body, "model").String()
			}
		}
	}
	if strings.Contains(c.GetHeader("Content-Type"), gin.MIMEMultipartPOSTForm) {
		form, err := common.ParseMultipartFormReusable(c)
		if err == nil {
			defer form.RemoveAll()
			if values := form.Value["model"]; len(values) > 0 && values[0] != "" {
				return values[0]
			}
		}
	} else if modelName := c.PostForm("model"); modelName != "" {
		return modelName
	}
	return c.Query("model")
}

func checkRedisRateLimit(ctx context.Context, rdb *redis.Client, key string, maxCount int, duration int64) (bool, error) {
	if maxCount == 0 {
		return true, nil
	}
	length, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if length < int64(maxCount) {
		return true, nil
	}
	oldTimeString, err := rdb.LIndex(ctx, key, -1).Result()
	if err != nil {
		return false, err
	}
	oldTime, err := time.Parse(modelRateLimitTimeFormat, oldTimeString)
	if err != nil {
		return false, err
	}
	return time.Since(oldTime).Seconds() >= float64(duration), nil
}

func recordRedisRequest(ctx context.Context, rdb *redis.Client, key string, maxCount int) {
	if maxCount == 0 {
		return
	}
	pipeline := rdb.TxPipeline()
	pipeline.LPush(ctx, key, time.Now().UTC().Format(modelRateLimitTimeFormat))
	pipeline.LTrim(ctx, key, 0, int64(maxCount-1))
	pipeline.Expire(ctx, key, time.Duration(setting.ModelRequestRateLimitDurationMinutes)*time.Minute)
	if _, err := pipeline.Exec(ctx); err != nil {
		logger.LogError(ctx, "failed to record successful model request: "+err.Error())
	}
}

func rpmRedisKey(userID int) string {
	return fmt.Sprintf("rateLimit:rpm:%d", userID)
}

func reserveRedisRPMSlot(c *gin.Context, userID int, durationSeconds int64, maxRPM int) (bool, int, int, error) {
	nowMilliseconds := time.Now().UnixMilli()
	windowMilliseconds := durationSeconds * 1000
	member := fmt.Sprintf("%d-%s-%d", nowMilliseconds, rpmProcessPrefix, rpmRecordCounter.Add(1))
	result, err := rpmCheckAndReserveLua.Run(
		c.Request.Context(),
		common.RDB,
		[]string{rpmRedisKey(userID)},
		member,
		nowMilliseconds,
		windowMilliseconds,
		maxRPM,
	).Result()
	if err != nil {
		return true, 0, 0, err
	}
	values, ok := result.([]interface{})
	if !ok || len(values) != 3 {
		return true, 0, 0, fmt.Errorf("unexpected RPM limiter reply %T", result)
	}
	allowed := redisReplyIntegerForRPM(values[0]) == 1
	inWindow := int(redisReplyIntegerForRPM(values[1]))
	if allowed {
		c.Set(rpmReservedMemberKey, member)
		return true, inWindow, 0, nil
	}
	oldestMilliseconds := redisReplyIntegerForRPM(values[2])
	retryAfter := durationSeconds
	if oldestMilliseconds > 0 {
		retryAfter -= (nowMilliseconds - oldestMilliseconds) / 1000
	}
	if retryAfter < 1 {
		retryAfter = 1
	}
	return false, inWindow + 1, int(retryAfter), nil
}

func shouldReleaseRPMSlot(c *gin.Context) bool {
	return c.Writer.Status() >= http.StatusBadRequest || c.GetBool(RPMReleaseSlotKey)
}

func releaseRedisRPMSlotOnFailure(c *gin.Context, userID int) {
	value, exists := c.Get(rpmReservedMemberKey)
	if !exists {
		return
	}
	member, _ := value.(string)
	if member == "" || !shouldReleaseRPMSlot(c) {
		return
	}
	if err := common.RDB.ZRem(context.Background(), rpmRedisKey(userID), member).Err(); err != nil {
		logger.LogError(c.Request.Context(), "failed to release RPM slot: "+err.Error())
	}
}

func enforceRPM(c *gin.Context, durationSeconds int64) bool {
	configuration := operation_setting.GetRateLimitBanSetting()
	if !configuration.Enabled || configuration.MaxRPM <= 0 {
		return true
	}
	modelName := extractModelName(c)
	if operation_setting.IsModelRateLimitWhitelisted(modelName) {
		return true
	}
	userID := c.GetInt("id")
	if common.RedisEnabled {
		allowed, current, retryAfter, err := reserveRedisRPMSlot(c, userID, durationSeconds, configuration.MaxRPM)
		if err != nil {
			logger.LogError(c.Request.Context(), "RPM Redis check failed; request allowed: "+err.Error())
			return true
		}
		if allowed {
			return true
		}
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("RPM limit exceeded user=%d model=%s current=%d limit=%d", userID, modelName, current, configuration.MaxRPM))
		triggerRateLimitReview(c, userID, modelName, current, configuration.MaxRPM)
		service.WriteRateLimitError(c, service.RateLimitExceededMessage, retryAfter, configuration.MaxRPM, 0)
		return false
	}

	key := fmt.Sprintf("RPM:user:%d", userID)
	allowed, reservation, current := inMemoryRateLimiter.Reserve(key, configuration.MaxRPM, durationSeconds)
	if allowed {
		if reservation != nil {
			c.Set(rpmMemoryReservationKey, reservation)
		}
		return true
	}
	logger.LogWarn(c.Request.Context(), fmt.Sprintf("RPM limit exceeded user=%d model=%s current=%d limit=%d", userID, modelName, current, configuration.MaxRPM))
	triggerRateLimitReview(c, userID, modelName, current, configuration.MaxRPM)
	service.WriteRateLimitError(c, service.RateLimitExceededMessage, int(durationSeconds), configuration.MaxRPM, 0)
	return false
}

func releaseMemoryRPMSlotOnFailure(c *gin.Context) {
	value, exists := c.Get(rpmMemoryReservationKey)
	if !exists || !shouldReleaseRPMSlot(c) {
		return
	}
	reservation, _ := value.(*common.InMemoryRateLimitReservation)
	reservation.Release()
}

func extractStreamFlag(c *gin.Context) bool {
	if value, ok := c.Get(string(constant.ContextKeyIsStream)); ok {
		if isStream, typeOK := value.(bool); typeOK {
			return isStream
		}
	}
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		return false
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return false
	}
	reader, err := storage.NewReader()
	if err != nil {
		return false
	}
	defer reader.Close()
	body, err := io.ReadAll(io.LimitReader(reader, 64<<10))
	return err == nil && gjson.GetBytes(body, "stream").Bool()
}

func triggerRateLimitReview(c *gin.Context, userID int, modelName string, currentValue int, limitValue int) {
	trigger := service.RateLimitReviewTrigger{
		UserID:         userID,
		ModelName:      modelName,
		Endpoint:       c.Request.URL.Path,
		CurrentValue:   currentValue,
		LimitValue:     limitValue,
		RequestSnippet: common.ExtractRequestSnippet(c),
		ClientIP:       c.ClientIP(),
		IsStream:       extractStreamFlag(c),
	}
	go func() {
		if err := service.EnqueueRateLimitReview(context.Background(), trigger); err != nil {
			logger.LogError(context.Background(), "failed to enqueue rate-limit review: "+err.Error())
		}
	}()
}

func redisRateLimitHandler(duration int64, totalMaxCount, successMaxCount int) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("id")
		defer releaseRedisRPMSlotOnFailure(c, userID)
		if !enforceRPM(c, duration) {
			return
		}

		ctx := c.Request.Context()
		successKey := fmt.Sprintf("rateLimit:%s:%d", ModelRequestRateLimitSuccessCountMark, userID)
		allowed, err := checkRedisRateLimit(ctx, common.RDB, successKey, successMaxCount, duration)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
			return
		}
		if !allowed {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您已达到请求数限制：%d分钟内最多请求%d次", setting.ModelRequestRateLimitDurationMinutes, successMaxCount))
			return
		}
		if totalMaxCount > 0 {
			tokenBucket := limiter.New(ctx, common.RDB)
			allowed, err = tokenBucket.Allow(ctx, fmt.Sprintf("rateLimit:%d", userID), limiter.WithCapacity(int64(totalMaxCount)*duration), limiter.WithRate(int64(totalMaxCount)), limiter.WithRequested(duration))
			if err != nil {
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}
			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您已达到总请求数限制：%d分钟内最多请求%d次，包括失败次数，请检查您的请求是否正确", setting.ModelRequestRateLimitDurationMinutes, totalMaxCount))
				return
			}
		}
		c.Next()
		if c.Writer.Status() < http.StatusBadRequest {
			recordRedisRequest(ctx, common.RDB, successKey, successMaxCount)
		}
	}
}

func memoryRateLimitHandler(duration int64, totalMaxCount, successMaxCount int) gin.HandlerFunc {
	inMemoryRateLimiter.Init(time.Duration(setting.ModelRequestRateLimitDurationMinutes) * time.Minute)
	return func(c *gin.Context) {
		defer releaseMemoryRPMSlotOnFailure(c)
		if !enforceRPM(c, duration) {
			return
		}
		userID := strconv.Itoa(c.GetInt("id"))
		if totalMaxCount > 0 && !inMemoryRateLimiter.Request(ModelRequestRateLimitCountMark+userID, totalMaxCount, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}
		if successMaxCount > 0 && !inMemoryRateLimiter.Request(ModelRequestRateLimitSuccessCountMark+userID+"_check", successMaxCount, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}
		c.Next()
	}
}

func ClearUserRateLimitKeys(userID int) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user id")
	}
	memoryKeys := []string{
		fmt.Sprintf("RPM:user:%d", userID),
		ModelRequestRateLimitCountMark + strconv.Itoa(userID),
		ModelRequestRateLimitSuccessCountMark + strconv.Itoa(userID),
		ModelRequestRateLimitSuccessCountMark + strconv.Itoa(userID) + "_check",
	}
	for _, key := range memoryKeys {
		inMemoryRateLimiter.Delete(key)
	}
	if !common.RedisEnabled || common.RDB == nil {
		return nil
	}
	keys := []string{
		rpmRedisKey(userID),
		fmt.Sprintf("rateLimit:%s:%d", ModelRequestRateLimitSuccessCountMark, userID),
		fmt.Sprintf("rateLimit:%s", strconv.Itoa(userID)),
		fmt.Sprintf("rateLimit:%d", userID),
	}
	err := common.RDB.Del(context.Background(), keys...).Err()
	if err != nil {
		return err
	}
	return deleteRedisRateLimitKeys(fmt.Sprintf("rateLimit:rpm:%d:*", userID))
}

func deleteRedisRateLimitKeys(pattern string) error {
	var cursor uint64
	for {
		keys, nextCursor, err := common.RDB.Scan(context.Background(), cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := common.RDB.Del(context.Background(), keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			return nil
		}
	}
}

func ModelRequestRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		if c.GetInt("role") >= common.RoleRootUser {
			c.Next()
			return
		}
		rpmEnabled := operation_setting.GetRateLimitBanSetting().Enabled
		if !setting.ModelRequestRateLimitEnabled && !rpmEnabled {
			c.Next()
			return
		}
		duration := int64(setting.ModelRequestRateLimitDurationMinutes * 60)
		totalMaxCount := 0
		successMaxCount := 0
		if setting.ModelRequestRateLimitEnabled {
			totalMaxCount = setting.ModelRequestRateLimitCount
			successMaxCount = setting.ModelRequestRateLimitSuccessCount
			group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
			if group == "" {
				group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
			}
			if groupTotal, groupSuccess, found := setting.GetGroupRateLimit(group); found {
				totalMaxCount = groupTotal
				successMaxCount = groupSuccess
			}
		}
		if common.RedisEnabled {
			redisRateLimitHandler(duration, totalMaxCount, successMaxCount)(c)
			return
		}
		memoryRateLimitHandler(duration, totalMaxCount, successMaxCount)(c)
	}
}
