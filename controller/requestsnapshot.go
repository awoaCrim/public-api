package controller

import (
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/requestsnapshot"

	"github.com/gin-gonic/gin"
)

func createSnapshotAccess(c *gin.Context, requestID string, snapshotID int64, success bool, result string) error {
	return model.CreateRequestSnapshotAccess(&model.RequestSnapshotAccess{
		RequestId:  requestID,
		SnapshotId: snapshotID,
		OperatorId: c.GetInt("id"),
		Operator:   c.GetString("username"),
		Action:     model.RequestSnapshotActionRead,
		Success:    success,
		Result:     result,
		Ip:         c.ClientIP(),
		Node:       common.NodeName,
	})
}

// GetRequestSnapshot serves the captured body of a request. The route is
// protected by RootAuth and audits every read attempt synchronously. No
// secondary verification or delegatable permission is required.
func GetRequestSnapshot(c *gin.Context) {
	requestID := c.Param("request_id")
	snapshotID := int64(0)

	row, rowErr := model.GetRequestSnapshotByRequestId(requestID)
	if rowErr == nil && row != nil {
		snapshotID = row.Id
	}

	recordAccess := func(success bool, result string) bool {
		if err := createSnapshotAccess(c, requestID, snapshotID, success, result); err != nil {
			common.SysError("failed to record request snapshot access: " + err.Error())
			return false
		}
		return true
	}

	// Root authorization is established by the route before this point. Check
	// the audit table before loading the body so a known audit outage never causes
	// snapshot bytes to be read into memory.
	if err := model.CheckRequestSnapshotAccessStorage(); err != nil {
		common.SysError("request snapshot access audit unavailable: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    "SNAPSHOT_AUDIT_FAILED",
			"message": "Snapshot access could not be audited",
		})
		return
	}

	content, err := requestsnapshot.Default().Read(c.Request.Context(), requestID)
	if err != nil {
		switch {
		case errors.Is(err, requestsnapshot.ErrSnapshotNotFound):
			recordAccess(false, model.SnapshotResultNotFound)
			c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "SNAPSHOT_NOT_FOUND", "message": "Snapshot not found"})
		case errors.Is(err, requestsnapshot.ErrSnapshotDeleted):
			recordAccess(false, model.SnapshotResultDeleted)
			c.JSON(http.StatusGone, gin.H{"success": false, "code": "SNAPSHOT_DELETED", "message": "Snapshot deleted"})
		case errors.Is(err, requestsnapshot.ErrSnapshotMissing):
			recordAccess(false, model.SnapshotResultMissing)
			c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "SNAPSHOT_MISSING", "message": "Snapshot file missing"})
		case errors.Is(err, requestsnapshot.ErrSnapshotUnavailable):
			recordAccess(false, model.SnapshotResultUnavailable)
			c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "SNAPSHOT_UNAVAILABLE", "message": "Snapshot unavailable"})
		case errors.Is(err, requestsnapshot.ErrSnapshotCorrupt):
			recordAccess(false, model.SnapshotResultCorrupt)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "SNAPSHOT_CORRUPT", "message": "Snapshot corrupt"})
		default:
			if ownerNode, ok := requestsnapshot.IsWrongNodeError(err); ok {
				recordAccess(false, model.SnapshotResultWrongNode)
				c.JSON(http.StatusConflict, gin.H{"success": false, "code": "SNAPSHOT_WRONG_NODE", "message": "Snapshot stored on another node", "owner_node": ownerNode})
				return
			}
			recordAccess(false, model.SnapshotResultError)
			common.SysError("request snapshot read failed: " + err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"code":    "SNAPSHOT_READ_FAILED",
				"message": "Snapshot could not be read",
			})
		}
		return
	}

	// Audit the successful read before responding. A successful content access
	// is not allowed to proceed without a durable audit row.
	if !recordAccess(true, model.SnapshotResultOk) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    "SNAPSHOT_AUDIT_FAILED",
			"message": "Snapshot access could not be audited",
		})
		return
	}

	contentType := ""
	if row != nil {
		contentType = row.ContentType
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"request_id":     requestID,
			"content_type":   contentType,
			"size":           len(content),
			"content_base64": base64.StdEncoding.EncodeToString(content),
		},
	})
}
