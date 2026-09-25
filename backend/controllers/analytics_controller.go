package controllers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/config"
	"coal-governance-backend/database"
	"coal-governance-backend/utils"
)

type AnalyticsController struct {
	Cfg *config.Config
}

func NewAnalyticsController(cfg *config.Config) *AnalyticsController {
	return &AnalyticsController{Cfg: cfg}
}

// GetChartsData returns aggregated records for the 7 Chart.js dashboard charts.
func (ac *AnalyticsController) GetChartsData(c *gin.Context) {
	// First update risk scores to keep charts live
	_ = ac.RecalculateRiskForMines()

	// 1. Violations by Category
	vioCatRows, err := database.DB.Query(`
		SELECT cc.name, COUNT(*)
		FROM violations v
		JOIN compliance_categories cc ON cc.id = v.category_id
		GROUP BY cc.name`)
	type labelCount struct {
		Label string `json:"label"`
		Count int    `json:"count"`
	}
	vioCategories := []labelCount{}
	if err == nil {
		defer vioCatRows.Close()
		for vioCatRows.Next() {
			var lc labelCount
			if err := vioCatRows.Scan(&lc.Label, &lc.Count); err == nil {
				vioCategories = append(vioCategories, lc)
			}
		}
	}

	// 2. Corrective Action Statuses
	caStatusRows, err := database.DB.Query(`SELECT status, COUNT(*) FROM corrective_actions GROUP BY status`)
	caStatuses := []labelCount{}
	if err == nil {
		defer caStatusRows.Close()
		for caStatusRows.Next() {
			var lc labelCount
			if err := caStatusRows.Scan(&lc.Label, &lc.Count); err == nil {
				caStatuses = append(caStatuses, lc)
			}
		}
	}

	// 3. Risk Distribution
	riskDistRows, err := database.DB.Query(`
		SELECT r.classification, COUNT(*)
		FROM risk_scores r
		WHERE r.computed_at = (SELECT MAX(computed_at) FROM risk_scores WHERE mine_id = r.mine_id)
		GROUP BY r.classification`)
	riskDist := []labelCount{}
	if err == nil {
		defer riskDistRows.Close()
		for riskDistRows.Next() {
			var lc labelCount
			if err := riskDistRows.Scan(&lc.Label, &lc.Count); err == nil {
				riskDist = append(riskDist, lc)
			}
		}
	}

	// 4. Mine Risk Rankings
    // Notice: IFNULL (MySQL) is changed to COALESCE (Postgres)
	mineRankRows, err := database.DB.Query(`
		SELECT m.mine_name, COALESCE(r.score, 0)
		FROM mines m
		LEFT JOIN risk_scores r ON r.mine_id = m.id AND r.computed_at = (SELECT MAX(computed_at) FROM risk_scores WHERE mine_id = m.id)
		ORDER BY r.score DESC, m.mine_name ASC
		LIMIT 10`)
	type mineRank struct {
		MineName string  `json:"mine_name"`
		Score    float64 `json:"score"`
	}
	mineRanks := []mineRank{}
	if err == nil {
		defer mineRankRows.Close()
		for mineRankRows.Next() {
			var mr mineRank
			if err := mineRankRows.Scan(&mr.MineName, &mr.Score); err == nil {
				mineRanks = append(mineRanks, mr)
			}
		}
	}

	// 5. Compliance Trend (dynamic past 6 months of inspections)
    // Notice: DATE_FORMAT (MySQL) is changed to TO_CHAR (Postgres)
	complianceTrendRows, err := database.DB.Query(`
		SELECT TO_CHAR(inspection_date, 'Mon YYYY') as month_yr,
			   SUM(CASE WHEN status='APPROVED' THEN 1 ELSE 0 END) as approved_count,
			   COUNT(*) as total_count
		FROM inspections
		GROUP BY month_yr
		ORDER BY MIN(inspection_date) ASC
		LIMIT 6`)
	type trendPoint struct {
		Month string  `json:"month"`
		Rate  float64 `json:"rate"`
	}
	complianceTrend := []trendPoint{}
	if err == nil {
		defer complianceTrendRows.Close()
		for complianceTrendRows.Next() {
			var monthYr string
			var approved, total int
			if err := complianceTrendRows.Scan(&monthYr, &approved, &total); err == nil {
				rate := 100.0
				if total > 0 {
					rate = (float64(approved) / float64(total)) * 100.0
				}
				complianceTrend = append(complianceTrend, trendPoint{Month: monthYr, Rate: rate})
			}
		}
	}

	// 6. Inspection Trend
	inspectionTrendRows, err := database.DB.Query(`
		SELECT TO_CHAR(inspection_date, 'Mon YYYY') as month_yr, COUNT(*)
		FROM inspections
		GROUP BY month_yr
		ORDER BY MIN(inspection_date) ASC
		LIMIT 6`)
	inspectionsTrend := []labelCount{}
	if err == nil {
		defer inspectionTrendRows.Close()
		for inspectionTrendRows.Next() {
			var lc labelCount
			if err := inspectionTrendRows.Scan(&lc.Label, &lc.Count); err == nil {
				inspectionsTrend = append(inspectionsTrend, lc)
			}
		}
	}

	// 7. Incident Trend
	incidentRows, err := database.DB.Query(`SELECT incident_type, COUNT(*) FROM incidents GROUP BY incident_type`)
	incidentTrend := []labelCount{}
	if err == nil {
		defer incidentRows.Close()
		for incidentRows.Next() {
			var lc labelCount
			if err := incidentRows.Scan(&lc.Label, &lc.Count); err == nil {
				incidentTrend = append(incidentTrend, lc)
			}
		}
	}

	utils.Success(c, http.StatusOK, "Charts data loaded", gin.H{
		"violations_by_category":    vioCategories,
		"corrective_actions_status": caStatuses,
		"risk_distribution":         riskDist,
		"mine_risk_ranking":         mineRanks,
		"compliance_trend":          complianceTrend,
		"inspections_trend":         inspectionsTrend,
		"incidents_trend":           incidentTrend,
	})
}

