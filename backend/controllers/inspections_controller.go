package controllers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/config"
	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/models"
	"coal-governance-backend/utils"
)

type InspectionsController struct {
	Cfg *config.Config
}

func NewInspectionsController(cfg *config.Config) *InspectionsController {
	return &InspectionsController{Cfg: cfg}
}

// ListInspections lists inspections with filters.
func (ic *InspectionsController) ListInspections(c *gin.Context) {
	query := `
		SELECT i.id, i.mine_id, m.mine_name, i.inspection_type, i.inspector_id, u.full_name,
			   i.inspection_date, i.inspection_time, i.gps_latitude, i.gps_longitude, i.remarks, i.status, i.created_at
		FROM inspections i
		JOIN mines m ON m.id = i.mine_id
		JOIN users u ON u.id = i.inspector_id
		WHERE 1=1`
	args := []interface{}{}

	if mineID := c.Query("mine_id"); mineID != "" {
		query += " AND i.mine_id = ?"
		args = append(args, mineID)
	}
	if status := c.Query("status"); status != "" {
		query += " AND i.status = ?"
		args = append(args, status)
	}
	if inspectorID := c.Query("inspector_id"); inspectorID != "" {
		query += " AND i.inspector_id = ?"
		args = append(args, inspectorID)
	}

	query += " ORDER BY i.inspection_date DESC, i.inspection_time DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch inspections", err.Error())
		return
	}
	defer rows.Close()

	inspections := []models.Inspection{}
	for rows.Next() {
		var i models.Inspection
		var dateVal, timeVal []uint8
		var createdAtVal time.Time

		err := rows.Scan(&i.ID, &i.MineID, &i.MineName, &i.InspectionType, &i.InspectorID, &i.InspectorName,
			&dateVal, &timeVal, &i.GPSLatitude, &i.GPSLongitude, &i.Remarks, &i.Status, &createdAtVal)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse inspections", err.Error())
			return
		}
		i.InspectionDate = string(dateVal)
		i.InspectionTime = string(timeVal)
		i.CreatedAt = createdAtVal
		inspections = append(inspections, i)
	}

	utils.Success(c, http.StatusOK, "Inspections fetched successfully", inspections)
}

