package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDataAssetTestRouter(t *testing.T, dataDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(serveDataAssets("/data-assets/", dataDir))
	engine.NoRoute(func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("spa-index"))
	})
	return engine
}

func TestServeDataAssetsReturnsImageAndPreservesBytes(t *testing.T) {
	dataDir := t.TempDir()
	assetBytes := []byte("\x89PNG\r\n\x1a\nfixture-avatar")
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "avatar.png"), assetBytes, 0o644))
	engine := newDataAssetTestRouter(t, dataDir)

	request := httptest.NewRequest(http.MethodGet, "/data-assets/avatar.png", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, assetBytes, response.Body.Bytes())
	assert.True(t, strings.HasPrefix(response.Header().Get("Content-Type"), "image/png"))
	assert.Equal(t, "public, max-age=86400", response.Header().Get("Cache-Control"))
}

func TestServeDataAssetsSupportsHeadWithoutExposingBody(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "avatar.png"), []byte("\x89PNG\r\n\x1a\nfixture-avatar"), 0o644))
	engine := newDataAssetTestRouter(t, dataDir)

	request := httptest.NewRequest(http.MethodHead, "/data-assets/avatar.png", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Body.Bytes())
	assert.True(t, strings.HasPrefix(response.Header().Get("Content-Type"), "image/png"))
}

func TestServeDataAssetsRejectsWriteMethods(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "avatar.png"), []byte("png"), 0o644))
	engine := newDataAssetTestRouter(t, dataDir)

	request := httptest.NewRequest(http.MethodPost, "/data-assets/avatar.png", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusNotFound, response.Code)
}

func TestServeDataAssetsRejectsInvalidPathsBeforeSPAFallback(t *testing.T) {
	dataDir := t.TempDir()
	outsideDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "notes.txt"), []byte("private"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dataDir, "directory.png"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "secret.png"), []byte("secret"), 0o644))
	engine := newDataAssetTestRouter(t, dataDir)

	tests := []string{
		"/data-assets/missing.png",
		"/data-assets/notes.txt",
		"/data-assets/directory.png",
		"/data-assets/../" + filepath.Base(outsideDir) + "/secret.png",
		"/data-assets/%2e%2e/" + filepath.Base(outsideDir) + "/secret.png",
		"/data-assets/",
	}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)

			assert.Equal(t, http.StatusNotFound, response.Code)
			assert.NotEqual(t, []byte("spa-index"), response.Body.Bytes())
			assert.NotContains(t, response.Body.String(), "secret")
		})
	}
}

func TestServeDataAssetsRejectsSymlinkOutsideRoot(t *testing.T) {
	dataDir := t.TempDir()
	outsideFile := filepath.Join(t.TempDir(), "secret.png")
	require.NoError(t, os.WriteFile(outsideFile, []byte("secret"), 0o644))
	linkPath := filepath.Join(dataDir, "linked.png")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	engine := newDataAssetTestRouter(t, dataDir)

	request := httptest.NewRequest(http.MethodGet, "/data-assets/linked.png", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusNotFound, response.Code)
	assert.False(t, bytes.Contains(response.Body.Bytes(), []byte("secret")))
}

func TestServeDataAssetsLeavesSPAFallbackForUnrelatedPaths(t *testing.T) {
	engine := newDataAssetTestRouter(t, t.TempDir())

	request := httptest.NewRequest(http.MethodGet, "/dashboard/settings", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "spa-index", response.Body.String())
}
