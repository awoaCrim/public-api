package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// RequestSnapshot status values. The DB row is the metadata/access record only;
// the captured request body itself lives in encrypted files under the snapshot
// storage directory and is never persisted here.
const (
	RequestSnapshotStatusStored  = "stored"  // file exists and is readable
	RequestSnapshotStatusFailed  = "failed"  // capture did not complete
	RequestSnapshotStatusDeleted = "deleted" // intentionally removed (tombstone)
	RequestSnapshotStatusMissing = "missing" // row exists but file disappeared
)

// Access action values for RequestSnapshotAccess.
const (
	RequestSnapshotActionRead   = "read"
	RequestSnapshotActionDelete = "delete"
	RequestSnapshotActionClean  = "clean"
)

// Safe result codes returned to operators (never contain request content).
const (
	SnapshotResultOk          = "ok"
	SnapshotResultNotFound    = "not_found"
	SnapshotResultDeleted     = "deleted"
	SnapshotResultMissing     = "missing"
	SnapshotResultUnavailable = "unavailable" // capture failed or disabled
	SnapshotResultCorrupt     = "corrupt"
	SnapshotResultDenied      = "denied"
	SnapshotResultError       = "error"
	SnapshotResultWrongNode   = "wrong_node"
)

var ErrRequestSnapshotAlreadyExists = errors.New("request snapshot already exists")

// RequestSnapshot is the main-DB metadata record for one captured request.
// Only metadata lives here; the encrypted body bytes are stored as files.
type RequestSnapshot struct {
	Id            int64  `json:"id" gorm:"primaryKey"`
	RequestId     string `json:"request_id" gorm:"type:varchar(64);uniqueIndex;default:''"`
	UserId        int    `json:"user_id" gorm:"index;default:0"`
	TokenId       int    `json:"token_id" gorm:"default:0"`
	ModelName     string `json:"model_name" gorm:"type:varchar(128);default:''"`
	RelayFormat   string `json:"relay_format" gorm:"type:varchar(32);default:''"`
	Method        string `json:"method" gorm:"type:varchar(16);default:''"`
	Path          string `json:"path" gorm:"type:varchar(512);default:''"`
	ContentType   string `json:"content_type" gorm:"type:varchar(128);default:''"`
	PlainSize     int64  `json:"plain_size" gorm:"default:0"`
	EncryptedSize int64  `json:"encrypted_size" gorm:"default:0"`
	KeyVersion    int    `json:"key_version" gorm:"default:0"`
	Node          string `json:"node" gorm:"type:varchar(128);index;default:''"`
	RelativePath  string `json:"relative_path" gorm:"type:varchar(256);default:''"`
	Status        string `json:"status" gorm:"type:varchar(16);index;default:'stored'"`
	ErrorCode     string `json:"error_code" gorm:"type:varchar(32);default:''"`
	CreatedAt     int64  `json:"created_at" gorm:"index;default:0"`
	UpdatedAt     int64  `json:"updated_at" gorm:"default:0"`
}

// RequestSnapshotAccess records one operator action against a snapshot. It is
// deliberately content-free: operator, action, outcome, IP, node, and time.
type RequestSnapshotAccess struct {
	Id         int64  `json:"id" gorm:"primaryKey"`
	RequestId  string `json:"request_id" gorm:"type:varchar(64);index;default:''"`
	SnapshotId int64  `json:"snapshot_id" gorm:"index;default:0"`
	OperatorId int    `json:"operator_id" gorm:"index;default:0"`
	Operator   string `json:"operator" gorm:"type:varchar(64);default:''"`
	Action     string `json:"action" gorm:"type:varchar(16);default:''"`
	Success    bool   `json:"success"`
	Result     string `json:"result" gorm:"type:varchar(32);default:''"`
	Ip         string `json:"ip" gorm:"type:varchar(64);default:''"`
	Node       string `json:"node" gorm:"type:varchar(128);default:''"`
	CreatedAt  int64  `json:"created_at" gorm:"index;default:0"`
}

// GetRequestSnapshotByRequestId returns the snapshot row for a request id.
func GetRequestSnapshotByRequestId(requestId string) (*RequestSnapshot, error) {
	var row RequestSnapshot
	err := DB.Where("request_id = ?", requestId).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// CreateRequestSnapshot inserts a snapshot row. A request id captured twice is
// rejected so a single request can never be recorded more than once. The
// request_id unique index is the backstop; the pre-check keeps the outcome
// deterministic across all supported databases.
func CreateRequestSnapshot(row *RequestSnapshot) error {
	var count int64
	if err := DB.Model(&RequestSnapshot{}).Where("request_id = ?", row.RequestId).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrRequestSnapshotAlreadyExists
	}
	now := time.Now().Unix()
	row.CreatedAt = now
	row.UpdatedAt = now
	return DB.Create(row).Error
}

