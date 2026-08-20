package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func runRPMLua(t *testing.T, rdb *redis.Client, key, member string, nowMs, windowMs int64, maxRPM int) (int64, int64, int64) {
	t.Helper()
	result, err := rpmCheckAndReserveLua.Run(context.Background(), rdb, []string{key}, member, nowMs, windowMs, maxRPM).Result()
	require.NoError(t, err)
	values, ok := result.([]interface{})
	require.True(t, ok)
	require.Len(t, values, 3)
	return redisReplyIntegerForRPM(values[0]), redisReplyIntegerForRPM(values[1]), redisReplyIntegerForRPM(values[2])
}

func TestRPMSlidingWindowReservesOnlyAllowedRequests(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	const maxRPM = 5
	const windowMs = int64(60_000)
	nowMs := time.Now().UnixMilli()
	key := "rateLimit:rpm:test-user"

	for index := 1; index <= maxRPM; index++ {
		allowed, count, _ := runRPMLua(t, rdb, key, fmt.Sprintf("request-%d", index), nowMs+int64(index), windowMs, maxRPM)
		assert.EqualValues(t, 1, allowed)
		assert.EqualValues(t, index, count)
	}

	allowed, count, oldestScore := runRPMLua(t, rdb, key, "rejected", nowMs+6, windowMs, maxRPM)
	assert.Zero(t, allowed)
	assert.EqualValues(t, maxRPM, count)
	assert.NotZero(t, oldestScore)
	cardinality, err := rdb.ZCard(context.Background(), key).Result()
	require.NoError(t, err)
	assert.EqualValues(t, maxRPM, cardinality)

	allowed, count, _ = runRPMLua(t, rdb, key, "after-window", nowMs+windowMs+1000, windowMs, maxRPM)
	assert.EqualValues(t, 1, allowed)
	assert.EqualValues(t, 1, count)
}

func TestRPMSlidingWindowMigratesLegacyKeyTypes(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	key := "rateLimit:rpm:legacy-user"
	require.NoError(t, rdb.RPush(context.Background(), key, "legacy").Err())

	allowed, count, _ := runRPMLua(t, rdb, key, "current", time.Now().UnixMilli(), 60_000, 5)
	assert.EqualValues(t, 1, allowed)
	assert.EqualValues(t, 1, count)
	keyType, err := rdb.Type(context.Background(), key).Result()
	require.NoError(t, err)
	assert.Equal(t, "zset", keyType)
}

func TestRPMModelExtractionPreservesMultipartBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1"))
	file, err := writer.CreateFormFile("image", "image.png")
	require.NoError(t, err)
	_, err = file.Write([]byte("fake image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())

	assert.Equal(t, "gpt-image-1", extractModelName(context))
	form, err := common.ParseMultipartFormReusable(context)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, form.RemoveAll()) })
	assert.Equal(t, "gpt-image-1", form.Value["model"][0])
	require.Len(t, form.File["image"], 1)
	image, err := form.File["image"][0].Open()
	require.NoError(t, err)
	defer image.Close()
	imageBytes, err := io.ReadAll(image)
	require.NoError(t, err)
	assert.Equal(t, []byte("fake image"), imageBytes)
}