// GetInspection gets a specific inspection with checklist items and observations.
func (ic *InspectionsController) GetInspection(c *gin.Context) {
	id := c.Param("id")

	var i models.Inspection
	var dateVal, timeVal []uint8

	err := database.DB.QueryRow(`
		SELECT i.id, i.mine_id, m.mine_name, i.inspection_type, i.inspector_id, u.full_name,
			   i.inspection_date, i.inspection_time, i.gps_latitude, i.gps_longitude, i.remarks, i.status, i.created_at
		FROM inspections i
		JOIN mines m ON m.id = i.mine_id
		JOIN users u ON u.id = i.inspector_id
		WHERE i.id = ?`, id).Scan(&i.ID, &i.MineID, &i.MineName, &i.InspectionType, &i.InspectorID, &i.InspectorName,
		&dateVal, &timeVal, &i.GPSLatitude, &i.GPSLongitude, &i.Remarks, &i.Status, &i.CreatedAt)

	if err == sql.ErrNoRows {
		utils.Fail(c, http.StatusNotFound, "Inspection not found", "not found")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch inspection", err.Error())
		return
	}
	i.InspectionDate = string(dateVal)
	i.InspectionTime = string(timeVal)

	// Fetch Checklist Items
	itemRows, err := database.DB.Query(`SELECT id, checklist_item, result, remarks FROM inspection_items WHERE inspection_id = ?`, id)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch inspection items", err.Error())
		return
	}
	defer itemRows.Close()

	i.Items = []models.InspectionItem{}
	for itemRows.Next() {
		var it models.InspectionItem
		it.InspectionID = i.ID
		if err := itemRows.Scan(&it.ID, &it.ChecklistItem, &it.Result, &it.Remarks); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse item", err.Error())
			return
		}
		i.Items = append(i.Items, it)
	}

	// Fetch Observations
	obsRows, err := database.DB.Query(`
		SELECT o.id, o.category_id, c.name, o.observation, o.severity, o.evidence_path, o.created_at
		FROM observations o
		LEFT JOIN compliance_categories c ON c.id = o.category_id
		WHERE o.inspection_id = ?`, id)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch observations", err.Error())
		return
	}
	defer obsRows.Close()

	type detailedObs struct {
		ID           int       `json:"id"`
		CategoryID   *int      `json:"category_id"`
		CategoryName string    `json:"category_name"`
		Observation  string    `json:"observation"`
		Severity     string    `json:"severity"`
		EvidencePath string    `json:"evidence_path"`
		CreatedAt    time.Time `json:"created_at"`
	}

	observations := []detailedObs{}
	for obsRows.Next() {
		var o detailedObs
		var catID sql.NullInt64
		var catName sql.NullString
		if err := obsRows.Scan(&o.ID, &catID, &catName, &o.Observation, &o.Severity, &o.EvidencePath, &o.CreatedAt); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse observation", err.Error())
			return
		}
		if catID.Valid {
			val := int(catID.Int64)
			o.CategoryID = &val
		}
		if catName.Valid {
			o.CategoryName = catName.String
		}
		observations = append(observations, o)
	}

	// Fetch AI Analysis
	var aiAnalysis models.AIInspectionAnalysis
	var aiConf float64
	var aiCreated time.Time
	err = database.DB.QueryRow(`
		SELECT id, category, severity, risk_level, risk_score, summary, reasoning, recommended_action, recurring_issue, urgency, confidence, model_name, created_at
		FROM ai_inspection_analyses
		WHERE inspection_id = ?`, id).Scan(
		&aiAnalysis.ID, &aiAnalysis.Category, &aiAnalysis.Severity, &aiAnalysis.RiskLevel, &aiAnalysis.RiskScore,
		&aiAnalysis.Summary, &aiAnalysis.Reasoning, &aiAnalysis.RecommendedAction, &aiAnalysis.RecurringIssue,
		&aiAnalysis.Urgency, &aiConf, &aiAnalysis.ModelName, &aiCreated)

	var aiVal interface{} = nil
	if err == nil {
		aiAnalysis.InspectionID, _ = strconv.Atoi(id)
		aiAnalysis.Confidence = aiConf
		aiAnalysis.CreatedAt = aiCreated
		aiVal = aiAnalysis
	}

	utils.Success(c, http.StatusOK, "Inspection details fetched", gin.H{
		"inspection":   i,
		"observations": observations,
		"ai_analysis":  aiVal,
	})
}

