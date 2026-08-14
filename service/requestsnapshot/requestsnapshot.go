// Package requestsnapshot captures and serves complete request bodies for
// authenticated, well-formed relay requests. Captured bytes are encrypted at
// rest (AES-256-GCM, HKDF-derived keys from CRYPTO_SECRET/SESSION_SECRET) in a
// node-local directory; the main database only holds metadata and access audit
// rows. The feature is off by default and only becomes operational when a
// stable key source was explicitly configured in the environment.
package requestsnapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/requestsnapshot_setting"
	"gorm.io/gorm"
)

// CaptureMeta describes one request at capture time. It is metadata only and
// never contains request content.
type CaptureMeta struct {
	RequestID   string
	UserID      int
	TokenID     int
	ModelName   string
	RelayFormat string
	Method      string
	Path        string
	ContentType string
}

// Manager is the request snapshot service surface used by controllers.
type Manager interface {
	// Capture persists the request body for a single request. It is a no-op
	// (nil error) when the feature is disabled, and fails closed with a safe
	// configuration state when no stable key exists. It never stores partial
	// bodies and never captures the same request id twice.
	Capture(ctx context.Context, meta CaptureMeta, body []byte) error
	// Read returns the exact captured bytes for a request id.
	Read(ctx context.Context, requestID string) ([]byte, error)
	// Delete removes an own-node snapshot and marks its row as a tombstone.
	Delete(ctx context.Context, requestID string) error
	// Cleanup runs one node-local maintenance pass (retention, capacity,
	// orphans, missing detection, bounded audit history).
	Cleanup(ctx context.Context) (CleanupResult, error)
}

// Sentinel errors map to stable safe codes at the API boundary.
var (
	ErrSnapshotNotFound    = errors.New("request snapshot not found")
	ErrSnapshotDeleted     = errors.New("request snapshot deleted")
	ErrSnapshotMissing     = errors.New("request snapshot file missing")
	ErrSnapshotUnavailable = errors.New("request snapshot unavailable")
	ErrSnapshotCorrupt     = errors.New("request snapshot corrupt")
)

// WrongNodeError reports that the snapshot lives on another node.
type WrongNodeError struct {
	OwnerNode string
}

func (e *WrongNodeError) Error() string {
	return fmt.Sprintf("request snapshot belongs to node %q", e.OwnerNode)
}

// IsWrongNodeError reports whether err carries the owner node of a remote
// snapshot.
func IsWrongNodeError(err error) (string, bool) {
	var wrong *WrongNodeError
	if errors.As(err, &wrong) {
		return wrong.OwnerNode, true
	}
	return "", false
}

// CleanupResult summarizes one maintenance pass.
type CleanupResult struct {
	RetentionDeleted   int   // own-node stored snapshots expired by retention
	CapacityDeleted    int   // own-node stored snapshots expired by capacity
	OrphansRemoved     int   // ownerless files older than the orphan grace
	MarkedMissing      int   // own-node stored rows whose file vanished
	AccessPruned       int   // access audit rows older than retention
	TerminalRowsPruned int   // failed/missing metadata rows older than retention
	TombstonesPruned   int   // tombstone rows older than retention
	StoredBytes        int64 // own-node stored bytes after the pass
}

// StableKeyConfigured reports whether CRYPTO_SECRET or SESSION_SECRET was
// explicitly set in the environment. Without an explicit stable secret the
// derived keys change on every restart, so the feature is not operational.
func StableKeyConfigured() bool {
	return os.Getenv("CRYPTO_SECRET") != "" || os.Getenv("SESSION_SECRET") != ""
}

const cleanupBatchSize = 200

type manager struct {
	// mu serializes captures and capacity decisions on this node so concurrent
	// local captures can never push the node over its capacity bound.
	mu sync.Mutex
}

// NewManager returns an isolated manager (used by tests and the node singleton).
func NewManager() Manager {
	return &manager{}
}

var defaultManager Manager = NewManager()

