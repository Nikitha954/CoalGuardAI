package controllers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/models"
	"coal-governance-backend/utils"
)

type GrievanceController struct{}

func NewGrievanceController() *GrievanceController {
	return &GrievanceController{}
}

type submitGrievanceRequest struct {
	WorkerID    *int   `json:"worker_id"`
	MineID      int    `json:"mine_id" binding:"required"`
	Category    string `json:"category" binding:"required"` // Safety, Compensation, Working Conditions, Harassment, Equipment, Other
	Description string `json:"description" binding:"required"`
}

// SubmitGrievance records a new grievance from worker or field personnel.
func (gc *GrievanceController) SubmitGrievance(c *gin.Context) {
	var req submitGrievanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid grievance request", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	var newID int64
	err := database.DB.QueryRow(`
		INSERT INTO grievances (worker_id, mine_id, category, description, status)
		VALUES (?, ?, ?, ?, 'SUBMITTED')
		RETURNING id`,
		req.WorkerID, req.MineID, req.Category, req.Description).Scan(&newID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to submit grievance", err.Error())
		return
	}

	// Notify Safety Officer / Mine Manager
	var mineName string
	_ = database.DB.QueryRow(`SELECT mine_name FROM mines WHERE id = ?`, req.MineID).Scan(&mineName)

	title := fmt.Sprintf("New Grievance Submitted — %s", mineName)
	message := fmt.Sprintf("Grievance #%d (%s) reported: %s", newID, req.Category, req.Description)

	// Fan out notification to Mine Manager and Safety Officer
	rows, _ := database.DB.Query(`
		SELECT u.id FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.status = 'ACTIVE' AND r.role_key IN ('MINE_MANAGER', 'SAFETY_OFFICER', 'SUPER_ADMIN')`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var recID int
			if err := rows.Scan(&recID); err == nil {
				_, _ = database.DB.Exec(`
					INSERT INTO notifications (recipient_id, title, message, severity, type, is_read)
					VALUES (?, ?, ?, 'WARNING', 'GRIEVANCE', FALSE)`,
					recID, title, message)
			}
		}
	}

	utils.LogAudit(userID, "GRIEVANCE_SUBMITTED", "GRIEVANCES", strconv.FormatInt(newID, 10),
		map[string]interface{}{"mine_id": req.MineID, "category": req.Category, "worker_id": req.WorkerID}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Grievance submitted successfully", gin.H{"id": newID})
}

// ListGrievances fetches grievances with optional filters.
func (gc *GrievanceController) ListGrievances(c *gin.Context) {
	query := `
		SELECT g.id, g.worker_id, COALESCE(w.full_name, 'Anonymous Worker'), COALESCE(w.worker_code, ''),
		       g.mine_id, m.mine_name, g.category, g.description, g.status,
		       g.assigned_to, COALESCE(u.full_name, 'Unassigned'),
		       COALESCE(g.resolution_notes, ''), g.created_at, g.updated_at
		FROM grievances g
		JOIN mines m ON m.id = g.mine_id
		LEFT JOIN workers w ON w.id = g.worker_id
		LEFT JOIN users u ON u.id = g.assigned_to
		WHERE 1=1`
	args := []interface{}{}

	if mineID := c.Query("mine_id"); mineID != "" {
		query += " AND g.mine_id = ?"
		args = append(args, mineID)
	}
	if status := c.Query("status"); status != "" {
		query += " AND g.status = ?"
		args = append(args, status)
	}
	if category := c.Query("category"); category != "" {
		query += " AND g.category = ?"
		args = append(args, category)
	}
	if workerID := c.Query("worker_id"); workerID != "" {
		query += " AND g.worker_id = ?"
		args = append(args, workerID)
	}

	query += " ORDER BY g.created_at DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query grievances", err.Error())
		return
	}
	defer rows.Close()

	list := []models.Grievance{}
	for rows.Next() {
		var g models.Grievance
		var workerID, assignedTo sql.NullInt64
		err := rows.Scan(
			&g.ID, &workerID, &g.WorkerName, &g.WorkerCode,
			&g.MineID, &g.MineName, &g.Category, &g.Description, &g.Status,
			&assignedTo, &g.AssignedToName,
			&g.ResolutionNotes, &g.CreatedAt, &g.UpdatedAt,
		)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse grievance data", err.Error())
			return
		}
		if workerID.Valid {
			wid := int(workerID.Int64)
			g.WorkerID = &wid
		}
		if assignedTo.Valid {
			aid := int(assignedTo.Int64)
			g.AssignedTo = &aid
		}
		list = append(list, g)
	}

	utils.Success(c, http.StatusOK, "Grievances fetched successfully", list)
}

