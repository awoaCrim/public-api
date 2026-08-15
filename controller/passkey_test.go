package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type passkeyTestBody struct {
	*strings.Reader
}

func (*passkeyTestBody) Close() error { return nil }

func TestRequestSnapshotSecurityProofScopeIsNotAllowed(t *testing.T) {
	assert.False(t, isAllowedSecurityProofScope("request_snapshot.read"))
	assert.False(t, isAllowedSecurityProofScope("request_snapshot.write"))
}

func TestParsePasskeyFinishRequestDoesNotRewriteRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bodyText := `{"flow_token":"flow-1","credential":{"id":"credential-1"}}`
	body := &passkeyTestBody{Reader: strings.NewReader(bodyText)}
	request := httptest.NewRequest(http.MethodPost, "/api/user/passkey/register/finish", nil)
	request.Body = body
	request.ContentLength = int64(len(bodyText))
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	parsed, err := parsePasskeyFinishRequest(context)
	require.NoError(t, err)
	assert.Equal(t, "flow-1", parsed.FlowToken)
	assert.JSONEq(t, `{"id":"credential-1"}`, string(parsed.Credential))
	assert.Same(t, body, context.Request.Body)
	assert.Equal(t, int64(len(bodyText)), context.Request.ContentLength)
}

func TestPasskeyRegisterFinishWithoutProofReachesWebAuthnValidation(t *testing.T) {
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	settings := system_setting.GetPasskeySettings()
	previousSettings := *settings
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TwoFA{}, &model.AuthFlow{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	*settings = system_setting.PasskeySettings{Enabled: true}
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
		*settings = previousSettings
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	user := &model.User{
		Username: "passkey-proof-user", Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1,
	}
	require.NoError(t, db.Create(user).Error)
	// A configured 2FA account must not reintroduce the removed registration gate.
	require.NoError(t, db.Create(&model.TwoFA{UserId: user.Id, Secret: "totp-secret", IsEnabled: true}).Error)
	identity := struct {
		userID    int
		sessionID string
	}{userID: user.Id, sessionID: "passkey-register-session"}

	for _, test := range []struct {
		name  string
		proof string
	}{
		{name: "missing proof"},
		{name: "arbitrary proof header", proof: "not-a-security-proof"},
	} {
		t.Run(test.name, func(t *testing.T) {
			flowToken, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
				Purpose: model.AuthFlowPurposePasskeyRegister, UserId: identity.userID, SessionId: identity.sessionID,
				Payload: `{}`, ExpiresAt: time.Now().Add(time.Minute),
			})
			require.NoError(t, err)
			body := fmt.Sprintf(`{"flow_token":%q,"credential":{}}`, flowToken)
			request := httptest.NewRequest(http.MethodPost, "/api/user/passkey/register/finish", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			if test.proof != "" {
				request.Header.Set("X-Security-Proof", test.proof)
			}
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = request
			context.Set("id", identity.userID)
			context.Set("session_id", identity.sessionID)
			context.Set("auth_version", int64(1))
			context.Set("session_version", int64(1))

			PasskeyRegisterFinish(context)

			assert.Equal(t, http.StatusOK, response.Code)
			assert.NotContains(t, response.Body.String(), "SECURITY_PROOF")
			var responseBody struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &responseBody))
			assert.False(t, responseBody.Success, "the intentionally invalid WebAuthn payload should fail validation")
			flow, err := model.GetAuthFlow(flowToken, model.AuthFlowMatch{
				Purpose: model.AuthFlowPurposePasskeyRegister, UserId: identity.userID, SessionId: identity.sessionID,
			})
			require.NoError(t, err)
			assert.Nil(t, flow.ConsumedAt, "invalid WebAuthn input must not consume the flow")
		})
	}
}