// Default returns the process-wide manager.
func Default() Manager {
	return defaultManager
}

func currentNode() string {
	return common.NodeName
}

// storageDir resolves the per-node storage directory for the live setting.
func storageDir() string {
	setting := requestsnapshot_setting.GetSetting()
	return filepath.Join(setting.StoragePath, nodeDirName(currentNode()))
}

// stableKeyOperational reports whether the process both started with an
// explicit stable source and has usable key material loaded.
func stableKeyOperational() bool {
	if !StableKeyConfigured() {
		return false
	}
	return common.CryptoSecret != ""
}

// enabledState returns whether capture may proceed. When the feature is
// disabled capture is skipped entirely; when it is enabled but the key source
// is unstable the feature fails closed.
func enabledState() (operational bool, disabled bool, reason string) {
	setting := requestsnapshot_setting.GetSetting()
	if !setting.Enabled {
		return false, true, ""
	}
	if !stableKeyOperational() {
		return false, false, "config_unstable_key"
	}
	return true, false, ""
}

// Capture implements Manager.
func (m *manager) Capture(_ context.Context, meta CaptureMeta, body []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	operational, disabled, reason := enabledState()
	if disabled {
		return nil
	}

	// Exactly once: a request id already recorded on any node is never
	// captured twice.
	existing, err := model.GetRequestSnapshotByRequestId(meta.RequestID)
	if err == nil && existing != nil {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if !operational {
		return m.recordFailed(meta, reason)
	}

	setting := requestsnapshot_setting.GetSetting()
	maxBodyBytes := int64(setting.MaxBodyMb) << 20
	if int64(len(body)) > maxBodyBytes {
		return m.recordFailed(meta, "oversize")
	}

	rel, err := safeRelativePath(meta.RequestID)
	if err != nil {
		return m.recordFailed(meta, "invalid_request_id")
	}
	envelope, err := encryptSnapshot(body, meta.RequestID, rel)
	if err != nil {
		return m.recordFailed(meta, "encrypt_failed")
	}

	dir := storageDir()
	if err := ensureSnapshotDir(dir); err != nil {
		return m.recordFailed(meta, "io_error")
	}

	// Capacity is per node and enforced under the node lock: the sum of
	// encrypted sizes already stored plus this capture must stay under the cap.
	total, err := model.CountOwnNodeStoredSize(currentNode())
	if err != nil {
		return m.recordFailed(meta, "db_error")
	}
	if total+int64(len(envelope)) > int64(setting.MaxTotalMb)<<20 {
		return m.recordFailed(meta, "capacity")
	}

	if err := atomicWriteFile(dir, rel, envelope); err != nil {
		return m.recordFailed(meta, "io_error")
	}

	row := &model.RequestSnapshot{
		RequestId:     meta.RequestID,
		UserId:        meta.UserID,
		TokenId:       meta.TokenID,
		ModelName:     meta.ModelName,
		RelayFormat:   meta.RelayFormat,
		Method:        meta.Method,
		Path:          meta.Path,
		ContentType:   meta.ContentType,
		PlainSize:     int64(len(body)),
		EncryptedSize: int64(len(envelope)),
		KeyVersion:    currentKeyVersion,
		Node:          currentNode(),
		RelativePath:  rel,
		Status:        model.RequestSnapshotStatusStored,
		ErrorCode:     "",
	}
	if err := model.CreateRequestSnapshot(row); err != nil {
		if errors.Is(err, model.ErrRequestSnapshotAlreadyExists) {
			// Lost a race against another node for the same request id: the
			// file we just wrote is an orphan and will be reconciled later.
			_ = os.Remove(filepath.Join(dir, rel))
			return nil
		}
		_ = os.Remove(filepath.Join(dir, rel))
		return m.recordFailed(meta, "db_error")
	}
	return nil
}

// recordFailed writes a failed metadata row so the operator can see the
// capture outcome. It never stores request content.
func (m *manager) recordFailed(meta CaptureMeta, reason string) error {
	row := &model.RequestSnapshot{
		RequestId:   meta.RequestID,
		UserId:      meta.UserID,
		TokenId:     meta.TokenID,
		ModelName:   meta.ModelName,
		RelayFormat: meta.RelayFormat,
		Method:      meta.Method,
		Path:        meta.Path,
		ContentType: meta.ContentType,
		Node:        currentNode(),
		Status:      model.RequestSnapshotStatusFailed,
		ErrorCode:   reason,
	}
	if err := model.CreateRequestSnapshot(row); err != nil {
		if errors.Is(err, model.ErrRequestSnapshotAlreadyExists) {
			return nil
		}
		return err
	}
	return nil
}

// Read implements Manager.
func (m *manager) Read(_ context.Context, requestID string) ([]byte, error) {
	row, err := model.GetRequestSnapshotByRequestId(requestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSnapshotNotFound
		}
		return nil, err
	}
	switch row.Status {
	case model.RequestSnapshotStatusDeleted:
		return nil, ErrSnapshotDeleted
	case model.RequestSnapshotStatusMissing:
		return nil, ErrSnapshotMissing
	case model.RequestSnapshotStatusFailed:
		return nil, ErrSnapshotUnavailable
	}
	if row.Node != currentNode() {
		return nil, &WrongNodeError{OwnerNode: row.Node}
	}
	if !stableKeyOperational() {
		return nil, ErrSnapshotUnavailable
	}
	if !validateRelativePath(row.RelativePath) {
		return nil, ErrSnapshotCorrupt
	}
	envelope, err := os.ReadFile(filepath.Join(storageDir(), row.RelativePath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSnapshotMissing
		}
		return nil, err
	}
	plain, err := decryptSnapshot(envelope, requestID, row.RelativePath)
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// Delete implements Manager: an own-node stored snapshot becomes a tombstone.
func (m *manager) Delete(_ context.Context, requestID string) error {
	row, err := model.GetRequestSnapshotByRequestId(requestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSnapshotNotFound
		}
		return err
	}
	if row.Node != currentNode() {
		return &WrongNodeError{OwnerNode: row.Node}
	}
	if row.Status != model.RequestSnapshotStatusStored {
		return nil
	}
	if validateRelativePath(row.RelativePath) {
		_ = os.Remove(filepath.Join(storageDir(), row.RelativePath))
	}
	return model.UpdateRequestSnapshotStatus(requestID, model.RequestSnapshotStatusDeleted, "")
}