// CreateInspection handles multi-part submission of inspections and observations.
func (ic *InspectionsController) CreateInspection(c *gin.Context) {
	userID, _ := c.Get(middleware.CtxUserID)

	mineIDStr := c.PostForm("mine_id")
	inspectionType := c.PostForm("inspection_type")
	gpsLatStr := c.PostForm("gps_latitude")
	gpsLngStr := c.PostForm("gps_longitude")
	remarks := c.PostForm("remarks")
	status := c.PostForm("status") // DRAFT or SUBMITTED
	checklistJSON := c.PostForm("checklist")

	if mineIDStr == "" || inspectionType == "" || status == "" {
		utils.Fail(c, http.StatusBadRequest, "Missing required fields", "mine_id, inspection_type, and status are required")
		return
	}

	mineID, _ := strconv.Atoi(mineIDStr)
	gpsLat, _ := strconv.ParseFloat(gpsLatStr, 64)
	gpsLng, _ := strconv.ParseFloat(gpsLngStr, 64)

	if status != "DRAFT" && status != "SUBMITTED" {
		status = "DRAFT"
	}

	tx, err := database.DB.Begin()
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to start transaction", err.Error())
		return
	}
	defer tx.Rollback()

	now := time.Now()
	inspectionDate := now.Format("2006-01-02")
	inspectionTime := now.Format("15:04:05")

	var inspectionID int
	// WRAPPED WITH database.Rebind()
	err = tx.QueryRow(database.Rebind(`
		INSERT INTO inspections (mine_id, inspection_type, inspector_id, inspection_date, inspection_time, gps_latitude, gps_longitude, remarks, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`),
		mineID, inspectionType, userID, inspectionDate, inspectionTime, gpsLat, gpsLng, remarks, status).Scan(&inspectionID)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to create inspection record", err.Error())
		return
	}

	// Save Checklist Items
	if checklistJSON != "" {
		var items []struct {
			ChecklistItem string `json:"checklist_item"`
			Result        string `json:"result"` // PASS, FAIL, NA
			Remarks       string `json:"remarks"`
		}
		if err := json.Unmarshal([]byte(checklistJSON), &items); err == nil {
			for _, item := range items {
				// WRAPPED WITH database.Rebind()
				_, err = tx.Exec(database.Rebind(`
					INSERT INTO inspection_items (inspection_id, checklist_item, result, remarks)
					VALUES (?, ?, ?, ?)`),
					inspectionID, item.ChecklistItem, item.Result, item.Remarks)
				if err != nil {
					utils.Fail(c, http.StatusInternalServerError, "Failed to save checklist item", err.Error())
					return
				}
			}
		}
	}

	// Handle Optional Observation and Evidence Image Upload
	obsText := c.PostForm("observation")
	if obsText != "" {
		obsCategoryStr := c.PostForm("observation_category_id")
		obsSeverity := c.PostForm("observation_severity")
		if obsSeverity == "" {
			obsSeverity = "LOW"
		}

		var catIDVal interface{} = nil
		if obsCategoryStr != "" {
			if cid, err := strconv.Atoi(obsCategoryStr); err == nil {
				catIDVal = cid
			}
		}

		evidencePath := ""
		file, header, err := c.Request.FormFile("evidence")
		if err == nil {
			defer file.Close()
			// Generate filename
			ext := filepath.Ext(header.Filename)
			filename := fmt.Sprintf("evidence_%d_%d%s", inspectionID, time.Now().Unix(), ext)
			uploadDir := "./uploads"
			_ = os.MkdirAll(uploadDir, os.ModePerm)
			evidencePath = filepath.Join(uploadDir, filename)

			out, err := os.Create(evidencePath)
			if err != nil {
				utils.Fail(c, http.StatusInternalServerError, "Failed to save upload file", err.Error())
				return
			}
			defer out.Close()
			_, _ = io.Copy(out, file)
			// Normalize path for web/cross-platform
			evidencePath = filepath.ToSlash(evidencePath)
		}

		// WRAPPED WITH database.Rebind()
		_, err = tx.Exec(database.Rebind(`
			INSERT INTO observations (inspection_id, category_id, observation, severity, evidence_path)
			VALUES (?, ?, ?, ?, ?)`),
			inspectionID, catIDVal, obsText, obsSeverity, evidencePath)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to save observation", err.Error())
			return
		}
	}

	if err := tx.Commit(); err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to commit inspection", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "INSPECTION_CREATED", "INSPECTIONS", strconv.Itoa(inspectionID),
		map[string]interface{}{"status": status, "mine_id": mineID}, c.ClientIP())

	// If status is submitted, we immediately run check for auto violations
	if status == "SUBMITTED" {
		autoGenerateViolations(inspectionID)
	}

	utils.Success(c, http.StatusCreated, "Inspection created successfully", gin.H{"id": inspectionID})
}

