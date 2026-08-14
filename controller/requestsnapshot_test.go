package controller

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/requestsnapshot"
	"github.com/QuantumNous/new-api/setting/requestsnapshot_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newSnapshotEndpointTest wires the process-level snapshot service (used by
// GetRequestSnapshot) against an in-memory SQLite main DB, a temp storage dir,
// and a stable session identity with a valid request_snapshot.read proof.
func newSnapshotEndpointTest(t *testing.T) (identity service.AuthIdentity, proof string) {
	t.Helper()

	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.RequestSnapshot{}, &model.RequestSnapshotAccess{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		_ = sqlDB.Close()
	})

	previousNode := common.NodeName
	common.NodeName = "endpoint-node"
	t.Cleanup(func() { common.NodeName = previousNode })

	previousSecret := common.SessionSecret
	common.SessionSecret = "endpoint-snapshot-session-secret"
	t.Cleanup(func() { common.SessionSecret = previousSecret })

	previousCrypto := common.CryptoSecret
	t.Setenv("CRYPTO_SECRET", "endpoint-snapshot-crypto-secret")
	common.CryptoSecret = "endpoint-snapshot-crypto-secret"
	t.Cleanup(func() { common.CryptoSecret = previousCrypto })

	setting := requestsnapshot_setting.GetSetting()
	previousSetting := *setting
	*setting = requestsnapshot_setting.RequestSnapshotSetting{
		Enabled:              true,
		StoragePath:          t.TempDir(),
		MaxBodyMb:            2,
		MaxTotalMb:           8,
		RetentionDays:        30,
		CleanupIntervalHours: 24,
		OrphanGraceMinutes:   60,
	}
	requestsnapshot_setting.Normalize()
	t.Cleanup(func() {
		*setting = previousSetting
	})

	identity = service.AuthIdentity{
		UserID: 42, SessionID: "endpoint-snapshot-session", UserAuthVersion: 1, SessionVersion: 1,
	}
	proof, _, err = service.IssueSecurityProof(identity, "2fa", []string{snapshotProofScope})
	require.NoError(t, err)
	return identity, proof
}

func snapshotRequest(t *testing.T, identity service.AuthIdentity, requestID, proof string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/log/"+requestID+"/snapshot", nil)
	if proof != "" {
		req.Header.Set("X-Security-Proof", proof)
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "request_id", Value: requestID}}
	ctx.Set("id", identity.UserID)
	ctx.Set("username", "endpoint-admin")
	ctx.Set("session_id", identity.SessionID)
	ctx.Set("auth_version", identity.UserAuthVersion)
	ctx.Set("session_version", identity.SessionVersion)
	GetRequestSnapshot(ctx)
	return rec
}

func countAccessRows(t *testing.T, requestID string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.RequestSnapshotAccess{}).
		Where("request_id = ?", requestID).Count(&count).Error)
	return count
}

func TestGetRequestSnapshotProofGate(t *testing.T) {
	identity, validProof := newSnapshotEndpointTest(t)

	// A proof with the wrong scope must be rejected.
	wrongScopeProof, _, err := service.IssueSecurityProof(identity, "2fa", []string{"channel.key.read"})
	require.NoError(t, err)

	tests := []struct {
		name         string
		proof        string
		expectedCode string
	}{
		{name: "missing-proof", expectedCode: "SECURITY_PROOF_REQUIRED"},
		{name: "wrong-scope-proof", proof: wrongScopeProof, expectedCode: "SECURITY_PROOF_SCOPE_MISMATCH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := snapshotRequest(t, identity, "req-proof-"+tt.name, tt.proof)
			assert.Equal(t, http.StatusForbidden, rec.Code)
			var body struct {
				Code string `json:"code"`
			}
			require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tt.expectedCode, body.Code)

			// Failed post-permission attempts are audited synchronously.
			assert.Equal(t, int64(1), countAccessRows(t, "req-proof-"+tt.name))
			var access model.RequestSnapshotAccess
			require.NoError(t, model.DB.Where("request_id = ?", "req-proof-"+tt.name).First(&access).Error)
			assert.False(t, access.Success)
			assert.Equal(t, model.RequestSnapshotActionRead, access.Action)
			assert.Equal(t, tt.expectedCode, access.Result)
			assert.Equal(t, common.NodeName, access.Node)
			assert.Equal(t, "endpoint-admin", access.Operator)
		})
	}

	// A valid proof reaches the read path (request id does not exist).
	rec := snapshotRequest(t, identity, "req-proof-valid-missing", validProof)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	var body struct {
		Code string `json:"code"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "SNAPSHOT_NOT_FOUND", body.Code)
}

func TestGetRequestSnapshotSuccessRoundTripAndAudit(t *testing.T) {
	identity, proof := newSnapshotEndpointTest(t)

	payload := []byte("{\"messages\":[{\"role\":\"user\",\"content\":\"confidential\"}],\"stream\":true}")
	require.NoError(t, requestsnapshot.Default().Capture(t.Context(), requestsnapshot.CaptureMeta{
		RequestID: "req-success", UserID: 42, TokenID: 9, ModelName: "gpt-4o",
		RelayFormat: "openai", Method: "POST", Path: "/v1/chat/completions", ContentType: "application/json",
	}, payload))

	rec := snapshotRequest(t, identity, "req-success", proof)
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			RequestID     string `json:"request_id"`
			ContentType   string `json:"content_type"`
			Size          int    `json:"size"`
			ContentBase64 string `json:"content_base64"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, "req-success", body.Data.RequestID)
	assert.Equal(t, "application/json", body.Data.ContentType)
	assert.Equal(t, len(payload), body.Data.Size)
	decoded, err := base64.StdEncoding.DecodeString(body.Data.ContentBase64)
	require.NoError(t, err)
	assert.Equal(t, payload, decoded, "exact request bytes must round-trip through the endpoint")

	// Successful access is audited before the response.
	accessRows := []model.RequestSnapshotAccess{}
	require.NoError(t, model.DB.Where("request_id = ?", "req-success").Find(&accessRows).Error)
	require.Len(t, accessRows, 1)
	assert.True(t, accessRows[0].Success)
	assert.Equal(t, model.SnapshotResultOk, accessRows[0].Result)
}