// Cleanup implements Manager. It is one node-local pass and only ever touches
// files and rows belonging to the current node.
func (m *manager) Cleanup(ctx context.Context) (CleanupResult, error) {
	var result CleanupResult
	node := currentNode()
	setting := requestsnapshot_setting.GetSetting()
	now := time.Now().Unix()

	dir := storageDir()
	_ = ensureSnapshotDir(dir)

	// 1. Retention: oldest own-node stored snapshots past their retention are
	// removed and their rows become tombstones.
	retentionCutoff := now - int64(setting.RetentionDays)*24*60*60
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		rows, err := model.ListExpiredOwnNodeStoredSnapshots(node, retentionCutoff, cleanupBatchSize)
		if err != nil {
			return result, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if validateRelativePath(row.RelativePath) {
				_ = os.Remove(filepath.Join(dir, row.RelativePath))
			}
			if err := model.UpdateRequestSnapshotStatus(row.RequestId, model.RequestSnapshotStatusDeleted, ""); err != nil {
				return result, err
			}
			result.RetentionDeleted++
		}
		if len(rows) < cleanupBatchSize {
			break
		}
	}

	// 2. Capacity: drop the oldest own-node stored snapshots until the node is
	// back under its cap. Rows are consumed one at a time (oldest first) so the
	// pass never over-deletes beyond the bound.
	maxTotalBytes := int64(setting.MaxTotalMb) << 20
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		total, err := model.CountOwnNodeStoredSize(node)
		if err != nil {
			return result, err
		}
		if total <= maxTotalBytes {
			break
		}
		rows, err := model.ListOwnNodeStoredSnapshots(node, 1)
		if err != nil {
			return result, err
		}
		if len(rows) == 0 {
			break
		}
		row := rows[0]
		if validateRelativePath(row.RelativePath) {
			_ = os.Remove(filepath.Join(dir, row.RelativePath))
		}
		if err := model.UpdateRequestSnapshotStatus(row.RequestId, model.RequestSnapshotStatusDeleted, ""); err != nil {
			return result, err
		}
		result.CapacityDeleted++
	}

	// 3. Orphan reconciliation: files older than the grace period without a
	// matching own-node row are removed; own-node stored rows whose file is
	// gone become missing. Other-node files are never touched.
	rows, err := model.ListOwnNodeSnapshotsByNode(node)
	if err != nil {
		return result, err
	}
	rowPaths := make(map[string]bool, len(rows))
	for _, row := range rows {
		rowPaths[row.RelativePath] = true
	}
	orphanCutoff := now - int64(setting.OrphanGraceMinutes)*60
	files, err := listSnapshotFiles(dir)
	if err != nil {
		return result, err
	}
	for _, name := range files {
		if rowPaths[name] {
			continue
		}
		info, statErr := os.Stat(filepath.Join(dir, name))
		if statErr != nil {
			continue
		}
		if info.ModTime().Unix() >= orphanCutoff {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err == nil {
			result.OrphansRemoved++
		}
	}
	for _, row := range rows {
		if row.Status != model.RequestSnapshotStatusStored {
			continue
		}
		if !validateRelativePath(row.RelativePath) {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(dir, row.RelativePath)); os.IsNotExist(statErr) {
			if err := model.UpdateRequestSnapshotStatus(row.RequestId, model.RequestSnapshotStatusMissing, "file_missing"); err != nil {
				return result, err
			}
			result.MarkedMissing++
		}
	}

	// 4. Bounded history: prune audit rows plus terminal metadata past
	// retention so the main database cannot grow forever when captures fail,
	// files disappear, or tombstones age out. Audit/tombstone retention is
	// global because these tables share the main database; terminal file-state
	// rows remain node-scoped.
	pruned, err := model.DeleteRequestSnapshotAccessOlderThan(retentionCutoff)
	if err != nil {
		return result, err
	}
	result.AccessPruned = int(pruned)
	terminalRows, err := model.DeleteOwnNodeRequestSnapshotTerminalRowsOlderThan(node, retentionCutoff)
	if err != nil {
		return result, err
	}
	result.TerminalRowsPruned = int(terminalRows)
	tombstones, err := model.DeleteRequestSnapshotTombstonesOlderThan(retentionCutoff)
	if err != nil {
		return result, err
	}
	result.TombstonesPruned = int(tombstones)

	// 5. Report own-node stored bytes after the pass (test seam and future
	// status display).
	total, err := model.CountOwnNodeStoredSize(node)
	if err == nil {
		result.StoredBytes = total
	}
	return result, nil
}

// StartCleanupLoop runs the node-local maintenance pass on every node (not a
// master-only lease: each node owns its own files and must clean them). The
// loop exits when ctx is cancelled. Tests exercise the single-pass Cleanup
// directly.
func StartCleanupLoop(ctx context.Context) {
	go func() {
		// First pass shortly after boot so stale files from earlier runs are
		// reconciled even if the interval is long.
		timer := time.NewTimer(1 * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if _, err := defaultManager.Cleanup(ctx); err != nil && ctx.Err() == nil {
					common.SysError("request snapshot cleanup failed: " + err.Error())
				}
			}
			interval := time.Duration(requestsnapshot_setting.GetSetting().CleanupIntervalHours) * time.Hour
			if interval <= 0 {
				interval = time.Duration(requestsnapshot_setting.DefaultCleanupIntervalHours) * time.Hour
			}
			timer.Reset(interval)
		}
	}()
}