// GetGrievance returns a single grievance by ID.
func (gc *GrievanceController) GetGrievance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid grievance ID", err.Error())
		return
	}

	var g models.Grievance
	var workerID, assignedTo sql.NullInt64

	row := database.DB.QueryRow(`
		SELECT g.id, g.worker_id, COALESCE(w.full_name, 'Anonymous Worker'), COALESCE(w.worker_code, ''),
		       g.mine_id, m.mine_name, g.category, g.description, g.status,
		       g.assigned_to, COALESCE(u.full_name, 'Unassigned'),
		       COALESCE(g.resolution_notes, ''), g.created_at, g.updated_at
		FROM grievances g
		JOIN mines m ON m.id = g.mine_id
		LEFT JOIN workers w ON w.id = g.worker_id
		LEFT JOIN users u ON u.id = g.assigned_to
		WHERE g.id = ?`, id)

	err = row.Scan(
		&g.ID, &workerID, &g.WorkerName, &g.WorkerCode,
		&g.MineID, &g.MineName, &g.Category, &g.Description, &g.Status,
		&assignedTo, &g.AssignedToName,
		&g.ResolutionNotes, &g.CreatedAt, &g.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		utils.Fail(c, http.StatusNotFound, "Grievance not found", "No record with ID")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch grievance", err.Error())
		return
	}

	if workerID.Valid {
		wid := int(workerID.Int64)
		g.WorkerID = &wid
	}
	if assignedTo.Valid {
		aid := int(assignedTo.Int64)
		g.AssignedTo = &aid
	}

	utils.Success(c, http.StatusOK, "Grievance fetched successfully", g)
}

// AssignGrievance assigns an officer to investigate/resolve the grievance.
func (gc *GrievanceController) AssignGrievance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid grievance ID", err.Error())
		return
	}

	var req struct {
		AssignedTo int `json:"assigned_to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid assignment request", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	_, err = database.DB.Exec(`
		UPDATE grievances 
		SET assigned_to = ?, status = 'IN_REVIEW', updated_at = NOW()
		WHERE id = ?`, req.AssignedTo, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to assign grievance", err.Error())
		return
	}

	// Notify assignee
	_, _ = database.DB.Exec(`
		INSERT INTO notifications (recipient_id, title, message, severity, type, is_read)
		VALUES (?, 'Grievance Assigned To You', CONCAT('You have been assigned to review Grievance #', ?), 'INFO', 'GRIEVANCE', FALSE)`,
		req.AssignedTo, id)

	utils.LogAudit(userID, "GRIEVANCE_ASSIGNED", "GRIEVANCES", strconv.Itoa(id),
		map[string]interface{}{"assigned_to": req.AssignedTo}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Grievance assigned successfully", nil)
}

// ResolveGrievance marks a grievance as RESOLVED with resolution notes.
func (gc *GrievanceController) ResolveGrievance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid grievance ID", err.Error())
		return
	}

	var req struct {
		ResolutionNotes string `json:"resolution_notes" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Resolution notes are required", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	_, err = database.DB.Exec(`
		UPDATE grievances 
		SET resolution_notes = ?, status = 'RESOLVED', updated_at = NOW()
		WHERE id = ?`, req.ResolutionNotes, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to resolve grievance", err.Error())
		return
	}

	utils.LogAudit(userID, "GRIEVANCE_RESOLVED", "GRIEVANCES", strconv.Itoa(id),
		map[string]interface{}{"resolution_notes": req.ResolutionNotes}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Grievance marked as resolved", nil)
}

// EscalateGrievance escalates a grievance to higher authority/corporate level.
func (gc *GrievanceController) EscalateGrievance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid grievance ID", err.Error())
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	notes := fmt.Sprintf("[ESCALATED]: %s", req.Reason)
	_, err = database.DB.Exec(`
		UPDATE grievances 
		SET status = 'ESCALATED', 
		    resolution_notes = CASE WHEN resolution_notes IS NULL OR resolution_notes = '' THEN ? ELSE CONCAT(resolution_notes, '\n', ?) END,
		    updated_at = NOW()
		WHERE id = ?`, notes, notes, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to escalate grievance", err.Error())
		return
	}

	// Notify Corporate Managers & Super Admin
	rows, _ := database.DB.Query(`
		SELECT u.id FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.status = 'ACTIVE' AND r.role_key IN ('CORPORATE_MANAGER', 'SUPER_ADMIN')`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var recID int
			if err := rows.Scan(&recID); err == nil {
				_, _ = database.DB.Exec(`
					INSERT INTO notifications (recipient_id, title, message, severity, type, is_read)
					VALUES (?, 'ESCALATED Grievance Alert', CONCAT('Grievance #', ?, ' has been escalated for executive intervention: ', ?), 'CRITICAL', 'GRIEVANCE', FALSE)`,
					recID, id, req.Reason)
			}
		}
	}

	utils.LogAudit(userID, "GRIEVANCE_ESCALATED", "GRIEVANCES", strconv.Itoa(id),
		map[string]interface{}{"escalation_reason": req.Reason}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Grievance escalated successfully", nil)
}