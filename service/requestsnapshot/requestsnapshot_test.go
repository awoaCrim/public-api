package requestsnapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/requestsnapshot_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newTestManager wires an isolated manager against an in-memory SQLite main DB
// and a temp storage dir, with the feature enabled under a stable secret.
func newTestManager(t *testing.T, mutate func(*requestsnapshot_setting.RequestSnapshotSetting)) Manager {
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
	common.NodeName = "test-node"
	t.Cleanup(func() { common.NodeName = previousNode })

	setTestCryptoSecret(t, "requestsnapshot-manager-test-secret")

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
	if mutate != nil {
		mutate(setting)
	}
	requestsnapshot_setting.Normalize()
	t.Cleanup(func() {
		*setting = previousSetting
	})

	return NewManager()
}

func captureMeta(requestID string) CaptureMeta {
	return CaptureMeta{
		RequestID:   requestID,
		UserID:      7,
		TokenID:     11,
		ModelName:   "gpt-4o",
		RelayFormat: "openai",
		Method:      "POST",
		Path:        "/v1/chat/completions",
		ContentType: "application/json",
	}
}

func mustCountSnapshots(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.RequestSnapshot{}).Count(&count).Error)
	return count
}

func TestCaptureDisabledIsNoop(t *testing.T) {
	mgr := newTestManager(t, func(s *requestsnapshot_setting.RequestSnapshotSetting) {
		s.Enabled = false
	})

	require.NoError(t, mgr.Capture(context.Background(), captureMeta("req-disabled"), []byte("never stored")))

	assert.Equal(t, int64(0), mustCountSnapshots(t))
	storageRoot := requestsnapshot_setting.GetSetting().StoragePath
	entries, err := os.ReadDir(storageRoot)
	require.NoError(t, err)
	assert.Empty(t, entries, "disabled feature must not create storage directories")
}

func TestCaptureUnstableKeyFailsClosed(t *testing.T) {
	// No CRYPTO_SECRET / SESSION_SECRET in the environment for this test.
	previous := common.CryptoSecret
	common.CryptoSecret = ""
	common.SessionSecret = ""
	t.Cleanup(func() { common.CryptoSecret = previous })

	mgr := newTestManager(t, nil)
	// No CRYPTO_SECRET / SESSION_SECRET in the environment: the feature is
	// enabled but must fail closed because the key source is unstable.
	t.Setenv("CRYPTO_SECRET", "")
	t.Setenv("SESSION_SECRET", "")
	require.True(t, requestsnapshot_setting.GetSetting().Enabled)

	require.NoError(t, mgr.Capture(context.Background(), captureMeta("req-unstable"), []byte("must not be stored as plaintext")))

	row, err := model.GetRequestSnapshotByRequestId("req-unstable")
	require.NoError(t, err)
	assert.Equal(t, model.RequestSnapshotStatusFailed, row.Status)
	assert.Equal(t, "config_unstable_key", row.ErrorCode)

	// No file may exist for a failed capture.
	dir := filepath.Join(requestsnapshot_setting.GetSetting().StoragePath, nodeDirName(common.NodeName))
	files, err := listSnapshotFiles(dir)
	require.NoError(t, err)
	assert.Empty(t, files)

	// Read of the failed row fails closed.
	_, err = mgr.Read(context.Background(), "req-unstable")
	assert.ErrorIs(t, err, ErrSnapshotUnavailable)
}

func TestCaptureReadExactPersistence(t *testing.T) {
	mgr := newTestManager(t, nil)

	payload := []byte("{\"messages\":[{\"role\":\"user\",\"content\":\"你好，世界\"}],\"stream\":false,\"n\":2}\x00\x01\x02")
	require.NoError(t, mgr.Capture(context.Background(), captureMeta("req-exact"), payload))

	got, err := mgr.Read(context.Background(), "req-exact")
	require.NoError(t, err)
	assert.Equal(t, payload, got, "exact captured bytes must round-trip")

	row, err := model.GetRequestSnapshotByRequestId("req-exact")
	require.NoError(t, err)
	assert.Equal(t, model.RequestSnapshotStatusStored, row.Status)
	assert.Equal(t, common.NodeName, row.Node)
	assert.Equal(t, "openai", row.RelayFormat)
	assert.Equal(t, "gpt-4o", row.ModelName)
	assert.Equal(t, int64(len(payload)), row.PlainSize)
	assert.Greater(t, row.EncryptedSize, int64(len(payload)))
	assert.Equal(t, currentKeyVersion, row.KeyVersion)
	assert.Equal(t, 7, row.UserId)
	assert.Equal(t, 11, row.TokenId)
	assert.True(t, validateRelativePath(row.RelativePath))

	// File exists with owner-only permissions and lives under the node dir.
	full := filepath.Join(filepath.Join(requestsnapshot_setting.GetSetting().StoragePath, nodeDirName(common.NodeName)), row.RelativePath)
	info, err := os.Stat(full)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	// Encrypted bytes on disk must not contain the plaintext.
	raw, err := os.ReadFile(full)
	require.NoError(t, err)
	assert.NotContains(t, raw, "你好，世界")
}

