package controllers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/models"
	"coal-governance-backend/utils"
)

type ViolationsController struct{}

func NewViolationsController() *ViolationsController {
	return &ViolationsController{}
}

// ListViolations lists violations with optional filters.
func (vc *ViolationsController) ListViolations(c *gin.Context) {
	query := `
		SELECT v.id, v.violation_code, v.mine_id, m.mine_name, v.inspection_id, v.category_id, cc.name AS category_name,
		       v.description, v.severity, v.evidence_path, v.reported_by, u1.full_name AS reported_by_name,
		       v.responsible_person, u2.full_name AS responsible_name, v.deadline, v.status, v.created_at
		FROM violations v
		JOIN mines m ON m.id = v.mine_id
		JOIN compliance_categories cc ON cc.id = v.category_id
		JOIN users u1 ON u1.id = v.reported_by
		LEFT JOIN users u2 ON u2.id = v.responsible_person
		WHERE 1=1`
	args := []interface{}{}

	if mineID := c.Query("mine_id"); mineID != "" {
		query += " AND v.mine_id = ?"
		args = append(args, mineID)
	}
	if status := c.Query("status"); status != "" {
		query += " AND v.status = ?"
		args = append(args, status)
	}
	if severity := c.Query("severity"); severity != "" {
		query += " AND v.severity = ?"
		args = append(args, severity)
	}
	if categoryID := c.Query("category_id"); categoryID != "" {
		query += " AND v.category_id = ?"
		args = append(args, categoryID)
	}

	query += " ORDER BY v.created_at DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch violations", err.Error())
		return
	}
	defer rows.Close()

	violations := []models.Violation{}
	for rows.Next() {
		var v models.Violation
		var deadlineVal []uint8
		var insID sql.NullInt64
		var respID sql.NullInt64
		var respName sql.NullString
		var evPath sql.NullString

		err := rows.Scan(&v.ID, &v.ViolationCode, &v.MineID, &v.MineName, &insID, &v.CategoryID, &v.CategoryName,
			&v.Description, &v.Severity, &evPath, &v.ReportedBy, &v.ReportedByName,
			&respID, &respName, &deadlineVal, &v.Status, &v.CreatedAt)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse violations", err.Error())
			return
		}

		if deadlineVal != nil {
			v.Deadline = string(deadlineVal)
		}
		if insID.Valid {
			id := int(insID.Int64)
			v.InspectionID = &id
		}
		if respID.Valid {
			id := int(respID.Int64)
			v.ResponsiblePerson = &id
		}
		if respName.Valid {
			v.ResponsibleName = respName.String
		}
		if evPath.Valid {
			v.EvidencePath = evPath.String
		}

		violations = append(violations, v)
	}

	utils.Success(c, http.StatusOK, "Violations fetched successfully", violations)
}

// GetViolation gets details of a single violation.
func (vc *ViolationsController) GetViolation(c *gin.Context) {
	id := c.Param("id")

	var v models.Violation
	var deadlineVal []uint8
	var insID sql.NullInt64
	var respID sql.NullInt64
	var respName sql.NullString
	var evPath sql.NullString

	err := database.DB.QueryRow(`
		SELECT v.id, v.violation_code, v.mine_id, m.mine_name, v.inspection_id, v.category_id, cc.name AS category_name,
		       v.description, v.severity, v.evidence_path, v.reported_by, u1.full_name AS reported_by_name,
		       v.responsible_person, u2.full_name AS responsible_name, v.deadline, v.status, v.created_at
		FROM violations v
		JOIN mines m ON m.id = v.mine_id
		JOIN compliance_categories cc ON cc.id = v.category_id
		JOIN users u1 ON u1.id = v.reported_by
		LEFT JOIN users u2 ON u2.id = v.responsible_person
		WHERE v.id = ?`, id).Scan(&v.ID, &v.ViolationCode, &v.MineID, &v.MineName, &insID, &v.CategoryID, &v.CategoryName,
		&v.Description, &v.Severity, &evPath, &v.ReportedBy, &v.ReportedByName,
		&respID, &respName, &deadlineVal, &v.Status, &v.CreatedAt)

	if err == sql.ErrNoRows {
		utils.Fail(c, http.StatusNotFound, "Violation not found", "not found")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Database query failed", err.Error())
		return
	}

	if deadlineVal != nil {
		v.Deadline = string(deadlineVal)
	}
	if insID.Valid {
		id := int(insID.Int64)
		v.InspectionID = &id
	}
	if respID.Valid {
		id := int(respID.Int64)
		v.ResponsiblePerson = &id
	}
	if respName.Valid {
		v.ResponsibleName = respName.String
	}
	if evPath.Valid {
		v.EvidencePath = evPath.String
	}

	utils.Success(c, http.StatusOK, "Violation details fetched", v)
}