// GetRiskScores returns the computed risk scores and explanations.
func (ac *AnalyticsController) GetRiskScores(c *gin.Context) {
	// Re-run computation first to ensure freshness
	_ = ac.RecalculateRiskForMines()

	rows, err := database.DB.Query(`
		SELECT r.id, r.mine_id, m.mine_name, r.score, r.classification, r.factors_json, r.computed_at
		FROM risk_scores r
		JOIN mines m ON m.id = r.mine_id
		WHERE r.computed_at = (SELECT MAX(computed_at) FROM risk_scores WHERE mine_id = r.mine_id)
		ORDER BY r.score DESC`)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query risk scores", err.Error())
		return
	}
	defer rows.Close()

	type riskItem struct {
		ID             int             `json:"id"`
		MineID         int             `json:"mine_id"`
		MineName       string          `json:"mine_name"`
		Score          float64         `json:"score"`
		Classification string          `json:"classification"`
		Factors        json.RawMessage `json:"factors"`
		ComputedAt     time.Time       `json:"computed_at"`
	}

	scores := []riskItem{}
	for rows.Next() {
		var ri riskItem
		var factors sql.NullString
		err := rows.Scan(&ri.ID, &ri.MineID, &ri.MineName, &ri.Score, &ri.Classification, &factors, &ri.ComputedAt)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse risk scores", err.Error())
			return
		}
		if factors.Valid {
			ri.Factors = json.RawMessage(factors.String)
		} else {
			ri.Factors = json.RawMessage("{}")
		}
		scores = append(scores, ri)
	}

	utils.Success(c, http.StatusOK, "Risk scores fetched", scores)
}

// TriggerRiskRecalculate handles manual recalculation trigger.
func (ac *AnalyticsController) TriggerRiskRecalculate(c *gin.Context) {
	err := ac.RecalculateRiskForMines()
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Risk recalculation failed", err.Error())
		return
	}
	utils.Success(c, http.StatusOK, "Risk scores successfully re-evaluated", nil)
}