func TestCaptureOversizeNoPartialSave(t *testing.T) {
	mgr := newTestManager(t, func(s *requestsnapshot_setting.RequestSnapshotSetting) {
		s.MaxBodyMb = 1
	})

	oversize := strings.Repeat("x", (1<<20)+1)
	require.NoError(t, mgr.Capture(context.Background(), captureMeta("req-oversize"), []byte(oversize)))

	row, err := model.GetRequestSnapshotByRequestId("req-oversize")
	require.NoError(t, err)
	assert.Equal(t, model.RequestSnapshotStatusFailed, row.Status)
	assert.Equal(t, "oversize", row.ErrorCode)
	assert.Equal(t, int64(0), row.PlainSize, "no partial save: plain size must stay zero")

	dir := filepath.Join(requestsnapshot_setting.GetSetting().StoragePath, nodeDirName(common.NodeName))
	files, err := listSnapshotFiles(dir)
	require.NoError(t, err)
	assert.Empty(t, files, "no file may be written for an oversized body")
}

func TestCaptureDuplicateRequestIdOnce(t *testing.T) {
	mgr := newTestManager(t, nil)
	require.NoError(t, mgr.Capture(context.Background(), captureMeta("req-dup"), []byte("first")))
	require.NoError(t, mgr.Capture(context.Background(), captureMeta("req-dup"), []byte("second")))

	assert.Equal(t, int64(1), mustCountSnapshots(t))
	got, err := mgr.Read(context.Background(), "req-dup")
	require.NoError(t, err)
	assert.Equal(t, []byte("first"), got, "first capture wins")
}

func TestCaptureConcurrentCapacityNeverExceedsCap(t *testing.T) {
	mgr := newTestManager(t, func(s *requestsnapshot_setting.RequestSnapshotSetting) {
		s.MaxBodyMb = 1
		s.MaxTotalMb = 2 // tiny cap: only a subset of captures may fit
	})

	const goroutines = 12
	payload := make([]byte, 512*1024)
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			meta := captureMeta(fmt.Sprintf("req-conc-%d", i))
			errs[i] = mgr.Capture(context.Background(), meta, payload)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		require.NoError(t, err, "capture %d", i)
	}

	// Per-node capacity is enforced under the node lock: the stored encrypted
	// size can never exceed the cap.
	total, err := model.CountOwnNodeStoredSize(common.NodeName)
	require.NoError(t, err)
	assert.LessOrEqual(t, total, int64(2<<20), "stored size must not exceed the per-node cap")

	var storedCount int64
	require.NoError(t, model.DB.Model(&model.RequestSnapshot{}).
		Where("node = ? AND status = ?", common.NodeName, model.RequestSnapshotStatusStored).
		Count(&storedCount).Error)
	assert.Greater(t, storedCount, int64(0), "some captures must have succeeded")
	var failedCount int64
	require.NoError(t, model.DB.Model(&model.RequestSnapshot{}).
		Where("node = ? AND status = ?", common.NodeName, model.RequestSnapshotStatusFailed).
		Count(&failedCount).Error)
	assert.Greater(t, failedCount, int64(0), "some captures must have been rejected by capacity")
}