func TestCreateSnapshotAccessPersistsSuccessAndFailsWhenAuditStoreIsUnavailable(t *testing.T) {
	identity, _ := newSnapshotEndpointTest(t)
	req := httptest.NewRequest(http.MethodGet, "/api/log/req-audit/snapshot", nil)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Set("id", identity.UserID)
	ctx.Set("username", "endpoint-admin")

	require.NoError(t, createSnapshotAccess(ctx, "req-audit", 17, true, model.SnapshotResultOk))
	assert.Equal(t, int64(1), countAccessRows(t, "req-audit"))

	require.NoError(t, model.DB.Migrator().DropTable(&model.RequestSnapshotAccess{}))
	assert.Error(t, model.CheckRequestSnapshotAccessStorage())
	err := createSnapshotAccess(ctx, "req-audit-failed", 18, true, model.SnapshotResultOk)
	assert.Error(t, err, "successful content access must fail closed when its audit row cannot be stored")
}

func TestGetRequestSnapshotSafeCodes(t *testing.T) {
	identity, proof := newSnapshotEndpointTest(t)

	t.Run("deleted", func(t *testing.T) {
		require.NoError(t, requestsnapshot.Default().Capture(t.Context(), requestsnapshot.CaptureMeta{
			RequestID: "req-deleted", UserID: 42, ModelName: "m", RelayFormat: "openai",
		}, []byte("gone")))
		require.NoError(t, requestsnapshot.Default().Delete(t.Context(), "req-deleted"))
		rec := snapshotRequest(t, identity, "req-deleted", proof)
		assert.Equal(t, http.StatusGone, rec.Code)
		assert.Contains(t, rec.Body.String(), "SNAPSHOT_DELETED")
		assert.Equal(t, int64(1), countAccessRows(t, "req-deleted"))
	})

	t.Run("wrong node", func(t *testing.T) {
		otherRow := &model.RequestSnapshot{
			RequestId: "req-remote", Node: "remote-node", Status: model.RequestSnapshotStatusStored,
			RelativePath: "remote.snap",
		}
		require.NoError(t, model.CreateRequestSnapshot(otherRow))
		rec := snapshotRequest(t, identity, "req-remote", proof)
		assert.Equal(t, http.StatusConflict, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, "SNAPSHOT_WRONG_NODE")
		assert.Contains(t, body, `"owner_node":"remote-node"`)
		var access model.RequestSnapshotAccess
		require.NoError(t, model.DB.Where("request_id = ?", "req-remote").First(&access).Error)
		assert.Equal(t, model.SnapshotResultWrongNode, access.Result)
	})

	t.Run("corrupt", func(t *testing.T) {
		require.NoError(t, requestsnapshot.Default().Capture(t.Context(), requestsnapshot.CaptureMeta{
			RequestID: "req-corrupt", UserID: 42, ModelName: "m", RelayFormat: "openai",
		}, []byte("integrity")))
		row, err := model.GetRequestSnapshotByRequestId("req-corrupt")
		require.NoError(t, err)
		nodeDir := filepath.Join(requestsnapshot_setting.GetSetting().StoragePath, requestsnapshot.NodeDirName(common.NodeName))
		require.NoError(t, os.WriteFile(filepath.Join(nodeDir, row.RelativePath), []byte("tampered"), 0o600))

		rec := snapshotRequest(t, identity, "req-corrupt", proof)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), "SNAPSHOT_CORRUPT")
		var access model.RequestSnapshotAccess
		require.NoError(t, model.DB.Where("request_id = ?", "req-corrupt").First(&access).Error)
		assert.Equal(t, model.SnapshotResultCorrupt, access.Result)
	})
}
