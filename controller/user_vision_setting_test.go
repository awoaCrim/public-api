package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	visionservice "github.com/QuantumNous/new-api/service/vision"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserVisionSettingControllerTest(t *testing.T) *model.User {
	t.Helper()
	gin.SetMode(gin.TestMode)

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:user_vision_setting_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	model.LOG_DB = db

	settings := dto.UserSetting{
		NotifyType:                       dto.NotifyTypeWebhook,
		QuotaWarningThreshold:            123.5,
		WebhookUrl:                       "https://notify.example/webhook",
		WebhookSecret:                    "existing-secret",
		NotificationEmail:                "notify@example.com",
		BarkUrl:                          "https://bark.example/push",
		GotifyUrl:                        "https://gotify.example",
		GotifyToken:                      "existing-token",
		GotifyPriority:                   7,
		UpstreamModelUpdateNotifyEnabled: true,
		AcceptUnsetRatioModel:            true,
		RecordIpLog:                      true,
		SidebarModules:                   "dashboard,logs",
		BillingPreference:                "subscription",
		Language:                         "ja",
		Vision: &dto.UserVisionSetting{
			Enabled:        true,
			VisionModel:    "old-vision-model",
			VisionSuffix:   "-old-vision",
			PromptTemplate: "old prompt",
			PhashThreshold: 8,
		},
	}
	settingBytes, err := common.Marshal(settings)
	require.NoError(t, err)
	user := &model.User{
		Id:       980001,
		Username: "vision-setting-user",
		AffCode:  "vision-setting-aff",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Setting:  string(settingBytes),
	}
	require.NoError(t, db.Create(user).Error)

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return user
}

func updateUserVisionSettingForTest(t *testing.T, userID int, vision *dto.UserVisionSetting) (bool, string) {
	t.Helper()
	requestBody, err := common.Marshal(UpdateUserVisionSettingRequest{Vision: vision})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/user/setting/vision", bytes.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userID)

	UpdateUserVisionSetting(ctx)

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response.Success, response.Message
}

func TestUpdateUserVisionSettingRejectsOutOfRangeThresholdWithoutMutation(t *testing.T) {
	for _, threshold := range []int{-1, 65} {
		t.Run(fmt.Sprintf("threshold_%d", threshold), func(t *testing.T) {
			user := setupUserVisionSettingControllerTest(t)
			originalSetting := user.Setting

			success, _ := updateUserVisionSettingForTest(t, user.Id, &dto.UserVisionSetting{
				Enabled:        false,
				VisionModel:    "new-vision-model",
				VisionSuffix:   "-vision",
				PromptTemplate: "new prompt",
				PhashThreshold: threshold,
			})
			assert.False(t, success)

			var unchanged model.User
			require.NoError(t, model.DB.First(&unchanged, user.Id).Error)
			assert.Equal(t, originalSetting, unchanged.Setting)
		})
	}
}

func TestUpdateUserSettingRejectsOutOfRangeVisionThresholdWithoutMutation(t *testing.T) {
	user := setupUserVisionSettingControllerTest(t)
	originalSetting := user.Setting
	requestBody, err := common.Marshal(UpdateUserSettingRequest{
		QuotaWarningType:      dto.NotifyTypeWebhook,
		QuotaWarningThreshold: 100,
		WebhookUrl:            "https://notify.example/new-webhook",
		Vision: &dto.UserVisionSetting{
			Enabled:        true,
			VisionModel:    "new-vision-model",
			VisionSuffix:   "-vision",
			PromptTemplate: "new prompt",
			PhashThreshold: 65,
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/user/setting", bytes.NewReader(requestBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", user.Id)

	UpdateUserSetting(ctx)

	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)

	var unchanged model.User
	require.NoError(t, model.DB.First(&unchanged, user.Id).Error)
	assert.Equal(t, originalSetting, unchanged.Setting)
}

func TestUpdateUserVisionSettingAcceptsThresholdBounds(t *testing.T) {
	for _, threshold := range []int{0, 64} {
		t.Run(fmt.Sprintf("threshold_%d", threshold), func(t *testing.T) {
			user := setupUserVisionSettingControllerTest(t)

			success, message := updateUserVisionSettingForTest(t, user.Id, &dto.UserVisionSetting{
				Enabled:        true,
				VisionModel:    "vision-model",
				VisionSuffix:   "-vision",
				PromptTemplate: "custom prompt",
				PhashThreshold: threshold,
			})
			require.True(t, success, message)

			var updated model.User
			require.NoError(t, model.DB.First(&updated, user.Id).Error)
			got := updated.GetSetting()
			require.NotNil(t, got.Vision)
			assert.Equal(t, threshold, got.Vision.PhashThreshold)
		})
	}
}

func TestUpdateUserVisionSettingUsesDefaultPromptWhenBlank(t *testing.T) {
	user := setupUserVisionSettingControllerTest(t)
	success, message := updateUserVisionSettingForTest(t, user.Id, &dto.UserVisionSetting{
		Enabled:        true,
		VisionModel:    "vision-model",
		VisionSuffix:   "-vision",
		PromptTemplate: " \n\t",
	})
	require.True(t, success, message)

	var updated model.User
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	got := updated.GetSetting()
	require.NotNil(t, got.Vision)
	assert.Equal(t, visionservice.DefaultPromptTemplate, got.Vision.PromptTemplate)
}

func TestUpdateUserVisionSettingPreservesUnrelatedSettingsAndExplicitZeroValues(t *testing.T) {
	user := setupUserVisionSettingControllerTest(t)
	success, message := updateUserVisionSettingForTest(t, user.Id, &dto.UserVisionSetting{
		Enabled:        false,
		VisionModel:    "new-vision-model",
		VisionSuffix:   "-vision",
		PromptTemplate: "new prompt",
		PhashThreshold: 0,
	})
	require.True(t, success, message)

	var updated model.User
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	got := updated.GetSetting()
	require.NotNil(t, got.Vision)
	assert.False(t, got.Vision.Enabled)
	assert.Equal(t, "new-vision-model", got.Vision.VisionModel)
	assert.Equal(t, "-vision", got.Vision.VisionSuffix)
	assert.Equal(t, "new prompt", got.Vision.PromptTemplate)
	assert.Zero(t, got.Vision.PhashThreshold)

	assert.Equal(t, dto.NotifyTypeWebhook, got.NotifyType)
	assert.Equal(t, 123.5, got.QuotaWarningThreshold)
	assert.Equal(t, "https://notify.example/webhook", got.WebhookUrl)
	assert.Equal(t, "existing-secret", got.WebhookSecret)
	assert.Equal(t, "notify@example.com", got.NotificationEmail)
	assert.Equal(t, "https://bark.example/push", got.BarkUrl)
	assert.Equal(t, "https://gotify.example", got.GotifyUrl)
	assert.Equal(t, "existing-token", got.GotifyToken)
	assert.Equal(t, 7, got.GotifyPriority)
	assert.True(t, got.UpstreamModelUpdateNotifyEnabled)
	assert.True(t, got.AcceptUnsetRatioModel)
	assert.True(t, got.RecordIpLog)
	assert.Equal(t, "dashboard,logs", got.SidebarModules)
	assert.Equal(t, "subscription", got.BillingPreference)
	assert.Equal(t, "ja", got.Language)
}
