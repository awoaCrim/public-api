package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupGitHubOAuthRegistrationAgeDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousRegisterEnabled := common.RegisterEnabled
	previousMinimumAgeYears := common.GitHubOAuthMinimumAgeYears
	previousQuotaForNewUser := common.QuotaForNewUser
	previousQuotaForInviter := common.QuotaForInviter
	previousQuotaForInvitee := common.QuotaForInvitee

	dsn := fmt.Sprintf(
		"file:github_oauth_registration_age_%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthFlow{}, &model.Option{}))

	model.DB = db
	model.LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.RegisterEnabled = true
	common.QuotaForNewUser = 0
	common.QuotaForInviter = 0
	common.QuotaForInvitee = 0

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedisEnabled
		common.RegisterEnabled = previousRegisterEnabled
		common.GitHubOAuthMinimumAgeYears = previousMinimumAgeYears
		common.QuotaForNewUser = previousQuotaForNewUser
		common.QuotaForInviter = previousQuotaForInviter
		common.QuotaForInvitee = previousQuotaForInvitee
		_ = sqlDB.Close()
	})

	return db
}

type githubOAuthBindTestProvider struct{}

func (p *githubOAuthBindTestProvider) GetName() string {
	return "GitHub"
}

func (p *githubOAuthBindTestProvider) IsEnabled() bool {
	return true
}

func (p *githubOAuthBindTestProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{AccessToken: "test-token"}, nil
}

func (p *githubOAuthBindTestProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return &oauth.OAuthUser{ProviderUserID: "github-bound", CreatedAt: nil}, nil
}

func (p *githubOAuthBindTestProvider) IsUserIDTaken(string) bool {
	return false
}

func (p *githubOAuthBindTestProvider) FillUserByProviderID(*model.User, string) error {
	return nil
}

func (p *githubOAuthBindTestProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GitHubId = providerUserID
}

func (p *githubOAuthBindTestProvider) GetProviderPrefix() string {
	return "github_"
}

func (p *githubOAuthBindTestProvider) ProviderUserIDColumn() string {
	return "github_id"
}

func TestHandleOAuthBindAllowsMissingGitHubAgeMetadata(t *testing.T) {
	db := setupGitHubOAuthRegistrationAgeDB(t)
	previousSessionSecret := common.SessionSecret
	common.SessionSecret = "github-oauth-bind-test-secret"
	t.Cleanup(func() {
		common.SessionSecret = previousSessionSecret
	})
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)

	user := &model.User{
		Id:       1002,
		Username: "github-bind-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	flowToken, flow, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeOAuth,
		Provider:  "github",
		Intent:    model.AuthFlowIntentBind,
		UserId:    user.Id,
		SessionId: "github-bind-session",
		ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/oauth/github/callback?code=test-code", nil)
	handleOAuthBind(c, &githubOAuthBindTestProvider{}, flow, flowToken)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, "github-bound", updated.GitHubId)
}

func TestFindOrCreateOAuthUserRejectsTooNewGitHubAccountWithoutCreatingUser(t *testing.T) {
	db := setupGitHubOAuthRegistrationAgeDB(t)
	common.GitHubOAuthMinimumAgeYears = 1

	var before int64
	require.NoError(t, db.Model(&model.User{}).Count(&before).Error)

	createdAt := time.Now().Add(-time.Hour)
	_, err := findOrCreateOAuthUser(nil, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: "github-too-new",
		Username:       "too-new",
		CreatedAt:      &createdAt,
	}, "")

	var tooNew *OAuthGitHubAccountTooNewError
	require.ErrorAs(t, err, &tooNew)
	assert.Equal(t, 1, tooNew.MinimumAgeYears)

	var after int64
	require.NoError(t, db.Model(&model.User{}).Count(&after).Error)
	assert.Equal(t, before, after)
}

