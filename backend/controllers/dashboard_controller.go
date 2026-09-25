package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/utils"
)

type DashboardController struct{}

func NewDashboardController() *DashboardController {
	return &DashboardController{}
}

// DashboardSummary returns the top-level KPI cards shown on the main dashboard.
// Phase 1 provides mine counts; violation/compliance/risk figures will populate
// once the corresponding Phase 2/4 modules are implemented and seeded with data,
// but the fields are always present so the frontend can render consistently.
func (dc *DashboardController) DashboardSummary(c *gin.Context) {
	var totalMines, activeMines int
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM mines`).Scan(&totalMines)
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM mines WHERE status='ACTIVE'`).Scan(&activeMines)

	var openViolations, criticalViolations int
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM violations WHERE status IN ('OPEN','IN_PROGRESS','OVERDUE')`).Scan(&openViolations)
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM violations WHERE status IN ('OPEN','IN_PROGRESS','OVERDUE') AND severity='CRITICAL'`).Scan(&criticalViolations)

	var overdueActions int
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM corrective_actions WHERE status='OVERDUE'`).Scan(&overdueActions)

	var highRiskMines int
	_ = database.DB.QueryRow(`
		SELECT COUNT(DISTINCT mine_id) FROM risk_scores rs
		WHERE rs.classification IN ('HIGH','CRITICAL')
		AND rs.computed_at = (SELECT MAX(computed_at) FROM risk_scores WHERE mine_id = rs.mine_id)`).Scan(&highRiskMines)

	var totalRules, activeRules int
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM compliance_rules`).Scan(&totalRules)
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM compliance_rules WHERE status='ACTIVE'`).Scan(&activeRules)

	utils.Success(c, http.StatusOK, "Dashboard summary fetched successfully", gin.H{
		"total_mines":             totalMines,
		"active_mines":            activeMines,
		"overall_compliance":      0, // populated once inspection/compliance-status engine (Phase 2) runs
		"open_violations":         openViolations,
		"critical_violations":     criticalViolations,
		"overdue_actions":         overdueActions,
		"high_risk_mines":         highRiskMines,
		"total_compliance_rules":  totalRules,
		"active_compliance_rules": activeRules,
	})
}