type manualViolationRequest struct {
	MineID      int    `json:"mine_id" binding:"required"`
	CategoryID  int    `json:"category_id" binding:"required"`
	Description string `json:"description" binding:"required"`
	Severity    string `json:"severity" binding:"required"` // LOW, MEDIUM, HIGH, CRITICAL
	Deadline    string `json:"deadline"`                   // YYYY-MM-DD
}

// CreateViolation handles direct manual logging of a compliance breach.
func (vc *ViolationsController) CreateViolation(c *gin.Context) {
	var req manualViolationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	// Determine violation code
	year := time.Now().Format("2006")
	var seq int
	_ = database.DB.QueryRow(`SELECT COUNT(*) + 1 FROM violations`).Scan(&seq)
	violationCode := fmt.Sprintf("VIO-%s-%04d", year, seq)

	// Handle default deadline if not provided
	if req.Deadline == "" {
		now := time.Now()
		var deadline time.Time
		switch req.Severity {
		case "CRITICAL":
			deadline = now.AddDate(0, 0, 1)
		case "HIGH":
			deadline = now.AddDate(0, 0, 3)
		case "MEDIUM":
			deadline = now.AddDate(0, 0, 7)
		default:
			deadline = now.AddDate(0, 0, 14)
		}
		req.Deadline = deadline.Format("2006-01-02")
	}

	var newID int64
	err := database.DB.QueryRow(`
		INSERT INTO violations (violation_code, mine_id, category_id, description, severity, reported_by, deadline, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'OPEN')
		RETURNING id`,
		violationCode, req.MineID, req.CategoryID, req.Description, req.Severity, userID, req.Deadline).Scan(&newID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to record manual violation", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "VIOLATION_MANUALLY_CREATED", "VIOLATIONS", violationCode,
		map[string]interface{}{"mine_id": req.MineID, "severity": req.Severity}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Violation reported successfully", gin.H{"id": newID, "violation_code": violationCode})
}

type updateViolationRequest struct {
	ResponsiblePerson *int   `json:"responsible_person"`
	Deadline          string `json:"deadline"`
	Status            string `json:"status"` // OPEN, IN_PROGRESS, RESOLVED, VERIFIED, CLOSED, OVERDUE
}

// UpdateViolation handles updating assignee, deadline, or status.
func (vc *ViolationsController) UpdateViolation(c *gin.Context) {
	id := c.Param("id")
	var req updateViolationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	// Fetch current violation record
	var currentStatus string
	var currentCode string
	err := database.DB.QueryRow(`SELECT status, violation_code FROM violations WHERE id = ?`, id).Scan(&currentStatus, &currentCode)
	if err == sql.ErrNoRows {
		utils.Fail(c, http.StatusNotFound, "Violation not found", "not found")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Database query failed", err.Error())
		return
	}

	// Build update statement dynamically
	query := "UPDATE violations SET updated_at = NOW()"
	args := []interface{}{}

	if req.ResponsiblePerson != nil {
		query += ", responsible_person = ?"
		args = append(args, *req.ResponsiblePerson)
		// If OPEN and assigning someone, transition to IN_PROGRESS
		if req.Status == "" && currentStatus == "OPEN" {
			req.Status = "IN_PROGRESS"
		}
	}
	if req.Deadline != "" {
		query += ", deadline = ?"
		args = append(args, req.Deadline)
	}
	if req.Status != "" {
		query += ", status = ?"
		args = append(args, req.Status)
	}

	query += " WHERE id = ?"
	args = append(args, id)

	_, err = database.DB.Exec(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update violation", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "VIOLATION_UPDATED", "VIOLATIONS", currentCode,
		map[string]interface{}{"status": req.Status, "responsible_person": req.ResponsiblePerson}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Violation updated successfully", nil)
}