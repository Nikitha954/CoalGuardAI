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

type EnvironmentalController struct{}

func NewEnvironmentalController() *EnvironmentalController {
	return &EnvironmentalController{}
}

// ListEnvironmentalData returns historical environmental telemetry.
func (ec *EnvironmentalController) ListEnvironmentalData(c *gin.Context) {
	query := `
		SELECT e.id, e.mine_id, m.mine_name, e.record_date,
			   e.aqi, e.water_quality_index, e.noise_level_db, e.dust_level, e.created_at
		FROM environmental_data e
		JOIN mines m ON m.id = e.mine_id
		WHERE 1=1`
	args := []interface{}{}

	if mineID := c.Query("mine_id"); mineID != "" {
		query += " AND e.mine_id = ?"
		args = append(args, mineID)
	}
	if startDate := c.Query("start_date"); startDate != "" {
		query += " AND e.record_date >= ?"
		args = append(args, startDate)
	}
	if endDate := c.Query("end_date"); endDate != "" {
		query += " AND e.record_date <= ?"
		args = append(args, endDate)
	}

	query += " ORDER BY e.record_date DESC, e.id DESC LIMIT 150"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query environmental data", err.Error())
		return
	}
	defer rows.Close()

	list := []models.EnvironmentalData{}
	for rows.Next() {
		var e models.EnvironmentalData
		var recordDate []uint8
		err := rows.Scan(&e.ID, &e.MineID, &e.MineName, &recordDate, &e.AQI, &e.WaterQualityIndex, &e.NoiseLevelDB, &e.DustLevel, &e.CreatedAt)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse environmental record", err.Error())
			return
		}
		e.RecordDate = string(recordDate)
		list = append(list, e)
	}

	utils.Success(c, http.StatusOK, "Environmental telemetry fetched successfully", list)
}

type logEnvironmentalRequest struct {
	MineID            int     `json:"mine_id" binding:"required"`
	RecordDate        string  `json:"record_date"` // YYYY-MM-DD
	AQI               float64 `json:"aqi" binding:"required"`
	WaterQualityIndex float64 `json:"water_quality_index" binding:"required"`
	NoiseLevelDB      float64 `json:"noise_level_db" binding:"required"`
	DustLevel         float64 `json:"dust_level" binding:"required"`
}

// LogEnvironmentalReading records daily environmental sensor readings and checks CPCB thresholds.
func (ec *EnvironmentalController) LogEnvironmentalReading(c *gin.Context) {
	var req logEnvironmentalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid environmental payload", err.Error())
		return
	}

	if req.RecordDate == "" {
		req.RecordDate = time.Now().Format("2006-01-02")
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	var recordID int64
	err := database.DB.QueryRow(`
		INSERT INTO environmental_data (mine_id, record_date, aqi, water_quality_index, noise_level_db, dust_level) 
		VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		req.MineID, req.RecordDate, req.AQI, req.WaterQualityIndex, req.NoiseLevelDB, req.DustLevel).Scan(&recordID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to record environmental data", err.Error())
		return
	}

	// Check if parameters exceed statutory thresholds (e.g. AQI > 200 or Dust > 150)
	if req.AQI > 200 || req.DustLevel > 150 || req.NoiseLevelDB > 85 {
		var mineName string
		_ = database.DB.QueryRow(`SELECT mine_name FROM mines WHERE id = ?`, req.MineID).Scan(&mineName)

		title := fmt.Sprintf("Environmental Threshold Exceeded — %s", mineName)
		msg := fmt.Sprintf("High pollution detected: AQI=%.1f, Dust=%.1f ug/m3, Noise=%.1f dB. Mandatory mitigation required.",
			req.AQI, req.DustLevel, req.NoiseLevelDB)

		// Log anomaly
		_, _ = database.DB.Exec(`
			INSERT INTO anomalies (mine_id, anomaly_type, description, detected_value, expected_value, severity, status)
			VALUES (?, 'ENVIRONMENTAL_BREACH', ?, ?, 100.0, 'HIGH', 'NEW')`,
			req.MineID, msg, req.AQI)

		// Fan out notification
		rows, _ := database.DB.Query(`
			SELECT u.id FROM users u
			JOIN roles r ON r.id = u.role_id
			WHERE u.status = 'ACTIVE' AND r.role_key IN ('SAFETY_OFFICER', 'MINE_MANAGER', 'REGULATORY_OFFICER', 'SUPER_ADMIN')`)
		if rows != nil {
			for rows.Next() {
				var recID int
				if err := rows.Scan(&recID); err == nil {
					_, _ = database.DB.Exec(`
						INSERT INTO notifications (recipient_id, title, message, severity, type, is_read)
						VALUES (?, ?, ?, 'WARNING', 'ENVIRONMENTAL_ALERT', FALSE)`,
						recID, title, msg)
				}
			}
			rows.Close()
		}
	}

	utils.LogAudit(userID, "ENVIRONMENTAL_DATA_LOGGED", "ENVIRONMENTAL", strconv.FormatInt(recordID, 10),
		map[string]interface{}{"mine_id": req.MineID, "aqi": req.AQI, "dust": req.DustLevel}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Environmental reading logged successfully", gin.H{"id": recordID})
}