func TestRPMMiddlewareReturnsOpenAICompatible429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// The 429 path asynchronously enqueues a rate-limit review, which
	// resolves the user record through the real enqueue pipeline; provide an
	// explicit test database so the audit task lands as skipped instead of
	// hitting a nil DB.
	previousDB := model.DB
	previousMainType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.LLMReviewTask{}, &model.LLMReviewAttempt{}, &model.LLMReviewGrace{}, &model.LLMReviewCalibration{}))
	require.NoError(t, db.Create(&model.User{Id: 73, Username: "rpm429", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}).Error)
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousMainType)
	})

	previousRedisEnabled, previousRDB := common.RedisEnabled, common.RDB
	server := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		require.NoError(t, common.RDB.Close())
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
	})

	setting := operation_setting.GetRateLimitBanSetting()
	previousSetting := *setting
	*setting = operation_setting.RateLimitBanSetting{Enabled: true, MaxRPM: 1}
	t.Cleanup(func() { *setting = previousSetting })

	handler := ModelRequestRateLimit()
	request := func(status int) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("id", 73)
			c.Set("role", common.RoleCommonUser)
		})
		router.Use(handler)
		router.POST("/v1/chat/completions", func(c *gin.Context) { c.Status(status) })
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
		return recorder
	}

	assert.Equal(t, http.StatusOK, request(http.StatusOK).Code)
	response := request(http.StatusOK)
	require.Equal(t, http.StatusTooManyRequests, response.Code)
	assert.Equal(t, "1", response.Header().Get("X-RateLimit-Limit-Requests"))
	assert.Equal(t, "0", response.Header().Get("X-RateLimit-Remaining-Requests"))
	assert.NotEmpty(t, response.Header().Get("Retry-After"))

	var body struct {
		Error struct {
			Type string `json:"type"`
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, "rate_limit_error", body.Error.Type)
	assert.Equal(t, "rate_limit_exceeded", body.Error.Code)

	// The 429 path enqueues the review audit task asynchronously; wait for it
	// to land before the fixture DB is torn down so the goroutine does not
	// race the cleanup.
	require.Eventually(t, func() bool {
		var count int64
		if err := db.Model(&model.LLMReviewTask{}).Where("user_id = ?", 73).Count(&count).Error; err != nil {
			return false
		}
		return count > 0
	}, 5*time.Second, 50*time.Millisecond)
}

func TestRPMMiddlewareReleasesFailedRequestSlot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRedisEnabled, previousRDB := common.RedisEnabled, common.RDB
	server := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		require.NoError(t, common.RDB.Close())
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
	})

	setting := operation_setting.GetRateLimitBanSetting()
	previousSetting := *setting
	*setting = operation_setting.RateLimitBanSetting{Enabled: true, MaxRPM: 1}
	t.Cleanup(func() { *setting = previousSetting })

	handler := ModelRequestRateLimit()
	run := func(downstreamStatus int) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("id", 74)
			c.Set("role", common.RoleCommonUser)
			c.Set("original_model", "test-model")
		})
		router.Use(handler)
		router.POST("/v1/chat/completions", func(c *gin.Context) { c.Status(downstreamStatus) })
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
		return recorder
	}

	assert.Equal(t, http.StatusBadGateway, run(http.StatusBadGateway).Code)
	assert.Equal(t, http.StatusOK, run(http.StatusOK).Code)
}

func TestRPMMiddlewareReleasesExplicitStreamFailureSlot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRedisEnabled, previousRDB := common.RedisEnabled, common.RDB
	server := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		require.NoError(t, common.RDB.Close())
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
	})

	setting := operation_setting.GetRateLimitBanSetting()
	previousSetting := *setting
	*setting = operation_setting.RateLimitBanSetting{Enabled: true, MaxRPM: 1}
	t.Cleanup(func() { *setting = previousSetting })

	run := func(release bool) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("id", 76)
			c.Set("role", common.RoleCommonUser)
			c.Set("original_model", "stream-model")
		})
		router.Use(ModelRequestRateLimit())
		router.POST("/v1/chat/completions", func(c *gin.Context) {
			c.Status(http.StatusOK)
			if release {
				MarkRPMFailure(c)
			}
		})
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
		return recorder
	}

	assert.Equal(t, http.StatusOK, run(true).Code)
	assert.Equal(t, http.StatusOK, run(false).Code)
}

