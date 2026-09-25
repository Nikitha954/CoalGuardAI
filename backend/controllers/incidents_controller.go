package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/utils"
)

type IncidentsController struct{}

func NewIncidentsController() *IncidentsController {
	return &IncidentsController{}
}

// ListIncidents lists incidents with optional filters (mine_id, status, severity).
func (ic *IncidentsController) ListIncidents(c *gin.Context) {
	query := `
		SELECT i.id, i.mine_id, m.mine_name, i.incident_type, i.description, i.severity,
		       i.reported_by, u.full_name AS reported_by_name, i.incident_date, i.status, i.created_at
		FROM incidents i
		JOIN mines m ON m.id = i.mine_id
		JOIN users u ON u.id = i.reported_by
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
	if severity := c.Query("severity"); severity != "" {
		query += " AND i.severity = ?"
		args = append(args, severity)
	}

	query += " ORDER BY i.created_at DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch incidents", err.Error())
		return
	}
	defer rows.Close()

	type incidentItem struct {
		ID             int       `json:"id"`
		MineID         int       `json:"mine_id"`
		MineName       string    `json:"mine_name"`
		IncidentType   string    `json:"incident_type"`
		Description    string    `json:"description"`
		Severity       string    `json:"severity"`
		ReportedBy     int       `json:"reported_by"`
		ReportedByName string    `json:"reported_by_name"`
		IncidentDate   time.Time `json:"incident_date"`
		Status         string    `json:"status"`
		CreatedAt      time.Time `json:"created_at"`
	}

	list := []incidentItem{}
	for rows.Next() {
		var it incidentItem
		if err := rows.Scan(&it.ID, &it.MineID, &it.MineName, &it.IncidentType, &it.Description,
			&it.Severity, &it.ReportedBy, &it.ReportedByName, &it.IncidentDate, &it.Status, &it.CreatedAt); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse incidents", err.Error())
			return
		}
		list = append(list, it)
	}

	utils.Success(c, http.StatusOK, "Incidents fetched successfully", list)
}

type emergencyRequest struct {
	MineID      int    `json:"mine_id" binding:"required"`
	Description string `json:"description"`
}

// TriggerEmergency handles the Emergency SOS button.
// It immediately logs a CRITICAL incident (skipping the normal incident form)
// and fans out an in-app CRITICAL notification to every relevant responder
// (mine managers, safety officers, corporate/regulatory oversight, and admins).
func (ic *IncidentsController) TriggerEmergency(c *gin.Context) {
	var req emergencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	// Resolve reporter name + mine name for readable alert messages.
	var reporterName string
	if err := database.DB.QueryRow(`SELECT full_name FROM users WHERE id = ?`, userID).Scan(&reporterName); err != nil {
		reporterName = "A field user"
	}

	var mineName string
	if err := database.DB.QueryRow(`SELECT mine_name FROM mines WHERE id = ?`, req.MineID).Scan(&mineName); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid mine selected", "mine not found")
		return
	}

	description := req.Description
	if description == "" {
		description = fmt.Sprintf("Emergency SOS triggered by %s at %s. Immediate attention required.", reporterName, mineName)
	}

	now := time.Now()

	// 1. Log a CRITICAL incident record.
	var incidentID int64
	err := database.DB.QueryRow(`
		INSERT INTO incidents (mine_id, incident_type, description, severity, reported_by, incident_date, status)
		VALUES (?, 'EMERGENCY_SOS', ?, 'CRITICAL', ?, ?, 'OPEN')
		RETURNING id`,
		req.MineID, description, userID, now).Scan(&incidentID)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to log emergency incident", err.Error())
		return
	}

	// 2. Fan out CRITICAL in-app notifications to every responder except the
	//    person who triggered the alert.
	rows, err := database.DB.Query(`
		SELECT u.id FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.status = 'ACTIVE' AND u.id != ?
		  AND r.role_key IN ('SUPER_ADMIN','MINE_MANAGER','SAFETY_OFFICER','CORPORATE_MANAGER','REGULATORY_OFFICER')`,
		userID)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to resolve responders", err.Error())
		return
	}
	defer rows.Close()

	title := fmt.Sprintf("EMERGENCY SOS — %s", mineName)
	message := fmt.Sprintf("%s raised an emergency SOS at %s. %s", reporterName, mineName, description)

	notified := 0
	for rows.Next() {
		var recipientID int
		if err := rows.Scan(&recipientID); err != nil {
			continue
		}
		if _, err := database.DB.Exec(`
			INSERT INTO notifications (recipient_id, title, message, severity, type, is_read)
			VALUES (?, ?, ?, 'CRITICAL', 'EMERGENCY', FALSE)`,
			recipientID, title, message); err == nil {
			notified++
		}
	}

	utils.LogAudit(userID, "EMERGENCY_SOS_TRIGGERED", "INCIDENTS", fmt.Sprintf("%d", incidentID),
		map[string]interface{}{"mine_id": req.MineID, "mine_name": mineName, "notified_count": notified}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Emergency alert sent", gin.H{
		"incident_id":    incidentID,
		"notified_count": notified,
		"mine_name":      mineName,
		"triggered_at":   now,
	})
}

type createIncidentRequest struct {
	MineID       int    `json:"mine_id" binding:"required"`
	IncidentType string `json:"incident_type" binding:"required"`
	Description  string `json:"description" binding:"required"`
	Severity     string `json:"severity"`
	IncidentDate string `json:"incident_date"`
}

// CreateIncident logs a standard or offline-synced field incident report.
func (ic *IncidentsController) CreateIncident(c *gin.Context) {
	var req createIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	if req.Severity == "" {
		req.Severity = "MEDIUM"
	}

	var incDate time.Time
	if req.IncidentDate != "" {
		var err error
		incDate, err = time.Parse("2006-01-02 15:04:05", req.IncidentDate)
		if err != nil {
			incDate, _ = time.Parse("2006-01-02", req.IncidentDate)
		}
	}
	if incDate.IsZero() {
		incDate = time.Now()
	}

	var newID int64
	err := database.DB.QueryRow(`
		INSERT INTO incidents (mine_id, incident_type, description, severity, reported_by, incident_date, status)
		VALUES (?, ?, ?, ?, ?, ?, 'OPEN')
		RETURNING id`,
		req.MineID, req.IncidentType, req.Description, req.Severity, userID, incDate).Scan(&newID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to create incident report", err.Error())
		return
	}

	utils.LogAudit(userID, "INCIDENT_REPORTED", "INCIDENTS", fmt.Sprintf("%d", newID),
		map[string]interface{}{"mine_id": req.MineID, "type": req.IncidentType, "severity": req.Severity}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Incident report submitted successfully", gin.H{"id": newID})
}

// UpdateIncidentStatus updates the status of an incident report.
func (ic *IncidentsController) UpdateIncidentStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid incident ID", err.Error())
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"` // OPEN, UNDER_REVIEW, CLOSED
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid status", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	_, err = database.DB.Exec(`UPDATE incidents SET status = ? WHERE id = ?`, req.Status, id)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update incident status", err.Error())
		return
	}

	utils.LogAudit(userID, "INCIDENT_STATUS_UPDATED", "INCIDENTS", strconv.Itoa(id),
		map[string]interface{}{"status": req.Status}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Incident status updated successfully", nil)
}