func TestCleanupRetentionThenCapacityThenOwnership(t *testing.T) {
	mgr := newTestManager(t, func(s *requestsnapshot_setting.RequestSnapshotSetting) {
		s.MaxBodyMb = 1
		s.MaxTotalMb = 8
		s.RetentionDays = 30
		s.OrphanGraceMinutes = 1
	})

	ctx := context.Background()
	now := time.Now().Unix()
	payload := make([]byte, 1<<20) // 1 MiB bodies; max_body_mb=1 allows them

	// Three stored snapshots captured under the generous 8 MiB cap.
	require.NoError(t, mgr.Capture(ctx, captureMeta("req-cap-a"), payload))
	require.NoError(t, mgr.Capture(ctx, captureMeta("req-cap-b"), payload))
	require.NoError(t, mgr.Capture(ctx, captureMeta("req-cap-c"), payload))

	// Fresh stored snapshot — must survive retention and be evicted last.
	require.NoError(t, mgr.Capture(ctx, captureMeta("req-fresh"), payload))

	// Old stored snapshot (past retention) — must be deleted by retention.
	require.NoError(t, mgr.Capture(ctx, captureMeta("req-retention-old"), payload))
	require.NoError(t, model.DB.Model(&model.RequestSnapshot{}).
		Where("request_id = ?", "req-retention-old").
		Update("created_at", now-31*24*60*60).Error)

	// Drop the cap so cleanup must evict the three oldest stored snapshots
	// (a, b, c) and stop with the fresh one still stored.
	requestsnapshot_setting.GetSetting().MaxTotalMb = 2

	result, err := mgr.Cleanup(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.RetentionDeleted, 1)
	assert.GreaterOrEqual(t, result.CapacityDeleted, 3)

	// The old row became a tombstone.
	oldRow, err := model.GetRequestSnapshotByRequestId("req-retention-old")
	require.NoError(t, err)
	assert.Equal(t, model.RequestSnapshotStatusDeleted, oldRow.Status)

	// Fresh snapshot still readable (evicted last).
	fresh, err := mgr.Read(ctx, "req-fresh")
	require.NoError(t, err)
	assert.Equal(t, payload, fresh)

	// Capacity evicted oldest-first: cap-a (oldest stored) first, and cap-b
	// and cap-c before the fresh snapshot would ever be touched.
	for _, id := range []string{"req-cap-a", "req-cap-b", "req-cap-c"} {
		row, err := model.GetRequestSnapshotByRequestId(id)
		require.NoError(t, err)
		assert.Equal(t, model.RequestSnapshotStatusDeleted, row.Status, "%s must be evicted before the fresh snapshot", id)
	}
	freshRow, err := model.GetRequestSnapshotByRequestId("req-fresh")
	require.NoError(t, err)
	assert.Equal(t, model.RequestSnapshotStatusStored, freshRow.Status)
}

