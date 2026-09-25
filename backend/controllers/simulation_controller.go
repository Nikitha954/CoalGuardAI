package controllers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/utils"
)

type SimulationController struct{}

func NewSimulationController() *SimulationController {
	return &SimulationController{}
}

type modeRequest struct {
	Mode string `json:"mode" binding:"required"`
}

// SetMode writes the target simulation state flag to the shared configuration file.
func (sc *SimulationController) SetMode(c *gin.Context) {
	var req modeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	mode := strings.TrimSpace(req.Mode)
	
	// Write to shared txt file at root of workspace (parent of backend directory)
	filePath := filepath.Join("..", "simulator_mode.txt")
	err := os.WriteFile(filePath, []byte(mode), 0644)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to write simulator mode file", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)
	utils.LogAudit(userID.(int), "SIMULATION_MODE_CHANGED", "SIMULATOR", mode, nil, c.ClientIP())

	utils.Success(c, http.StatusOK, "Simulation mode updated successfully", gin.H{"mode": mode})
}

// GetMode reads the active simulation mode from the shared config file.
func (sc *SimulationController) GetMode(c *gin.Context) {
	filePath := filepath.Join("..", "simulator_mode.txt")
	bytes, err := os.ReadFile(filePath)
	mode := "NORMAL_MODE"
	if err == nil {
		mode = strings.TrimSpace(string(bytes))
	}
	utils.Success(c, http.StatusOK, "Simulation mode fetched", gin.H{"mode": mode})
}

// ResetDemo cleans all dynamically generated simulator anomalies, incidents, and violations for Gevra Mine.
func (sc *SimulationController) ResetDemo(c *gin.Context) {
	tx, err := database.DB.Begin()
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to start transaction", err.Error())
		return
	}
	defer tx.Rollback()

	// 1. Delete notifications
	_, _ = tx.Exec(`DELETE FROM notifications`)

	// 2. Delete corrective actions
	_, _ = tx.Exec(`
		DELETE ca FROM corrective_actions ca 
		JOIN violations v ON v.id = ca.violation_id 
		WHERE v.mine_id = 1`)

	// 3. Delete violations
	_, _ = tx.Exec(`DELETE FROM violations WHERE mine_id = 1`)

	// 4. Delete incidents
	_, _ = tx.Exec(`DELETE FROM incidents WHERE mine_id = 1`)

	// 5. Delete anomalies
	_, _ = tx.Exec(`DELETE FROM anomalies WHERE mine_id = 1`)

	// 6. Delete risk scores
	_, _ = tx.Exec(`DELETE FROM risk_scores WHERE mine_id = 1`)

	// 7. Delete inspections
	_, _ = tx.Exec(`DELETE FROM inspections WHERE mine_id = 1`)

	// 8. Delete operational data and environmental data for today
	_, _ = tx.Exec(`DELETE FROM operational_data WHERE mine_id = 1`)
	_, _ = tx.Exec(`DELETE FROM environmental_data WHERE mine_id = 1`)

	if err := tx.Commit(); err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to reset database logs", err.Error())
		return
	}

	// 9. Reset file mode
	filePath := filepath.Join("..", "simulator_mode.txt")
	_ = os.WriteFile(filePath, []byte("NORMAL_MODE"), 0644)

	userID, _ := c.Get(middleware.CtxUserID)
	utils.LogAudit(userID.(int), "SIMULATION_DEMO_RESET", "SIMULATOR", "NORMAL_MODE", nil, c.ClientIP())

	utils.Success(c, http.StatusOK, "Demo database records successfully restored to seed baseline", nil)
}