// UpdateInspectionStatus manages the inspection workflow transitions.
func (ic *InspectionsController) UpdateInspectionStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required"` // SUBMITTED, REVIEWED, APPROVED
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid status payload", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	// Fetch current status
	var currentStatus string
	err := database.DB.QueryRow(`SELECT status FROM inspections WHERE id = ?`, id).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		utils.Fail(c, http.StatusNotFound, "Inspection not found", "not found")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Database query failed", err.Error())
		return
	}

	// Validate state transitions
	valid := false
	if currentStatus == "DRAFT" && req.Status == "SUBMITTED" {
		valid = true
	} else if currentStatus == "SUBMITTED" && req.Status == "REVIEWED" {
		valid = true
	} else if currentStatus == "REVIEWED" && req.Status == "APPROVED" {
		valid = true
	} else if currentStatus == "SUBMITTED" && req.Status == "APPROVED" {
		// Shortcuts allowed for demo convenience
		valid = true
	}

	if !valid {
		utils.Fail(c, http.StatusBadRequest, fmt.Sprintf("Invalid workflow transition from %s to %s", currentStatus, req.Status), "invalid transition")
		return
	}

	_, err = database.DB.Exec(`UPDATE inspections SET status = ? WHERE id = ?`, req.Status, id)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update inspection status", err.Error())
		return
	}

	insID, _ := strconv.Atoi(id)
	utils.LogAudit(userID.(int), "INSPECTION_STATUS_UPDATED", "INSPECTIONS", id, map[string]interface{}{"status": req.Status}, c.ClientIP())

	// Trigger auto violation generation if now submitted or approved
	if req.Status == "SUBMITTED" || req.Status == "APPROVED" {
		autoGenerateViolations(insID)
	}

	utils.Success(c, http.StatusOK, "Inspection status updated successfully", nil)
}

// autoGenerateViolations automatically parses failed checklist items and observations to raise violations.
func autoGenerateViolations(inspectionID int) {
	// Find mine_id and inspector_id
	var mineID, inspectorID int
	err := database.DB.QueryRow(`SELECT mine_id, inspector_id FROM inspections WHERE id = ?`, inspectionID).Scan(&mineID, &inspectorID)
	if err != nil {
		return
	}

	// 1. Process Failed Checklist Items
	rows, err := database.DB.Query(`SELECT checklist_item, remarks FROM inspection_items WHERE inspection_id = ? AND result = 'FAIL'`, inspectionID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var checklistItem, remarks string
			if err := rows.Scan(&checklistItem, &remarks); err == nil {
				createAutoViolation(mineID, inspectionID, checklistItem, remarks, inspectorID)
			}
		}
	}

	// 2. Process Observations
	obsRows, err := database.DB.Query(`SELECT category_id, observation, severity, evidence_path FROM observations WHERE inspection_id = ?`, inspectionID)
	if err == nil {
		defer obsRows.Close()
		for obsRows.Next() {
			var catID sql.NullInt64
			var observation, severity, evidencePath string
			if err := obsRows.Scan(&catID, &observation, &severity, &evidencePath); err == nil {
				categoryID := 1 // Default to Safety category if null
				if catID.Valid {
					categoryID = int(catID.Int64)
				}
				createViolationRecord(mineID, inspectionID, categoryID, observation, severity, evidencePath, inspectorID)
			}
		}
	}
}

func createAutoViolation(mineID, inspectionID int, checklistItem, remarks string, inspectorID int) {
	// Let's identify the category of this checklist item.
	// Since checklist items are free text, we can categorize by keyword matching, defaulting to Category = 1 (Safety).
	categoryID := 1 // Safety
	severity := "HIGH"
	description := fmt.Sprintf("Failed checklist item: %s. Remarks: %s", checklistItem, remarks)

	createViolationRecord(mineID, inspectionID, categoryID, description, severity, "", inspectorID)
}

