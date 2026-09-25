package controllers

import (
	"database/sql"
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

type CorrectiveActionsController struct{}

func NewCorrectiveActionsController() *CorrectiveActionsController {
	return &CorrectiveActionsController{}
}

// ListCorrectiveActions fetches corrective actions, running the check-overdue logic first.
func (cac *CorrectiveActionsController) ListCorrectiveActions(c *gin.Context) {
	// First run the escalation and status check so that listings are always up to date
	runEscalationCheck()

	query := `
		SELECT ca.id, ca.violation_id, v.violation_code, v.description AS violation_desc, m.mine_name, v.severity,
		       ca.assigned_to, u1.full_name AS assigned_to_name, ca.action_description, ca.deadline,
		       ca.submitted_at, ca.verified_by, u2.full_name AS verified_by_name, ca.verified_at,
		       ca.escalation_level, ca.status, ca.created_at
		FROM corrective_actions ca
		JOIN violations v ON v.id = ca.violation_id
		JOIN mines m ON m.id = v.mine_id
		JOIN users u1 ON u1.id = ca.assigned_to
		LEFT JOIN users u2 ON u2.id = ca.verified_by
		WHERE 1=1`
	args := []interface{}{}

	if assignedTo := c.Query("assigned_to"); assignedTo != "" {
		query += " AND ca.assigned_to = ?"
		args = append(args, assignedTo)
	}
	if status := c.Query("status"); status != "" {
		query += " AND ca.status = ?"
		args = append(args, status)
	}
	if violationID := c.Query("violation_id"); violationID != "" {
		query += " AND ca.violation_id = ?"
		args = append(args, violationID)
	}

	query += " ORDER BY ca.created_at DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch corrective actions", err.Error())
		return
	}
	defer rows.Close()

	actions := []models.CorrectiveAction{}
	for rows.Next() {
		var ca models.CorrectiveAction
		var deadlineVal []uint8
		var subAt, verAt sql.NullTime
		var verBy sql.NullInt64
		var verByName sql.NullString

		err := rows.Scan(&ca.ID, &ca.ViolationID, &ca.ViolationCode, &ca.ViolationDesc, &ca.MineName, &ca.Severity,
			&ca.AssignedTo, &ca.AssignedToName, &ca.ActionDescription, &deadlineVal,
			&subAt, &verBy, &verByName, &verAt,
			&ca.EscalationLevel, &ca.Status, &ca.CreatedAt)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse corrective action", err.Error())
			return
		}

		if deadlineVal != nil {
			ca.Deadline = string(deadlineVal)
		}
		if subAt.Valid {
			ca.SubmittedAt = &subAt.Time
		}
		if verAt.Valid {
			ca.VerifiedAt = &verAt.Time
		}
		if verBy.Valid {
			id := int(verBy.Int64)
			ca.VerifiedBy = &id
		}
		if verByName.Valid {
			ca.VerifiedByName = verByName.String
		}

		actions = append(actions, ca)
	}

	utils.Success(c, http.StatusOK, "Corrective actions fetched successfully", actions)
}

type correctiveActionRequest struct {
	ViolationID       int    `json:"violation_id" binding:"required"`
	AssignedTo        int    `json:"assigned_to" binding:"required"`
	ActionDescription string `json:"action_description" binding:"required"`
	Deadline          string `json:"deadline" binding:"required"` // YYYY-MM-DD
}

// CreateCorrectiveAction assigns a corrective action for an open violation.
func (cac *CorrectiveActionsController) CreateCorrectiveAction(c *gin.Context) {
	var req correctiveActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	tx, err := database.DB.Begin()
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to start transaction", err.Error())
		return
	}
	defer tx.Rollback()

	// 1. Create corrective action record
	var newID int64
	err = tx.QueryRow(`
		INSERT INTO corrective_actions (violation_id, assigned_to, action_description, deadline, status)
		VALUES (?, ?, ?, ?, 'ASSIGNED')
		RETURNING id`,
		req.ViolationID, req.AssignedTo, req.ActionDescription, req.Deadline).Scan(&newID)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to create corrective action", err.Error())
		return
	}

	// 2. Update violation status to IN_PROGRESS and link assignee
	_, err = tx.Exec(`
		UPDATE violations SET status = 'IN_PROGRESS', responsible_person = ?, deadline = ? WHERE id = ?`,
		req.AssignedTo, req.Deadline, req.ViolationID)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update violation link", err.Error())
		return
	}

	// Fetch violation code for audit log
	var violationCode string
	_ = tx.QueryRow(`SELECT violation_code FROM violations WHERE id = ?`, req.ViolationID).Scan(&violationCode)

	if err := tx.Commit(); err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to commit assignment", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "CORRECTIVE_ACTION_ASSIGNED", "CORRECTIVE_ACTIONS", strconv.FormatInt(newID, 10),
		map[string]interface{}{"violation_code": violationCode, "assigned_to": req.AssignedTo}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Corrective action assigned successfully", gin.H{"id": newID})
}