func (ac *AnalyticsController) HandleVoiceQuery(c *gin.Context) {
	var req struct {
		Query    string `json:"query"`
		Language string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Valid query text is required"})
		return
	}

	userID, _ := c.Get("userID")

	// Context Gathering: Fetch current risk scores for mines
	mineRankRows, err := database.DB.Query(`
		SELECT m.mine_name, COALESCE(r.score, 0)
		FROM mines m
		LEFT JOIN risk_scores r ON r.mine_id = m.id AND r.computed_at = (SELECT MAX(computed_at) FROM risk_scores WHERE mine_id = m.id)
		ORDER BY r.score DESC
		LIMIT 20`)
	var contextMines []map[string]interface{}
	if err == nil {
		defer mineRankRows.Close()
		for mineRankRows.Next() {
			var name string
			var score float64
			if err := mineRankRows.Scan(&name, &score); err == nil {
				contextMines = append(contextMines, map[string]interface{}{"mine_name": name, "risk_score": score})
			}
		}
	}

	// Also fetch pending violations counts
	vioRows, err := database.DB.Query(`
		SELECT m.mine_name, COUNT(v.id) 
		FROM violations v
		JOIN mines m ON v.mine_id = m.id
		WHERE v.status = 'OPEN'
		GROUP BY m.mine_name`)
	var contextViolations []map[string]interface{}
	if err == nil {
		defer vioRows.Close()
		for vioRows.Next() {
			var name string
			var count int
			if err := vioRows.Scan(&name, &count); err == nil {
				contextViolations = append(contextViolations, map[string]interface{}{"mine_name": name, "open_violations": count})
			}
		}
	}

	contextData := map[string]interface{}{
		"mines_risk":         contextMines,
		"pending_violations": contextViolations,
	}

	payload := map[string]interface{}{
		"query":        req.Query,
		"language":     req.Language,
		"context_data": contextData,
	}
	payloadBytes, _ := json.Marshal(payload)

	aiURL := ac.Cfg.AIServiceURL + "/ai/voice-assistant"
	resp, err := http.Post(aiURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to reach AI service"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var aiResult map[string]interface{}
	json.Unmarshal(body, &aiResult)

	// Log audit (Notice: ? changed to $1, $2, $3)
	database.DB.Exec(`
		INSERT INTO audit_logs (user_id, action, details)
		VALUES ($1, $2, $3)`,
		userID, "Voice Query", fmt.Sprintf("Query: %s | Lang: %s", req.Query, req.Language))

	c.JSON(http.StatusOK, aiResult)
}

// TranslateText forwards text to Python AI service for translation.
func (ac *AnalyticsController) TranslateText(c *gin.Context) {
	var req struct {
		Text           string `json:"text" binding:"required"`
		TargetLanguage string `json:"target_language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Text is required"})
		return
	}

	if req.TargetLanguage == "" {
		req.TargetLanguage = "English"
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"text":            req.Text,
		"target_language": req.TargetLanguage,
	})

	aiURL := ac.Cfg.AIServiceURL + "/ai/translate"
	resp, err := http.Post(aiURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to reach AI service"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var aiResult map[string]interface{}
	json.Unmarshal(body, &aiResult)

	c.JSON(http.StatusOK, aiResult)
}

// GetAnomalies returns all registered operational/environmental anomalies.
func (ac *AnalyticsController) GetAnomalies(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT a.id, a.mine_id, m.mine_name, a.anomaly_type, a.description, a.detected_value, a.expected_value, a.severity, a.status, a.detected_at
		FROM anomalies a
		JOIN mines m ON m.id = a.mine_id
		ORDER BY a.detected_at DESC`)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query anomalies", err.Error())
		return
	}
	defer rows.Close()

	type anomalyItem struct {
		ID            int       `json:"id"`
		MineID        int       `json:"mine_id"`
		MineName      string    `json:"mine_name"`
		AnomalyType   string    `json:"anomaly_type"`
		Description   string    `json:"description"`
		DetectedValue float64   `json:"detected_value"`
		ExpectedValue float64   `json:"expected_value"`
		Severity      string    `json:"severity"`
		Status        string    `json:"status"`
		DetectedAt    time.Time `json:"detected_at"`
	}

	list := []anomalyItem{}
	for rows.Next() {
		var ai anomalyItem
		err := rows.Scan(&ai.ID, &ai.MineID, &ai.MineName, &ai.AnomalyType, &ai.Description, &ai.DetectedValue, &ai.ExpectedValue, &ai.Severity, &ai.Status, &ai.DetectedAt)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse anomaly", err.Error())
			return
		}
		list = append(list, ai)
	}

	utils.Success(c, http.StatusOK, "Anomalies fetched", list)
}