func TestFindOrCreateOAuthUserFailsClosedWhenGitHubAccountAgeIsUnavailable(t *testing.T) {
	db := setupGitHubOAuthRegistrationAgeDB(t)
	common.GitHubOAuthMinimumAgeYears = 1

	_, err := findOrCreateOAuthUser(nil, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: "github-missing-created-at",
		Username:       "missing-created-at",
	}, "")

	var unavailable *OAuthGitHubAccountAgeUnavailableError
	require.ErrorAs(t, err, &unavailable)

	var count int64
	require.NoError(t, db.Model(&model.User{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestFindOrCreateOAuthUserAllowsOldGitHubAccount(t *testing.T) {
	db := setupGitHubOAuthRegistrationAgeDB(t)
	common.GitHubOAuthMinimumAgeYears = 1

	createdAt := time.Now().AddDate(-1, 0, 0).Add(-time.Minute)
	user, err := findOrCreateOAuthUser(nil, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: "github-old-enough",
		Username:       "old-enough",
		DisplayName:    "Old Enough",
		CreatedAt:      &createdAt,
	}, "")

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "github-old-enough", user.GitHubId)

	var count int64
	require.NoError(t, db.Model(&model.User{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestFindOrCreateOAuthUserExistingGitHubUserBypassesAgeRestriction(t *testing.T) {
	db := setupGitHubOAuthRegistrationAgeDB(t)
	common.GitHubOAuthMinimumAgeYears = 1

	existing := &model.User{
		Id:       1001,
		Username: "existing-github-user",
		GitHubId: "github-existing",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(existing).Error)

	createdAt := time.Now().Add(-time.Hour)
	user, err := findOrCreateOAuthUser(nil, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: "github-existing",
		CreatedAt:      &createdAt,
	}, "")

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, existing.Id, user.Id)

	var count int64
	require.NoError(t, db.Model(&model.User{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestFindOrCreateOAuthUserDisabledAgeRestrictionAllowsMissingMetadata(t *testing.T) {
	db := setupGitHubOAuthRegistrationAgeDB(t)
	common.GitHubOAuthMinimumAgeYears = 0

	user, err := findOrCreateOAuthUser(nil, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: "github-age-check-disabled",
		Username:       "age-check-disabled",
	}, "")

	require.NoError(t, err)
	require.NotNil(t, user)

	var count int64
	require.NoError(t, db.Model(&model.User{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCheckGitHubRegistrationAgeUsesCalendarYears(t *testing.T) {
	now := time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC)
	exactCutoff := now.AddDate(-1, 0, 0)
	justTooNew := exactCutoff.Add(time.Nanosecond)

	require.NoError(t, checkGitHubRegistrationAge(now, &exactCutoff, 1))
	var tooNew *OAuthGitHubAccountTooNewError
	err := checkGitHubRegistrationAge(now, &justTooNew, 1)
	require.ErrorAs(t, err, &tooNew)
	assert.Equal(t, 1, tooNew.MinimumAgeYears)

	leapYearNow := time.Date(2025, time.February, 28, 12, 0, 0, 0, time.UTC)
	leapDayAccount := time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC)
	assert.Error(t, checkGitHubRegistrationAge(leapYearNow, &leapDayAccount, 1))
}

func TestCheckGitHubRegistrationAgeDisabledAndUnavailable(t *testing.T) {
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	newAccount := now.Add(-time.Hour)

	require.NoError(t, checkGitHubRegistrationAge(now, &newAccount, 0))

	err := checkGitHubRegistrationAge(now, nil, 1)
	var unavailable *OAuthGitHubAccountAgeUnavailableError
	require.ErrorAs(t, err, &unavailable)
	assert.EqualError(t, unavailable, "unable to verify GitHub account age")

	err = checkGitHubRegistrationAge(now, &newAccount, -1)
	assert.True(t, errors.As(err, &unavailable))
}

func TestFindOrCreateOAuthUserMigratesLegacyGitHubLoginWithoutAgeMetadata(t *testing.T) {
	db := setupGitHubOAuthRegistrationAgeDB(t)
	common.GitHubOAuthMinimumAgeYears = 1

	existing := &model.User{
		Id:       1003,
		Username: "legacy-github-user",
		GitHubId: "legacy-login",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(existing).Error)

	user, err := findOrCreateOAuthUser(nil, &oauth.GitHubProvider{}, &oauth.OAuthUser{
		ProviderUserID: "123456",
		Username:       "legacy-login",
		Extra:          map[string]any{"legacy_id": "legacy-login"},
	}, "")

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, existing.Id, user.Id)

	var stored model.User
	require.NoError(t, db.First(&stored, existing.Id).Error)
	assert.Equal(t, "123456", stored.GitHubId)

	var count int64
	require.NoError(t, db.Model(&model.User{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestHandleOAuthUserResolutionErrorReturnsLocalizedGitHubAgeEnvelopes(t *testing.T) {
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		lang     string
		err      error
		expected string
	}{
		{
			name:     "English too-new error",
			lang:     "en",
			err:      &OAuthGitHubAccountTooNewError{MinimumAgeYears: 2},
			expected: "Your GitHub account must be at least 2 calendar years old to register",
		},
		{
			name:     "Simplified Chinese unavailable error",
			lang:     "zh-CN",
			err:      &OAuthGitHubAccountAgeUnavailableError{},
			expected: "无法验证 GitHub 账号注册时间，请稍后重试",
		},
		{
			name:     "Traditional Chinese too-new error",
			lang:     "zh-TW",
			err:      &OAuthGitHubAccountTooNewError{MinimumAgeYears: 1},
			expected: "您的 GitHub 帳號註冊時間必須滿 1 個日曆年才能註冊",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/github", nil)
			c.Request.Header.Set("Accept-Language", test.lang)

			handleOAuthUserResolutionError(c, test.err)

			assert.Equal(t, http.StatusOK, recorder.Code)
			var payload struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
			assert.False(t, payload.Success)
			assert.Equal(t, test.expected, payload.Message)
			assert.NotContains(t, payload.Message, "created_at")
		})
	}
}

func TestGetOptionsExposesGitHubOAuthMinimumAgeYears(t *testing.T) {
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{
		common.GitHubOAuthMinimumAgeYearsOptionKey: "4",
	}
	t.Cleanup(func() { common.OptionMap = previousMap })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	GetOptions(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool            `json:"success"`
		Data    []*model.Option `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	for _, option := range payload.Data {
		if option.Key == common.GitHubOAuthMinimumAgeYearsOptionKey {
			assert.Equal(t, "4", option.Value)
			return
		}
	}
	t.Fatalf("%s was not exposed by GetOptions", common.GitHubOAuthMinimumAgeYearsOptionKey)
}

func TestUpdateOptionRejectsInvalidGitHubOAuthMinimumAgeWithoutMutation(t *testing.T) {
	db := setupGitHubOAuthRegistrationAgeDB(t)
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{
		common.GitHubOAuthMinimumAgeYearsOptionKey: "3",
	}
	common.GitHubOAuthMinimumAgeYears = 3
	t.Cleanup(func() { common.OptionMap = previousMap })
	require.NoError(t, db.Create(&model.Option{
		Key:   common.GitHubOAuthMinimumAgeYearsOptionKey,
		Value: "3",
	}).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"GitHubOAuthMinimumAgeYears","value":1.5}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateOption(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.NotEmpty(t, payload.Message)
	assert.Equal(t, 3, common.GitHubOAuthMinimumAgeYears)
	assert.Equal(t, "3", common.OptionMap[common.GitHubOAuthMinimumAgeYearsOptionKey])

	var stored model.Option
	require.NoError(t, db.First(&stored, "key = ?", common.GitHubOAuthMinimumAgeYearsOptionKey).Error)
	assert.Equal(t, "3", stored.Value)
}
