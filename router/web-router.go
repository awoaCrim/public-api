package router

import (
	"embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// WebAssets holds the embedded dashboard frontend assets.
type WebAssets struct {
	BuildFS   embed.FS
	IndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets WebAssets) {
	frontendFS := common.EmbedFolder(assets.BuildFS, "web/dist")

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	// Keep data assets ahead of frontend static serving and the SPA fallback.
	// The handler deliberately returns 404 for every missing, non-image,
	// traversal, or out-of-root symlink path so those requests cannot be
	// index.html.
	router.Use(serveDataAssets("/data-assets/", "/data"))
	router.Use(static.Serve("/", frontendFS))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", assets.IndexPage)
	})
}

// serveDataAssets exposes approved image files from the mounted data directory
// without allowing missing or invalid asset paths to fall through to the SPA.
func serveDataAssets(urlPrefix, localDir string) gin.HandlerFunc {
	basePath := strings.TrimSuffix(urlPrefix, "/")
	prefix := basePath + "/"
	rootDir, err := filepath.Abs(localDir)
	if err != nil {
		rootDir = localDir
	}

	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if requestPath != basePath && !strings.HasPrefix(requestPath, prefix) {
			return
		}

		c.Set(middleware.RouteTagKey, "web")
		notFound := func() {
			c.AbortWithStatus(http.StatusNotFound)
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			notFound()
			return
		}

		relPath := strings.TrimPrefix(requestPath, prefix)
		if relPath == "" {
			notFound()
			return
		}
		relPath = filepath.Clean(filepath.FromSlash(relPath))
		if relPath == "." || relPath == ".." || filepath.IsAbs(relPath) || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
			notFound()
			return
		}

		switch strings.ToLower(filepath.Ext(relPath)) {
		case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp":
		default:
			notFound()
			return
		}

		rootPath, err := filepath.EvalSymlinks(rootDir)
		if err != nil {
			notFound()
			return
		}
		assetPath, err := filepath.EvalSymlinks(filepath.Join(rootDir, relPath))
		if err != nil || !isPathWithinRoot(rootPath, assetPath) {
			notFound()
			return
		}
		info, err := os.Stat(assetPath)
		if err != nil || !info.Mode().IsRegular() {
			notFound()
			return
		}

		c.Header("Cache-Control", "public, max-age=86400")
		c.File(assetPath)
		c.Abort()
	}
}

func isPathWithinRoot(rootPath, targetPath string) bool {
	relPath, err := filepath.Rel(rootPath, targetPath)
	if err != nil || relPath == ".." {
		return false
	}
	return !strings.HasPrefix(relPath, ".."+string(filepath.Separator)) && !filepath.IsAbs(relPath)
}