// RecalculateRiskForMines runs telemetry gathering and posts stats to Python AI service.
func (ac *AnalyticsController) RecalculateRiskForMines() error {
	minesRows, err := database.DB.Query(`SELECT id, mine_name FROM mines WHERE status='ACTIVE'`)
	if err != nil {
		return err
	}
	defer minesRows.Close()

	type mineInfo struct {
		ID   int
		Name string
	}
	mines := []mineInfo{}
	for minesRows.Next() {
		var mi mineInfo
		if err := minesRows.Scan(&mi.ID, &mi.Name); err == nil {
			mines = append(mines, mi)
		}
	}

	for _, mine := range mines {
		// Gather violations counts (Notice: ? changed to $1)
		var critVal, highVal, medVal, lowVal int
		_ = database.DB.QueryRow(`SELECT COUNT(*) FROM violations WHERE mine_id=$1 AND status IN ('OPEN','IN_PROGRESS','OVERDUE') AND severity='CRITICAL'`, mine.ID).Scan(&critVal)
		_ = database.DB.QueryRow(`SELECT COUNT(*) FROM violations WHERE mine_id=$1 AND status IN ('OPEN','IN_PROGRESS','OVERDUE') AND severity='HIGH'`, mine.ID).Scan(&highVal)
		_ = database.DB.QueryRow(`SELECT COUNT(*) FROM violations WHERE mine_id=$1 AND status IN ('OPEN','IN_PROGRESS','OVERDUE') AND severity='MEDIUM'`, mine.ID).Scan(&medVal)
		_ = database.DB.QueryRow(`SELECT COUNT(*) FROM violations WHERE mine_id=$1 AND status IN ('OPEN','IN_PROGRESS','OVERDUE') AND severity='LOW'`, mine.ID).Scan(&lowVal)

		// Overdue actions
		var overdueActions int
		_ = database.DB.QueryRow(`
			SELECT COUNT(*) FROM corrective_actions ca 
			JOIN violations v ON v.id = ca.violation_id 
			WHERE v.mine_id = $1 AND ca.status = 'OVERDUE'`, mine.ID).Scan(&overdueActions)

		// Operational Anomaly Check
		var hasProdAnomaly, hasAttAnomaly bool
		_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM anomalies WHERE mine_id=$1 AND anomaly_type='PRODUCTION' AND status='NEW')`, mine.ID).Scan(&hasProdAnomaly)
		_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM anomalies WHERE mine_id=$1 AND anomaly_type='ATTENDANCE' AND status='NEW')`, mine.ID).Scan(&hasAttAnomaly)

		// Environmental alerts count (past 7 days warning threshold)
        // Notice: DATE_SUB(NOW(), INTERVAL 7 DAY) changed to NOW() - INTERVAL '7 days' for Postgres
		var envAlerts int
		_ = database.DB.QueryRow(`
			SELECT COUNT(*) FROM environmental_data 
			WHERE mine_id = $1 
			  AND record_date >= NOW() - INTERVAL '7 days' 
			  AND (aqi > 150 OR water_quality_index < 65 OR dust_level > 200)`, mine.ID).Scan(&envAlerts)

		// Recurring violation in same category (>= 3 violations in past 30 days)
		var recurringCount int
		var recurringCat string
		_ = database.DB.QueryRow(`
			SELECT cc.name, COUNT(*) as cnt 
			FROM violations v 
			JOIN compliance_categories cc ON cc.id = v.category_id 
			WHERE v.mine_id = $1 AND v.created_at >= NOW() - INTERVAL '30 days' 
			GROUP BY cc.id 
			HAVING COUNT(*) >= 3 
			LIMIT 1`, mine.ID).Scan(&recurringCat, &recurringCount)

		stats := map[string]interface{}{
			"critical_violations":        critVal,
			"high_violations":            highVal,
			"medium_violations":          medVal,
			"low_violations":             lowVal,
			"overdue_actions":            overdueActions,
			"production_anomaly":         hasProdAnomaly,
			"attendance_anomaly":         hasAttAnomaly,
			"environmental_alerts":       envAlerts,
			"recurring_violations_count": recurringCount,
			"recurring_category":         recurringCat,
		}

		// JSON payload
		payload := map[string]interface{}{"stats": stats}
		jsonBytes, _ := json.Marshal(payload)

		// Call AI Flask service
		aiURL := fmt.Sprintf("%s/predict-risk", ac.Cfg.AIServiceURL)

		var score float64
		var classification string
		var factorsJSON []byte

		// Post HTTP request to python service
		client := &http.Client{Timeout: 2 * time.Second}
		resp, postErr := client.Post(aiURL, "application/json", bytes.NewBuffer(jsonBytes))

		if postErr == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)

			type aiResponse struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
				Data    struct {
					Score          float64       `json:"score"`
					Classification string        `json:"classification"`
					Factors        []interface{} `json:"factors"`
				} `json:"data"`
			}

			var aiRes aiResponse
			if err := json.Unmarshal(bodyBytes, &aiRes); err == nil && aiRes.Success {
				score = aiRes.Data.Score
				classification = aiRes.Data.Classification
				factorsJSON, _ = json.Marshal(aiRes.Data.Factors)
			}
		}

		// Fallback baseline calculation inside Go backend if AI Flask service is offline/unreachable
		if factorsJSON == nil {
			// Basic score
			rawScore := float64(critVal*25 + highVal*15 + medVal*8 + lowVal*3 + overdueActions*15)
			if hasProdAnomaly {
				rawScore += 12
			}
			if hasAttAnomaly {
				rawScore += 10
			}
			rawScore += float64(envAlerts * 10)
			if recurringCount > 0 {
				rawScore += 20
			}
			if rawScore > 100 {
				rawScore = 100
			}
			score = rawScore

			classification = "LOW"
			if score >= 81 {
				classification = "CRITICAL"
			} else if score >= 61 {
				classification = "HIGH"
			} else if score >= 31 {
				classification = "MEDIUM"
			}

			// Generate fallback factors
			fallbackFactors := []map[string]interface{}{}
			if critVal > 0 {
				fallbackFactors = append(fallbackFactors, map[string]interface{}{"name": strconv.Itoa(critVal) + " critical violations", "impact": critVal * 25})
			}
			if overdueActions > 0 {
				fallbackFactors = append(fallbackFactors, map[string]interface{}{"name": strconv.Itoa(overdueActions) + " overdue actions", "impact": overdueActions * 15})
			}
			if hasProdAnomaly {
				fallbackFactors = append(fallbackFactors, map[string]interface{}{"name": "Production anomaly drop", "impact": 12})
			}
			factorsJSON, _ = json.Marshal(fallbackFactors)
		}

		// Write to database (Notice: ?, ?, ?, ? changed to $1, $2, $3, $4)
		_, _ = database.DB.Exec(`
			INSERT INTO risk_scores (mine_id, score, classification, factors_json) 
			VALUES ($1, $2, $3, $4)`,
			mine.ID, score, classification, string(factorsJSON))
	}
	return nil
}