func TestRPMMiddlewareTriggersReviewWithoutBlocking429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRedisEnabled, previousRDB := common.RedisEnabled, common.RDB
	server := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		require.NoError(t, common.RDB.Close())
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRDB
	})

	setting := operation_setting.GetRateLimitBanSetting()
	previousSetting := *setting
	*setting = operation_setting.RateLimitBanSetting{Enabled: true, MaxRPM: 1}
	t.Cleanup(func() { *setting = previousSetting })

	previousEnqueue := service.EnqueueRateLimitReview
	triggered := make(chan service.RateLimitReviewTrigger, 1)
	service.EnqueueRateLimitReview = func(_ context.Context, trigger service.RateLimitReviewTrigger) error {
		triggered <- trigger
		return nil
	}
	t.Cleanup(func() { service.EnqueueRateLimitReview = previousEnqueue })

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 75)
		c.Set("role", common.RoleCommonUser)
		c.Set("original_model", "review-model")
	})
	router.Use(ModelRequestRateLimit())
	router.POST("/v1/chat/completions", func(c *gin.Context) { c.Status(http.StatusOK) })
	requestBody := `{"model":"review-model","stream":true,"messages":[{"role":"user","content":"hello"}]}`
	request := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		return req
	}
	router.ServeHTTP(httptest.NewRecorder(), request())

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request())
	require.Equal(t, http.StatusTooManyRequests, response.Code)

	select {
	case trigger := <-triggered:
		assert.Equal(t, 75, trigger.UserID)
		assert.Equal(t, "review-model", trigger.ModelName)
		assert.Equal(t, "/v1/chat/completions", trigger.Endpoint)
		assert.Equal(t, 2, trigger.CurrentValue)
		assert.Equal(t, 1, trigger.LimitValue)
		assert.True(t, trigger.IsStream)
		assert.Contains(t, trigger.RequestSnippet, "hello")
		assert.Contains(t, trigger.RequestBody, `"stream":true`)
	case <-time.After(time.Second):
		t.Fatal("rate-limit review was not triggered")
	}
}

func TestMemoryRPMReservationReportsWindowBoundary(t *testing.T) {
	limiter := common.InMemoryRateLimiter{}
	limiter.Init(time.Minute)

	allowed, reservation, _, _, _ := limiter.ReserveWithWindow("boundary-user", 1, 60)
	require.True(t, allowed)
	t.Cleanup(reservation.Release)

	allowed, _, _, windowStart, windowEnd := limiter.ReserveWithWindow("boundary-user", 1, 60)
	require.False(t, allowed)
	assert.Greater(t, windowStart, int64(0))
	assert.Greater(t, windowEnd, windowStart)
	assert.GreaterOrEqual(t, windowEnd-windowStart, int64(60))
	assert.LessOrEqual(t, windowEnd-windowStart, int64(61))
}

func TestRedisRPMReservationReportsWindowBoundary(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })
	previousRDB := common.RDB
	common.RDB = rdb
	t.Cleanup(func() { common.RDB = previousRDB })

	first, _ := gin.CreateTestContext(httptest.NewRecorder())
	first.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	allowed, _, _, _, _, err := reserveRedisRPMSlot(first, 901, 60, 1)
	require.NoError(t, err)
	require.True(t, allowed)

	second, _ := gin.CreateTestContext(httptest.NewRecorder())
	second.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	allowed, _, _, windowStart, windowEnd, err := reserveRedisRPMSlot(second, 901, 60, 1)
	require.NoError(t, err)
	require.False(t, allowed)
	assert.Greater(t, windowStart, int64(0))
	assert.Greater(t, windowEnd, windowStart)
	assert.GreaterOrEqual(t, windowEnd-windowStart, int64(60))
	assert.LessOrEqual(t, windowEnd-windowStart, int64(61))
}

func TestMemoryRPMWindowReleasesFailedRequests(t *testing.T) {
	limiter := common.InMemoryRateLimiter{}
	limiter.Init(time.Minute)

	allowed, reservation, _ := limiter.Reserve("user", 1, 60)
	require.True(t, allowed)
	reservation.Release()

	allowed, _, _ = limiter.Reserve("user", 1, 60)
	assert.True(t, allowed)
}

func TestInMemoryRPMReservationReleaseIsIdempotent(t *testing.T) {
	limiter := common.InMemoryRateLimiter{}
	limiter.Init(time.Minute)
	allowed, reservation, _ := limiter.Reserve("user", 1, 60)
	require.True(t, allowed)

	done := make(chan struct{}, 2)
	for range 2 {
		go func() {
			reservation.Release()
			done <- struct{}{}
		}()
	}
	<-done
	<-done

	allowed, _, _ = limiter.Reserve("user", 1, 60)
	assert.True(t, allowed)
}

func TestRPMMiddlewareExemptsRootRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setting := operation_setting.GetRateLimitBanSetting()
	previousSetting := *setting
	*setting = operation_setting.RateLimitBanSetting{Enabled: true, MaxRPM: 1}
	t.Cleanup(func() { *setting = previousSetting })

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	context.Set("id", 1)
	context.Set("role", common.RoleRootUser)
	ModelRequestRateLimit()(context)

	assert.False(t, context.IsAborted())
}