// UpdateRequestSnapshotStatus transitions a snapshot row's lifecycle state.
func UpdateRequestSnapshotStatus(requestId, status, errorCode string) error {
	now := time.Now().Unix()
	return DB.Model(&RequestSnapshot{}).
		Where("request_id = ?", requestId).
		Updates(map[string]interface{}{
			"status":     status,
			"error_code": errorCode,
			"updated_at": now,
		}).Error
}

// CheckRequestSnapshotAccessStorage verifies that the main-DB audit table is
// reachable before snapshot bytes are loaded. The subsequent insert remains
// authoritative; this check avoids reading content during a known audit
// outage.
func CheckRequestSnapshotAccessStorage() error {
	var count int64
	return DB.Model(&RequestSnapshotAccess{}).Limit(1).Count(&count).Error
}

// CreateRequestSnapshotAccess appends an access audit row.
func CreateRequestSnapshotAccess(row *RequestSnapshotAccess) error {
	row.CreatedAt = time.Now().Unix()
	return DB.Create(row).Error
}

// ListOwnNodeStoredSnapshots returns stored snapshots of one node ordered by
// creation time ascending (oldest first), used for capacity cleanup.
func ListOwnNodeStoredSnapshots(node string, limit int) ([]*RequestSnapshot, error) {
	var rows []*RequestSnapshot
	err := DB.Where("node = ? AND status = ?", node, RequestSnapshotStatusStored).
		Order("created_at asc, id asc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// CountOwnNodeStoredSize sums the encrypted file sizes of stored snapshots of
// one node.
func CountOwnNodeStoredSize(node string) (int64, error) {
	var total int64
	err := DB.Model(&RequestSnapshot{}).
		Where("node = ? AND status = ?", node, RequestSnapshotStatusStored).
		Select("COALESCE(SUM(encrypted_size), 0)").
		Scan(&total).Error
	return total, err
}

// ListOwnNodeSnapshotsByNode returns all rows belonging to one node
// (regardless of status) so the cleanup pass can reconcile files against rows.
func ListOwnNodeSnapshotsByNode(node string) ([]*RequestSnapshot, error) {
	var rows []*RequestSnapshot
	err := DB.Where("node = ?", node).Find(&rows).Error
	return rows, err
}

// ListExpiredOwnNodeStoredSnapshots returns stored snapshots of one node older
// than the given unix timestamp (retention cutoff).
func ListExpiredOwnNodeStoredSnapshots(node string, olderThan int64, limit int) ([]*RequestSnapshot, error) {
	var rows []*RequestSnapshot
	err := DB.Where("node = ? AND status = ? AND created_at < ?", node, RequestSnapshotStatusStored, olderThan).
		Order("created_at asc, id asc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// DeleteRequestSnapshotRow removes the metadata row entirely (used when a
// snapshot plus its file are fully removed within retention bounds by manual
// delete).
func DeleteRequestSnapshotRow(requestId string) error {
	return DB.Where("request_id = ?", requestId).Delete(&RequestSnapshot{}).Error
}

// ListRequestSnapshotTombstonesOlderThan returns tombstone rows (status
// deleted) older than the cutoff so the cleanup can bound their retention.
func ListRequestSnapshotTombstonesOlderThan(olderThan int64, limit int) ([]*RequestSnapshot, error) {
	var rows []*RequestSnapshot
	err := DB.Where("status = ? AND updated_at < ?", RequestSnapshotStatusDeleted, olderThan).
		Order("updated_at asc, id asc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// DeleteOwnNodeRequestSnapshotTerminalRowsOlderThan removes failed and
// missing metadata rows for one node after retention. These rows never have a
// readable file, so retaining them forever would let persistent capture
// failures grow the main database without bound.
func DeleteOwnNodeRequestSnapshotTerminalRowsOlderThan(node string, olderThan int64) (int64, error) {
	res := DB.Where(
		"node = ? AND status IN ? AND updated_at < ?",
		node,
		[]string{RequestSnapshotStatusFailed, RequestSnapshotStatusMissing},
		olderThan,
	).Delete(&RequestSnapshot{})
	return res.RowsAffected, res.Error
}

// DeleteRequestSnapshotTombstonesOlderThan removes tombstone rows older than
// the cutoff so metadata rows cannot grow forever.
func DeleteRequestSnapshotTombstonesOlderThan(olderThan int64) (int64, error) {
	res := DB.Where("status = ? AND updated_at < ?", RequestSnapshotStatusDeleted, olderThan).
		Delete(&RequestSnapshot{})
	return res.RowsAffected, res.Error
}

// DeleteRequestSnapshotAccessOlderThan removes access audit rows older than
// the cutoff so audit rows cannot grow forever.
func DeleteRequestSnapshotAccessOlderThan(olderThan int64) (int64, error) {
	res := DB.Where("created_at < ?", olderThan).Delete(&RequestSnapshotAccess{})
	return res.RowsAffected, res.Error
}