func TestCleanupOrphansMissingAndBoundedHistory(t *testing.T) {
	mgr := newTestManager(t, func(s *requestsnapshot_setting.RequestSnapshotSetting) {
		s.OrphanGraceMinutes = 1
		s.RetentionDays = 30
	})
	ctx := context.Background()
	dir := filepath.Join(requestsnapshot_setting.GetSetting().StoragePath, nodeDirName(common.NodeName))
	require.NoError(t, ensureSnapshotDir(dir))

	now := time.Now().Unix()

	// Own-node stored row whose file vanished -> missing.
	require.NoError(t, mgr.Capture(ctx, captureMeta("req-vanished"), []byte("gone")))
	row, err := model.GetRequestSnapshotByRequestId("req-vanished")
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(dir, row.RelativePath)))

	// Other-node row + file must never be touched.
	otherNode := "other-node"
	otherDir := filepath.Join(requestsnapshot_setting.GetSetting().StoragePath, nodeDirName(otherNode))
	require.NoError(t, ensureSnapshotDir(otherDir))
	otherRow := &model.RequestSnapshot{
		RequestId: "req-other", Node: otherNode, Status: model.RequestSnapshotStatusStored,
		RelativePath: "other.snap", CreatedAt: now - 100*24*60*60, UpdatedAt: now,
	}
	require.NoError(t, model.CreateRequestSnapshot(otherRow))
	require.NoError(t, os.WriteFile(filepath.Join(otherDir, "other.snap"), []byte("other node bytes"), 0o600))

	// Aged orphan file (older than grace, no row) -> removed.
	orphanPath := filepath.Join(dir, "orphan.snap")
	require.NoError(t, os.WriteFile(orphanPath, []byte("orphan"), 0o600))
	require.NoError(t, os.Chtimes(orphanPath, time.Now().Add(-2*time.Minute), time.Now().Add(-2*time.Minute)))

	// Young orphan (within grace) -> kept.
	youngPath := filepath.Join(dir, "young.snap")
	require.NoError(t, os.WriteFile(youngPath, []byte("young"), 0o600))

	// Old access audit row, old terminal metadata, and old tombstone -> pruned;
	// fresh records are kept.
	require.NoError(t, model.CreateRequestSnapshotAccess(&model.RequestSnapshotAccess{
		RequestId: "req-vanished", Action: model.RequestSnapshotActionRead, Success: true,
		Result: model.SnapshotResultOk, Node: common.NodeName,
	}))
	require.NoError(t, model.DB.Model(&model.RequestSnapshotAccess{}).
		Where("request_id = ? AND result = ?", "req-vanished", model.SnapshotResultOk).
		Update("created_at", now-40*24*60*60).Error)
	require.NoError(t, model.CreateRequestSnapshotAccess(&model.RequestSnapshotAccess{
		RequestId: "req-vanished", Action: model.RequestSnapshotActionRead, Success: true,
		Result: model.SnapshotResultOk, Node: common.NodeName,
	}))
	tombstoneRow := &model.RequestSnapshot{
		RequestId: "req-old-tombstone", Node: common.NodeName, Status: model.RequestSnapshotStatusDeleted,
		RelativePath: "old.snap",
	}
	require.NoError(t, model.CreateRequestSnapshot(tombstoneRow))
	require.NoError(t, model.DB.Model(&model.RequestSnapshot{}).
		Where("request_id = ?", "req-old-tombstone").
		Update("updated_at", now-40*24*60*60).Error)

	for _, terminal := range []*model.RequestSnapshot{
		{RequestId: "req-old-failed", Node: common.NodeName, Status: model.RequestSnapshotStatusFailed, ErrorCode: "capacity"},
		{RequestId: "req-old-missing", Node: common.NodeName, Status: model.RequestSnapshotStatusMissing, ErrorCode: "file_missing"},
		{RequestId: "req-fresh-failed", Node: common.NodeName, Status: model.RequestSnapshotStatusFailed, ErrorCode: "capacity"},
		{RequestId: "req-other-failed", Node: otherNode, Status: model.RequestSnapshotStatusFailed, ErrorCode: "capacity"},
	} {
		require.NoError(t, model.CreateRequestSnapshot(terminal))
	}
	require.NoError(t, model.DB.Model(&model.RequestSnapshot{}).
		Where("request_id IN ?", []string{"req-old-failed", "req-old-missing", "req-other-failed"}).
		Update("updated_at", now-40*24*60*60).Error)

	result, err := mgr.Cleanup(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.MarkedMissing, 1)

	vanished, err := model.GetRequestSnapshotByRequestId("req-vanished")
	require.NoError(t, err)
	assert.Equal(t, model.RequestSnapshotStatusMissing, vanished.Status)

	_, statErr := os.Stat(orphanPath)
	assert.True(t, os.IsNotExist(statErr), "aged orphan file must be removed")
	assert.GreaterOrEqual(t, result.OrphansRemoved, 1)

	_, statErr = os.Stat(youngPath)
	require.NoError(t, statErr, "young orphan file must be kept")

	// Other-node file and row are untouched.
	_, statErr = os.Stat(filepath.Join(otherDir, "other.snap"))
	require.NoError(t, statErr)
	other, err := model.GetRequestSnapshotByRequestId("req-other")
	require.NoError(t, err)
	assert.Equal(t, model.RequestSnapshotStatusStored, other.Status)

	// Bounded history: the aged tombstone, aged own-node terminal rows, and
	// aged access row are pruned. Fresh and other-node terminal rows survive.
	_, err = model.GetRequestSnapshotByRequestId("req-old-tombstone")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	for _, id := range []string{"req-old-failed", "req-old-missing"} {
		_, err = model.GetRequestSnapshotByRequestId(id)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "%s must be pruned", id)
	}
	for _, id := range []string{"req-fresh-failed", "req-other-failed"} {
		_, err = model.GetRequestSnapshotByRequestId(id)
		require.NoError(t, err, "%s must be retained", id)
	}
	assert.Equal(t, 2, result.TerminalRowsPruned)
	var accessCount int64
	require.NoError(t, model.DB.Model(&model.RequestSnapshotAccess{}).Count(&accessCount).Error)
	assert.Equal(t, int64(1), accessCount, "only the fresh access row survives")
}

func TestReadFailsClosedOnCorruptFile(t *testing.T) {
	mgr := newTestManager(t, nil)
	require.NoError(t, mgr.Capture(context.Background(), captureMeta("req-corrupt"), []byte("integrity matters")))

	row, err := model.GetRequestSnapshotByRequestId("req-corrupt")
	require.NoError(t, err)
	dir := filepath.Join(requestsnapshot_setting.GetSetting().StoragePath, nodeDirName(common.NodeName))
	require.NoError(t, os.WriteFile(filepath.Join(dir, row.RelativePath), []byte("garbage"), 0o600))

	_, err = mgr.Read(context.Background(), "req-corrupt")
	assert.ErrorIs(t, err, ErrSnapshotCorrupt)
}

