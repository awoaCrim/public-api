package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterFromBlacklistedIPCreatesDisabledUserAndReturnsForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupManageUserTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Token{}, &model.IPBlacklist{}))

	previousRegisterEnabled := common.RegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	previousEmailVerificationEnabled := common.EmailVerificationEnabled
	previousGenerateDefaultToken := constant.GenerateDefaultToken
	previousBlacklistEnabled := operation_setting.IsIPBlacklistEnabled()
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = true
	operation_setting.SetIPBlacklistEnabled(true)
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
		common.EmailVerificationEnabled = previousEmailVerificationEnabled
		constant.GenerateDefaultToken = previousGenerateDefaultToken
		operation_setting.SetIPBlacklistEnabled(previousBlacklistEnabled)
	})
	require.NoError(t, model.AddIPBlacklist("198.51.100.20", "test", 1))

	request := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(`{"username":"blocked-register","password":"password123"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "198.51.100.20:12345"
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request

	Register(context)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	var response struct {
		Code string `json:"code"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "IP_BLACKLISTED", response.Code)
	var user model.User
	require.NoError(t, model.DB.Where("username = ?", "blocked-register").First(&user).Error)
	assert.Equal(t, common.UserStatusDisabled, user.Status)
	var tokenCount int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("user_id = ?", user.Id).Count(&tokenCount).Error)
	assert.Zero(t, tokenCount)
}