func createViolationRecord(mineID, inspectionID int, categoryID int, description, severity, evidencePath string, inspectorID int) {
	// Check if already created to prevent duplicate violations
	var exists bool
	_ = database.DB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM violations WHERE inspection_id = ? AND description = ?)`,
		inspectionID, description).Scan(&exists)
	if exists {
		return
	}

	// Generate violation code: VIO-<year>-<random/seq>
	year := time.Now().Format("2006")
	var seq int
	_ = database.DB.QueryRow(`SELECT COUNT(*) + 1 FROM violations`).Scan(&seq)
	violationCode := fmt.Sprintf("VIO-%s-%04d", year, seq)

	// Determine deadline based on severity
	now := time.Now()
	var deadline time.Time
	switch severity {
	case "CRITICAL":
		deadline = now.AddDate(0, 0, 1) // 1 day
	case "HIGH":
		deadline = now.AddDate(0, 0, 3) // 3 days
	case "MEDIUM":
		deadline = now.AddDate(0, 0, 7) // 7 days
	default:
		deadline = now.AddDate(0, 0, 14) // 14 days
	}

	_, err := database.DB.Exec(`
		INSERT INTO violations (violation_code, mine_id, inspection_id, category_id, description, severity, evidence_path, reported_by, deadline, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'OPEN')`,
		violationCode, mineID, inspectionID, categoryID, description, severity, evidencePath, inspectorID, deadline.Format("2006-01-02"))
	if err != nil {
		fmt.Printf("Error auto-creating violation: %v\n", err)
	} else {
		// Log the audit event for compliance tracking
		utils.LogAudit(inspectorID, "VIOLATION_AUTO_CREATED", "VIOLATIONS", violationCode,
			map[string]interface{}{"mine_id": mineID, "severity": severity}, "127.0.0.1")
	}
}

// AnalyzeInspectionDraft runs Gemini AI analysis on raw, unsaved inspection draft inputs.
func (ic *InspectionsController) AnalyzeInspectionDraft(c *gin.Context) {
	var req struct {
		MineID         int    `json:"mine_id" binding:"required"`
		InspectionType string `json:"inspection_type" binding:"required"`
		Observation    string `json:"observation" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	var mineName string
	err := database.DB.QueryRow(`SELECT mine_name FROM mines WHERE id = ?`, req.MineID).Scan(&mineName)
	if err != nil {
		mineName = "Unknown Mine"
	}

	payload := map[string]interface{}{
		"observation":     req.Observation,
		"mine_name":       mineName,
		"inspection_type": req.InspectionType,
	}

	analysis, err := callAIServiceAnalyze(ic.Cfg.AIServiceURL, payload)
	if err != nil {
		fmt.Printf("[Gemini AI] Proxy error: %v. Using fallback.\n", err)
		analysis = getFallbackAIAnalysis("Unavailable – AI service could not be reached")
	}

	utils.Success(c, http.StatusOK, "Draft inspection analyzed", analysis)
}

