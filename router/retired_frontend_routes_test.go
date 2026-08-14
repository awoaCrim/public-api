package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRetiredFrontendAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	_, hasAsyncCleanup := routes[http.MethodPost+" /api/system-task/log-cleanup"]
	_, hasDirectDelete := routes[http.MethodDelete+" /api/log/"]
	_, hasConsoleMigration := routes[http.MethodPost+" /api/option/migrate_console_setting"]
	_, hasUsageAnalysis := routes[http.MethodGet+" /api/usage-analysis"]
	_, hasUsageAnalysisOptions := routes[http.MethodGet+" /api/usage-analysis/options"]
	assert.True(t, hasAsyncCleanup)
	assert.True(t, hasUsageAnalysis)
	assert.True(t, hasUsageAnalysisOptions)
	assert.False(t, hasDirectDelete)
	assert.False(t, hasConsoleMigration)
}