func TestReadStatusesFailClosed(t *testing.T) {
	mgr := newTestManager(t, nil)

	t.Run("not found", func(t *testing.T) {
		_, err := mgr.Read(context.Background(), "req-unknown")
		assert.ErrorIs(t, err, ErrSnapshotNotFound)
	})

	t.Run("deleted", func(t *testing.T) {
		require.NoError(t, mgr.Capture(context.Background(), captureMeta("req-deleted"), []byte("bye")))
		require.NoError(t, mgr.Delete(context.Background(), "req-deleted"))
		_, err := mgr.Read(context.Background(), "req-deleted")
		assert.ErrorIs(t, err, ErrSnapshotDeleted)
	})

	t.Run("missing", func(t *testing.T) {
		require.NoError(t, mgr.Capture(context.Background(), captureMeta("req-missing"), []byte("gone")))
		row, err := model.GetRequestSnapshotByRequestId("req-missing")
		require.NoError(t, err)
		require.NoError(t, model.UpdateRequestSnapshotStatus("req-missing", model.RequestSnapshotStatusMissing, "file_missing"))
		_ = row
		_, err = mgr.Read(context.Background(), "req-missing")
		assert.ErrorIs(t, err, ErrSnapshotMissing)
	})

	t.Run("wrong node", func(t *testing.T) {
		otherRow := &model.RequestSnapshot{
			RequestId: "req-remote", Node: "remote-node", Status: model.RequestSnapshotStatusStored,
			RelativePath: "remote.snap",
		}
		require.NoError(t, model.CreateRequestSnapshot(otherRow))
		_, err := mgr.Read(context.Background(), "req-remote")
		owner, ok := IsWrongNodeError(err)
		require.True(t, ok)
		assert.Equal(t, "remote-node", owner)
	})
}

func TestDeleteOnlyTouchesOwnNodeStored(t *testing.T) {
	mgr := newTestManager(t, nil)
	require.NoError(t, mgr.Capture(context.Background(), captureMeta("req-del-own"), []byte("mine")))

	require.NoError(t, mgr.Delete(context.Background(), "req-del-own"))
	row, err := model.GetRequestSnapshotByRequestId("req-del-own")
	require.NoError(t, err)
	assert.Equal(t, model.RequestSnapshotStatusDeleted, row.Status)

	err = mgr.Delete(context.Background(), "req-never-existed")
	assert.ErrorIs(t, err, ErrSnapshotNotFound)

	otherRow := &model.RequestSnapshot{
		RequestId: "req-del-remote", Node: "remote", Status: model.RequestSnapshotStatusStored,
		RelativePath: "remote.snap",
	}
	require.NoError(t, model.CreateRequestSnapshot(otherRow))
	err = mgr.Delete(context.Background(), "req-del-remote")
	owner, ok := IsWrongNodeError(err)
	require.True(t, ok)
	assert.Equal(t, "remote", owner)
}

func TestSnapshotFileNamingIsSafeAndDeterministic(t *testing.T) {
	rel1, err := safeRelativePath("any-request-id")
	require.NoError(t, err)
	rel2, err := safeRelativePath("any-request-id")
	require.NoError(t, err)
	assert.Equal(t, rel1, rel2, "deterministic per request id")
	assert.True(t, validateRelativePath(rel1))

	_, err = safeRelativePath("")
	assert.Error(t, err)

	// Traversal attempts are rejected.
	assert.False(t, validateRelativePath("../escape"))
	assert.False(t, validateRelativePath("a/b"))
	assert.False(t, validateRelativePath(`a\b`))
	assert.False(t, validateRelativePath(".hidden"))
	assert.False(t, validateRelativePath("a..b"))
	assert.True(t, validateRelativePath("a-b_c.d"))
}

func TestStableKeyConfigured(t *testing.T) {
	previous := common.CryptoSecret
	t.Cleanup(func() { common.CryptoSecret = previous })

	t.Setenv("CRYPTO_SECRET", "")
	t.Setenv("SESSION_SECRET", "")
	common.CryptoSecret = ""
	assert.False(t, StableKeyConfigured())
	assert.False(t, stableKeyOperational())

	t.Setenv("CRYPTO_SECRET", "k")
	assert.True(t, StableKeyConfigured())
	assert.False(t, stableKeyOperational(), "an environment source without loaded key material must fail closed")
	common.CryptoSecret = "k"
	assert.True(t, stableKeyOperational())

	t.Setenv("CRYPTO_SECRET", "")
	t.Setenv("SESSION_SECRET", "s")
	common.CryptoSecret = "s"
	assert.True(t, StableKeyConfigured())
	assert.True(t, stableKeyOperational())
}