// SubmitAction handles a worker submitting a completed corrective action.
func (cac *CorrectiveActionsController) SubmitAction(c *gin.Context) {
	id := c.Param("id")

	userID, _ := c.Get(middleware.CtxUserID)

	// Fetch corrective action and check assignment
	var assignedTo, violationID int
	var currentStatus string
	err := database.DB.QueryRow(`SELECT assigned_to, violation_id, status FROM corrective_actions WHERE id = ?`, id).Scan(&assignedTo, &violationID, &currentStatus)
	if err == sql.ErrNoRows {
		utils.Fail(c, http.StatusNotFound, "Corrective action not found", "not found")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Query failed", err.Error())
		return
	}

	// Verify that the submitter is the assigned worker
	if assignedTo != userID.(int) {
		utils.Fail(c, http.StatusForbidden, "You are not assigned to this corrective action", "forbidden")
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to start transaction", err.Error())
		return
	}
	defer tx.Rollback()

	// Update status to SUBMITTED
	_, err = tx.Exec(`
		UPDATE corrective_actions SET status = 'SUBMITTED', submitted_at = NOW() WHERE id = ?`, id)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update corrective action status", err.Error())
		return
	}

	// Update violation to RESOLVED (pending verification)
	_, err = tx.Exec(`
		UPDATE violations SET status = 'RESOLVED' WHERE id = ?`, violationID)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update violation status", err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to commit submission", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "CORRECTIVE_ACTION_SUBMITTED", "CORRECTIVE_ACTIONS", id, nil, c.ClientIP())

	utils.Success(c, http.StatusOK, "Corrective action submitted for review", nil)
}

// VerifyAction handles supervisor review (Verify and Close).
func (cac *CorrectiveActionsController) VerifyAction(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Approved bool   `json:"approved"` // true to close, false to reject back to ASSIGNED
		Remarks  string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	// Fetch corrective action details
	var violationID int
	var currentStatus string
	err := database.DB.QueryRow(`SELECT violation_id, status FROM corrective_actions WHERE id = ?`, id).Scan(&violationID, &currentStatus)
	if err == sql.ErrNoRows {
		utils.Fail(c, http.StatusNotFound, "Corrective action not found", "not found")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Query failed", err.Error())
		return
	}

	if currentStatus != "SUBMITTED" {
		utils.Fail(c, http.StatusBadRequest, "Corrective action is not submitted for review", "invalid state")
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to start transaction", err.Error())
		return
	}
	defer tx.Rollback()

	if req.Approved {
		// Close the corrective action
		_, err = tx.Exec(`
			UPDATE corrective_actions SET status = 'CLOSED', verified_by = ?, verified_at = NOW() WHERE id = ?`,
			userID, id)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to close corrective action", err.Error())
			return
		}

		// Close the violation
		_, err = tx.Exec(`UPDATE violations SET status = 'CLOSED' WHERE id = ?`, violationID)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to close violation", err.Error())
			return
		}

		utils.LogAudit(userID.(int), "CORRECTIVE_ACTION_VERIFIED_CLOSED", "CORRECTIVE_ACTIONS", id, nil, c.ClientIP())
	} else {
		// Reject it back to ASSIGNED status
		_, err = tx.Exec(`
			UPDATE corrective_actions SET status = 'ASSIGNED', submitted_at = NULL WHERE id = ?`, id)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to reject corrective action", err.Error())
			return
		}

		// Set violation back to IN_PROGRESS
		_, err = tx.Exec(`UPDATE violations SET status = 'IN_PROGRESS' WHERE id = ?`, violationID)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to reset violation status", err.Error())
			return
		}

		utils.LogAudit(userID.(int), "CORRECTIVE_ACTION_REJECTED", "CORRECTIVE_ACTIONS", id, map[string]interface{}{"remarks": req.Remarks}, c.ClientIP())
	}

	if err := tx.Commit(); err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to commit verification", err.Error())
		return
	}

	utils.Success(c, http.StatusOK, "Verification processed successfully", nil)
}

