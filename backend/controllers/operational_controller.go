package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/models"
	"coal-governance-backend/utils"
)

type OperationalController struct{}

func NewOperationalController() *OperationalController {
	return &OperationalController{}
}

// ListOperationalData returns production and operational telemetry.
func (oc *OperationalController) ListOperationalData(c *gin.Context) {
	query := `
		SELECT o.id, o.mine_id, m.mine_name, o.record_date,
		       o.production_tonnes, o.expected_production,
		       o.equipment_health_pct, o.attendance_pct, o.created_at
		FROM operational_data o
		JOIN mines m ON m.id = o.mine_id
		WHERE 1=1`
	args := []interface{}{}

	if mineID := c.Query("mine_id"); mineID != "" {
		query += " AND o.mine_id = ?"
		args = append(args, mineID)
	}
	if startDate := c.Query("start_date"); startDate != "" {
		query += " AND o.record_date >= ?"
		args = append(args, startDate)
	}
	if endDate := c.Query("end_date"); endDate != "" {
		query += " AND o.record_date <= ?"
		args = append(args, endDate)
	}

	query += " ORDER BY o.record_date DESC, o.id DESC LIMIT 150"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query operational data", err.Error())
		return
	}
	defer rows.Close()

	list := []models.OperationalData{}
	for rows.Next() {
		var o models.OperationalData
		var recordDate []uint8
		err := rows.Scan(
			&o.ID, &o.MineID, &o.MineName, &recordDate,
			&o.ProductionTonnes, &o.ExpectedProduction,
			&o.EquipmentHealthPct, &o.AttendancePct, &o.CreatedAt,
		)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse operational record", err.Error())
			return
		}
		o.RecordDate = string(recordDate)
		o.VarianceTonnes = o.ProductionTonnes - o.ExpectedProduction
		if o.ExpectedProduction > 0 {
			o.VariancePct = (o.VarianceTonnes / o.ExpectedProduction) * 100.0
		}
		list = append(list, o)
	}

	utils.Success(c, http.StatusOK, "Operational data fetched successfully", list)
}

type logOperationalRequest struct {
	MineID             int     `json:"mine_id" binding:"required"`
	RecordDate         string  `json:"record_date"` // YYYY-MM-DD
	ProductionTonnes   float64 `json:"production_tonnes" binding:"required"`
	ExpectedProduction float64 `json:"expected_production" binding:"required"`
	EquipmentHealthPct float64 `json:"equipment_health_pct" binding:"required"`
	AttendancePct      float64 `json:"attendance_pct" binding:"required"`
}

// LogOperationalData records daily production figures and equipment telemetry.
func (oc *OperationalController) LogOperationalData(c *gin.Context) {
	var req logOperationalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid operational payload", err.Error())
		return
	}

	if req.RecordDate == "" {
		req.RecordDate = time.Now().Format("2006-01-02")
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	var newID int64
	err := database.DB.QueryRow(`
		INSERT INTO operational_data (mine_id, record_date, production_tonnes, expected_production, equipment_health_pct, attendance_pct)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id`,
		req.MineID, req.RecordDate, req.ProductionTonnes, req.ExpectedProduction, req.EquipmentHealthPct, req.AttendancePct).Scan(&newID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to record operational data", err.Error())
		return
	}

	// Anomaly check: Production shortfall > 25% or Equipment Health < 75%
	if req.ExpectedProduction > 0 && req.ProductionTonnes < (req.ExpectedProduction*0.75) {
		var mineName string
		_ = database.DB.QueryRow(`SELECT mine_name FROM mines WHERE id = ?`, req.MineID).Scan(&mineName)

		desc := fmt.Sprintf("Production deficit at %s: Achieved %.1f tonnes vs Expected %.1f tonnes (%.1f%% drop)",
			mineName, req.ProductionTonnes, req.ExpectedProduction, (1.0-req.ProductionTonnes/req.ExpectedProduction)*100)

		_, _ = database.DB.Exec(`
			INSERT INTO anomalies (mine_id, anomaly_type, description, detected_value, expected_value, severity, status)
			VALUES (?, 'PRODUCTION_DROP', ?, ?, ?, 'HIGH', 'NEW')`,
			req.MineID, desc, req.ProductionTonnes, req.ExpectedProduction)
	}

	utils.LogAudit(userID, "OPERATIONAL_DATA_LOGGED", "OPERATIONAL", strconv.FormatInt(newID, 10),
		map[string]interface{}{"mine_id": req.MineID, "production_tonnes": req.ProductionTonnes}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Operational data logged successfully", gin.H{"id": newID})
}