// AnalyzeInspection triggers, executes, and saves Gemini AI analysis on an existing inspection.
func (ic *InspectionsController) AnalyzeInspection(c *gin.Context) {
	idStr := c.Param("id")
	inspectionID, err := strconv.Atoi(idStr)
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid inspection ID", err.Error())
		return
	}

	// Fetch inspection remarks, type, and mine name
	var remarks, inspectionType, mineName string
	err = database.DB.QueryRow(`
		SELECT i.remarks, i.inspection_type, m.mine_name 
		FROM inspections i 
		JOIN mines m ON m.id = i.mine_id 
		WHERE i.id = ?`, inspectionID).Scan(&remarks, &inspectionType, &mineName)
	if err == sql.ErrNoRows {
		utils.Fail(c, http.StatusNotFound, "Inspection not found", "not found")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Database error", err.Error())
		return
	}

	// Fetch observations text if any (we prefer qualitative observation text over general remarks)
	var obsText string
	_ = database.DB.QueryRow(`SELECT observation FROM observations WHERE inspection_id = ? LIMIT 1`, inspectionID).Scan(&obsText)

	textToAnalyze := obsText
	if textToAnalyze == "" {
		textToAnalyze = remarks
	}

	if textToAnalyze == "" {
		utils.Fail(c, http.StatusBadRequest, "No observation or remarks text found to analyze", "empty text")
		return
	}

	payload := map[string]interface{}{
		"observation":     textToAnalyze,
		"mine_name":       mineName,
		"inspection_type": inspectionType,
	}

	analysis, err := callAIServiceAnalyze(ic.Cfg.AIServiceURL, payload)
	if err != nil {
		fmt.Printf("[Gemini AI] Proxy error: %v. Using fallback.\n", err)
		analysis = getFallbackAIAnalysis("Unavailable – AI service could not be reached")
	}

	// Save or overwrite to db
	_, _ = database.DB.Exec(`DELETE FROM ai_inspection_analyses WHERE inspection_id = ?`, inspectionID)

	category := analysis["category"].(string)
	severity := analysis["severity"].(string)
	riskLevel := analysis["risk_level"].(string)

	// Convert risk score safely
	var riskScore int
	switch scoreVal := analysis["risk_score"].(type) {
	case float64:
		riskScore = int(scoreVal)
	case int:
		riskScore = scoreVal
	default:
		riskScore = 50
	}

	summary := analysis["summary"].(string)
	reasoning := analysis["reasoning"].(string)
	recAction := analysis["recommended_action"].(string)
	recurringIssue := analysis["recurring_issue"].(bool)
	urgency := analysis["urgency"].(string)

	// Convert confidence safely
	var confidence float64
	switch confVal := analysis["confidence"].(type) {
	case float64:
		confidence = confVal
	case int:
		confidence = float64(confVal)
	default:
		confidence = 0.0
	}

	modelName := "gemini-2.5-flash"
	if mName, ok := analysis["model_name"].(string); ok {
		modelName = mName
	}

	_, err = database.DB.Exec(`
		INSERT INTO ai_inspection_analyses (
			inspection_id, category, severity, risk_level, risk_score, 
			summary, reasoning, recommended_action, recurring_issue, urgency, confidence, model_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inspectionID, category, severity, riskLevel, riskScore,
		summary, reasoning, recAction, recurringIssue, urgency, confidence, modelName)

	if err != nil {
		fmt.Printf("Error saving AI analysis to db: %v\n", err)
	}

	userID, _ := c.Get(middleware.CtxUserID)
	utils.LogAudit(userID.(int), "INSPECTION_AI_ANALYZED", "INSPECTIONS", idStr, nil, c.ClientIP())

	utils.Success(c, http.StatusOK, "Inspection analyzed and cached", analysis)
}

func callAIServiceAnalyze(aiServiceURL string, payload map[string]interface{}) (map[string]interface{}, error) {
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(aiServiceURL+"/ai/analyze-inspection", "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI Service returned status %d", resp.StatusCode)
	}

	var res struct {
		Success  bool                   `json:"success"`
		Analysis map[string]interface{} `json:"analysis"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if !res.Success {
		return nil, fmt.Errorf("AI Service failed analysis")
	}

	return res.Analysis, nil
}

func getFallbackAIAnalysis(statusMsg string) map[string]interface{} {
	return map[string]interface{}{
		"category":           "Compliance",
		"severity":           "MEDIUM",
		"risk_level":         "MEDIUM",
		"risk_score":         50,
		"summary":            statusMsg,
		"reasoning":          "The automated AI analysis was bypassed or timed out because the external Gemini AI service is currently offline or unconfigured. Deterministic rule evaluation remains fully operational.",
		"recommended_action": "Review the observation manually and log standard corrective action plans.",
		"recurring_issue":    false,
		"urgency":            "NEEDS_ATTENTION",
		"confidence":         0.0,
		"model_name":         "gemini-2.5-flash (fallback)",
	}
}