type recurringViolationItem struct {
	MineID           int    `json:"mine_id"`
	MineName         string `json:"mine_name"`
	CategoryID       int    `json:"category_id"`
	CategoryName     string `json:"category_name"`
	TotalViolations  int    `json:"total_violations"`
	CriticalCount    int    `json:"critical_count"`
	HighCount        int    `json:"high_count"`
	MediumCount      int    `json:"medium_count"`
	OpenCount        int    `json:"open_count"`
	FirstDetected    string `json:"first_detected"`
	LatestDetected   string `json:"latest_detected"`
	RepeatLevel      string `json:"repeat_level"` // MODERATE, CHRONIC, SEVERE
	RiskScorePenalty int    `json:"risk_score_penalty"`
}

// GetRecurringViolations analyzes violations grouped by mine and category/rule across a rolling time window.
func (ac *AnalyticsController) GetRecurringViolations(c *gin.Context) {
	windowDaysStr := c.DefaultQuery("window_days", "90")
	windowDays, _ := strconv.Atoi(windowDaysStr)
	if windowDays <= 0 {
		windowDays = 90
	}

	mineID := c.Query("mine_id")

    // Notice: DATE_SUB(NOW(), INTERVAL %d DAY) changed to NOW() - INTERVAL '%d days' for Postgres
	query := fmt.Sprintf(`
		SELECT v.mine_id, m.mine_name, v.category_id, cc.name AS category_name,
			   COUNT(*) AS total_violations,
			   SUM(CASE WHEN v.severity = 'CRITICAL' THEN 1 ELSE 0 END) AS crit_cnt,
			   SUM(CASE WHEN v.severity = 'HIGH' THEN 1 ELSE 0 END) AS high_cnt,
			   SUM(CASE WHEN v.severity = 'MEDIUM' THEN 1 ELSE 0 END) AS med_cnt,
			   SUM(CASE WHEN v.status IN ('OPEN', 'IN_PROGRESS', 'OVERDUE') THEN 1 ELSE 0 END) AS open_cnt,
			   MIN(v.created_at) AS first_date,
			   MAX(v.created_at) AS latest_date
		FROM violations v
		JOIN mines m ON m.id = v.mine_id
		JOIN compliance_categories cc ON cc.id = v.category_id
		WHERE v.created_at >= NOW() - INTERVAL '%d days'`, windowDays)

	args := []interface{}{}
	if mineID != "" {
		query += " AND v.mine_id = $1"
		args = append(args, mineID)
	}

	query += `
		GROUP BY v.mine_id, m.mine_name, v.category_id, cc.name
		HAVING COUNT(*) >= 2
		ORDER BY total_violations DESC, crit_cnt DESC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to analyze recurring violations", err.Error())
		return
	}
	defer rows.Close()

	list := []recurringViolationItem{}
	totalRepeatIncidents := 0
	chronicMineCount := make(map[int]bool)

	for rows.Next() {
		var it recurringViolationItem
		var firstDate, latestDate time.Time
		err := rows.Scan(
			&it.MineID, &it.MineName, &it.CategoryID, &it.CategoryName,
			&it.TotalViolations, &it.CriticalCount, &it.HighCount, &it.MediumCount, &it.OpenCount,
			&firstDate, &latestDate,
		)
		if err != nil {
			continue
		}

		it.FirstDetected = firstDate.Format("2006-01-02")
		it.LatestDetected = latestDate.Format("2006-01-02")

		// Determine severity level & AI risk penalty
		if it.TotalViolations >= 5 || it.CriticalCount >= 2 {
			it.RepeatLevel = "SEVERE"
			it.RiskScorePenalty = 30
		} else if it.TotalViolations >= 3 || it.CriticalCount >= 1 || it.HighCount >= 2 {
			it.RepeatLevel = "CHRONIC"
			it.RiskScorePenalty = 20
		} else {
			it.RepeatLevel = "MODERATE"
			it.RiskScorePenalty = 10
		}

		totalRepeatIncidents += it.TotalViolations
		chronicMineCount[it.MineID] = true
		list = append(list, it)
	}

	utils.Success(c, http.StatusOK, "Recurring violation patterns analyzed", gin.H{
		"window_days":            windowDays,
		"recurring_groups_count": len(list),
		"affected_mines_count":   len(chronicMineCount),
		"total_repeat_incidents": totalRepeatIncidents,
		"recurring_violations":   list,
	})
}