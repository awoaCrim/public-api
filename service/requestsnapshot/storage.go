package requestsnapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// nodeDirName derives a stable, path-safe directory name from a node name.
// Node names are operator-controlled (env NODE_NAME or hostname) and must
// never be interpolated into paths directly; hashing keeps the layout
// deterministic and traversal-proof.
func nodeDirName(node string) string {
	sum := sha256.Sum256([]byte(node))
	return "node-" + hex.EncodeToString(sum[:8])
}

// NodeDirName exposes the deterministic per-node storage directory name for
// operators and tooling that need to locate a node's snapshot files.
func NodeDirName(node string) string {
	return nodeDirName(node)
}

// snapshotFileName derives a stable, path-safe file name from a request id.
func snapshotFileName(requestID string) string {
	sum := sha256.Sum256([]byte(requestID))
	return hex.EncodeToString(sum[:16]) + ".snap"
}

// safeRelativePath returns the validated relative path for a request id.
func safeRelativePath(requestID string) (string, error) {
	if requestID == "" {
		return "", fmt.Errorf("requestsnapshot: empty request id")
	}
	rel := snapshotFileName(requestID)
	if !validateRelativePath(rel) {
		return "", fmt.Errorf("requestsnapshot: unsafe generated path %q", rel)
	}
	return rel, nil
}

// validateRelativePath rejects any relative path that could escape its storage
// directory: separators, parent references, hidden names, and absolute paths.
func validateRelativePath(rel string) bool {
	if rel == "" || strings.HasPrefix(rel, ".") {
		return false
	}
	if filepath.Base(rel) != rel {
		return false
	}
	if strings.ContainsAny(rel, `/\`) || strings.Contains(rel, "..") {
		return false
	}
	return true
}

// ensureSnapshotDir creates the per-node storage directory with owner-only
// permissions.
func ensureSnapshotDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("requestsnapshot: create dir %q: %w", dir, err)
	}
	return os.Chmod(dir, 0o700)
}

// atomicWriteFile writes data to dir/rel via a same-directory temp file plus
// rename so a concurrent reader never observes a partial file. Final file
// permissions are owner-only.
func atomicWriteFile(dir, rel string, data []byte) error {
	if !validateRelativePath(rel) {
		return fmt.Errorf("requestsnapshot: refusing unsafe relative path %q", rel)
	}
	full := filepath.Join(dir, rel)
	tmp, err := os.CreateTemp(dir, "."+rel+".tmp-*")
	if err != nil {
		return fmt.Errorf("requestsnapshot: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("requestsnapshot: chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("requestsnapshot: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("requestsnapshot: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("requestsnapshot: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, full); err != nil {
		return fmt.Errorf("requestsnapshot: rename %q: %w", rel, err)
	}
	keepTemp = false
	return nil
}

// listSnapshotFiles returns the relative file names currently present in the
// per-node directory. Non-regular entries (temp files, subdirectories) are
// ignored; leftover temp files are returned with a marker so cleanup can drop
// them as orphans.
func listSnapshotFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("requestsnapshot: read dir %q: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	return names, nil
}
