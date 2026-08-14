package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// RoutingGroupMigrationPreview returns the read-only dry-run report of the
// legacy routing-group compatibility migration. Root only; never writes.
func RoutingGroupMigrationPreview(c *gin.Context) {
	report, err := service.PreviewRoutingGroupMigration()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}

// RoutingGroupMigrationStatus returns the migration marker plus a live
// reconciliation summary (pending imports, unmappable references, readiness).
func RoutingGroupMigrationStatus(c *gin.Context) {
	status, err := service.GetRoutingGroupMigrationStatus()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ready, blockers, err := service.RoutingGroupMigrationReadiness()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"status":   status,
		"ready":    ready,
		"blockers": blockers,
	})
}

// RoutingGroupMigrationRun executes the strict compatibility migration.
// Fail-closed: any active unmappable legacy token or grant blocks the run and
// the blocking report is returned without writing anything. On success the
// idempotent import runs in one transaction and the version marker is
// persisted; the legacy tables are never written or deleted.
func RoutingGroupMigrationRun(c *gin.Context) {
	report, err := service.MigrateRoutingGroupCompatibilityDataStrict()
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    report,
		})
		return
	}
	recordManageAudit(c, "routing_group_migration.run", map[string]interface{}{
		"imported_grants": len(report.GrantImports),
		"token_updates":   len(report.TokenUpdates),
	})
	common.ApiSuccess(c, report)
}