// TriggerEscalationCheck is an endpoint to manually trigger a deadline check.
func (cac *CorrectiveActionsController) TriggerEscalationCheck(c *gin.Context) {
	runEscalationCheck()
	utils.Success(c, http.StatusOK, "Escalation check completed", nil)
}

// runEscalationCheck scans for overdue actions and increments escalation levels and raises alerts.
func runEscalationCheck() {
	// Find all ASSIGNED or OVERDUE corrective actions that have exceeded their deadline
	now := time.Now().Format("2006-01-02")
	rows, err := database.DB.Query(`
		SELECT ca.id, ca.violation_id, v.violation_code, v.severity, ca.deadline, ca.escalation_level, ca.status
		FROM corrective_actions ca
		JOIN violations v ON v.id = ca.violation_id
		WHERE (ca.status = 'ASSIGNED' OR ca.status = 'OVERDUE')
		  AND ca.deadline < ?`, now)
	if err != nil {
		fmt.Printf("Error querying overdue corrective actions: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, violationID, escalationLevel int
		var violationCode, severity, deadline, status string

		err = rows.Scan(&id, &violationID, &violationCode, &severity, &deadline, &escalationLevel, &status)
		if err != nil {
			continue
		}

		// Mark status as OVERDUE
		if status != "OVERDUE" {
			_, _ = database.DB.Exec(`UPDATE corrective_actions SET status = 'OVERDUE' WHERE id = ?`, id)
			_, _ = database.DB.Exec(`UPDATE violations SET status = 'OVERDUE' WHERE id = ?`, violationID)
			utils.LogAudit(0, "CORRECTIVE_ACTION_OVERDUE", "CORRECTIVE_ACTIONS", strconv.Itoa(id), map[string]interface{}{"violation_code": violationCode}, "127.0.0.1")
		}

		// For HIGH/CRITICAL severity, escalate based on how overdue it is.
		if severity == "HIGH" || severity == "CRITICAL" {
			dlTime, err := time.Parse("2006-01-02", deadline)
			if err == nil {
				daysOverdue := int(time.Since(dlTime).Hours() / 24)

				newEscLevel := 0
				if daysOverdue >= 5 {
					newEscLevel = 3 // Corporate Manager
				} else if daysOverdue >= 3 {
					newEscLevel = 2 // Mine Manager
				} else if daysOverdue >= 1 {
					newEscLevel = 1 // Safety Officer
				}

				if newEscLevel > escalationLevel {
					_, _ = database.DB.Exec(`UPDATE corrective_actions SET escalation_level = ? WHERE id = ?`, newEscLevel, id)
					utils.LogAudit(0, "CORRECTIVE_ACTION_ESCALATED", "CORRECTIVE_ACTIONS", strconv.Itoa(id),
						map[string]interface{}{
							"violation_code":   violationCode,
							"escalation_level": newEscLevel,
							"days_overdue":     daysOverdue,
						}, "127.0.0.1")

					// Generate automated alerts (in-app notifications)
					generateEscalationNotifications(violationID, violationCode, severity, newEscLevel)
				}
			}
		}
	}
}

// generateEscalationNotifications inserts alerts into notifications table.
func generateEscalationNotifications(violationID int, violationCode, severity string, level int) {
	// Determine target role for notification
	var roleKey string
	switch level {
	case 1:
		roleKey = "SAFETY_OFFICER"
	case 2:
		roleKey = "MINE_MANAGER"
	case 3:
		roleKey = "CORPORATE_MANAGER"
	default:
		return
	}

	// Fetch users holding this role
	rows, err := database.DB.Query(`
		SELECT u.id FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE r.role_key = ? AND u.status = 'ACTIVE'`, roleKey)
	if err != nil {
		return
	}
	defer rows.Close()

	title := fmt.Sprintf("ESCALATION LEVEL %d: Overdue %s Violation", level, severity)
	message := fmt.Sprintf("Violation %s is overdue and has been escalated to Level %d. Immediate review required.", violationCode, level)

	for rows.Next() {
		var uid int
		if err := rows.Scan(&uid); err == nil {
			_, _ = database.DB.Exec(`
				INSERT INTO notifications (recipient_id, title, message, severity, type, is_read)
				VALUES (?, ?, ?, 'CRITICAL', 'ESCALATION', FALSE)`,
				uid, title, message)
		}
	}
}