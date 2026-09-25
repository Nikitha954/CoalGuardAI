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

type ContractorController struct{}

func NewContractorController() *ContractorController {
	return &ContractorController{}
}

// ListContractors returns all contractors with active worker counts and expiry countdowns.
func (cc *ContractorController) ListContractors(c *gin.Context) {
	// POSTGRES FIX: Replaced MySQL DATEDIFF() and CURDATE() with (date1 - CURRENT_DATE)
	query := `
		SELECT c.id, c.mine_id, m.mine_name, c.company_name, c.contact_person,
			   c.phone, c.email, c.contract_type, c.contract_start, c.contract_end,
			   c.status, COALESCE(c.blacklist_reason, ''), c.created_at,
			   (SELECT COUNT(*) FROM workers w WHERE w.contractor_id = c.id AND w.status = 'ACTIVE') AS worker_count,
			   (SELECT COUNT(*) FROM documents d WHERE d.contractor_id = c.id) AS document_count,
			   (c.contract_end - CURRENT_DATE) AS days_until_expiry
		FROM contractors c
		JOIN mines m ON m.id = c.mine_id
		WHERE 1=1`
	args := []interface{}{}

	if mineID := c.Query("mine_id"); mineID != "" {
		query += " AND c.mine_id = ?"
		args = append(args, mineID)
	}
	if status := c.Query("status"); status != "" {
		query += " AND c.status = ?"
		args = append(args, status)
	}
	if search := c.Query("search"); search != "" {
		query += " AND (c.company_name LIKE ? OR c.contact_person LIKE ?)"
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern)
	}

	query += " ORDER BY c.company_name ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch contractors", err.Error())
		return
	}
	defer rows.Close()

	contractors := []models.Contractor{}
	for rows.Next() {
		var ct models.Contractor
		var cStart, cEnd []uint8
		var daysLeft sql.NullInt64

		err := rows.Scan(
			&ct.ID, &ct.MineID, &ct.MineName, &ct.CompanyName, &ct.ContactPerson,
			&ct.Phone, &ct.Email, &ct.ContractType, &cStart, &cEnd,
			&ct.Status, &ct.BlacklistReason, &ct.CreatedAt,
			&ct.WorkerCount, &ct.DocumentCount, &daysLeft,
		)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse contractor data", err.Error())
			return
		}

		if cStart != nil {
			ct.ContractStart = string(cStart)
		}
		if cEnd != nil {
			ct.ContractEnd = string(cEnd)
		}
		if daysLeft.Valid {
			ct.DaysUntilExpiry = int(daysLeft.Int64)
		}

		contractors = append(contractors, ct)
	}

	utils.Success(c, http.StatusOK, "Contractors fetched successfully", contractors)
}

type contractorRequest struct {
	MineID        int    `json:"mine_id" binding:"required"`
	CompanyName   string `json:"company_name" binding:"required"`
	ContactPerson string `json:"contact_person"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	ContractType  string `json:"contract_type"`
	ContractStart string `json:"contract_start"` // YYYY-MM-DD
	ContractEnd   string `json:"contract_end"`   // YYYY-MM-DD
	Status        string `json:"status"`
}

// CreateContractor creates a new contractor profile.
func (cc *ContractorController) CreateContractor(c *gin.Context) {
	var req contractorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Status == "" {
		req.Status = "ACTIVE"
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	var newID int64
	err := database.DB.QueryRow(`
		INSERT INTO contractors (mine_id, company_name, contact_person, phone, email, contract_type, contract_start, contract_end, status) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		req.MineID, req.CompanyName, req.ContactPerson, req.Phone, req.Email,
		req.ContractType, req.ContractStart, req.ContractEnd, req.Status).Scan(&newID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to create contractor", err.Error())
		return
	}

	utils.LogAudit(userID, "CONTRACTOR_CREATED", "CONTRACTORS", strconv.FormatInt(newID, 10),
		map[string]interface{}{"company_name": req.CompanyName, "mine_id": req.MineID}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Contractor created successfully", gin.H{"id": newID})
}

// UpdateContractor updates contractor details.
func (cc *ContractorController) UpdateContractor(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid contractor ID", err.Error())
		return
	}

	var req contractorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	_, err = database.DB.Exec(`
		UPDATE contractors 
		SET mine_id = ?, company_name = ?, contact_person = ?, phone = ?, email = ?, 
			contract_type = ?, contract_start = ?, contract_end = ?, status = ?
		WHERE id = ?`,
		req.MineID, req.CompanyName, req.ContactPerson, req.Phone, req.Email,
		req.ContractType, req.ContractStart, req.ContractEnd, req.Status, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update contractor", err.Error())
		return
	}

	utils.LogAudit(userID, "CONTRACTOR_UPDATED", "CONTRACTORS", strconv.Itoa(id),
		map[string]interface{}{"company_name": req.CompanyName}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Contractor updated successfully", nil)
}

// BlacklistContractor puts a contractor into BLACKLISTED status with mandatory reason.
func (cc *ContractorController) BlacklistContractor(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid contractor ID", err.Error())
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Reason == "" {
		utils.Fail(c, http.StatusBadRequest, "A detailed blacklist reason is required", "reason missing")
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	var companyName string
	var mineID int
	_ = database.DB.QueryRow(`SELECT company_name, mine_id FROM contractors WHERE id = ?`, id).Scan(&companyName, &mineID)

	_, err = database.DB.Exec(`
		UPDATE contractors 
		SET status = 'BLACKLISTED', blacklist_reason = ? 
		WHERE id = ?`, req.Reason, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to blacklist contractor", err.Error())
		return
	}

	// Deactivate linked worker accounts
	_, _ = database.DB.Exec(`UPDATE workers SET status = 'INACTIVE' WHERE contractor_id = ?`, id)

	// Broadcast alert to Safety Officers and Super Admins
	title := fmt.Sprintf("Contractor Blacklisted: %s", companyName)
	message := fmt.Sprintf("Contractor %s has been blacklisted. Reason: %s", companyName, req.Reason)
	rows, _ := database.DB.Query(`
		SELECT u.id FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.status = 'ACTIVE' AND r.role_key IN ('SAFETY_OFFICER', 'MINE_MANAGER', 'SUPER_ADMIN', 'CORPORATE_MANAGER')`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var recID int
			if err := rows.Scan(&recID); err == nil {
				_, _ = database.DB.Exec(`
					INSERT INTO notifications (recipient_id, title, message, severity, type, is_read)
					VALUES (?, ?, ?, 'CRITICAL', 'CONTRACTOR_BLACKLIST', FALSE)`,
					recID, title, message)
			}
		}
	}

	utils.LogAudit(userID, "CONTRACTOR_BLACKLISTED", "CONTRACTORS", strconv.Itoa(id),
		map[string]interface{}{"company_name": companyName, "reason": req.Reason}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Contractor blacklisted successfully and alerts broadcast", nil)
}

// CheckContractExpiries runs a check for contracts expiring soon (<= 30 days) or already expired,
// generating notifications for oversight personnel.
func (cc *ContractorController) CheckContractExpiries(c *gin.Context) {
	// POSTGRES FIX: Replaced MySQL DATEDIFF() and DATE_ADD() with Postgres equivalents
	rows, err := database.DB.Query(`
		SELECT c.id, c.company_name, c.mine_id, m.mine_name, c.contract_end, (c.contract_end - CURRENT_DATE) AS days_left
		FROM contractors c
		JOIN mines m ON m.id = c.mine_id
		WHERE c.status = 'ACTIVE' AND c.contract_end <= CURRENT_DATE + INTERVAL '30 days'`)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query contract expiries", err.Error())
		return
	}
	defer rows.Close()

	type expiryAlert struct {
		ContractorID int    `json:"contractor_id"`
		CompanyName  string `json:"company_name"`
		MineName     string `json:"mine_name"`
		ContractEnd  string `json:"contract_end"`
		DaysLeft     int    `json:"days_left"`
	}

	alerts := []expiryAlert{}
	for rows.Next() {
		var a expiryAlert
		var cEnd []uint8
		if err := rows.Scan(&a.ContractorID, &a.CompanyName, &a.ContractorID, &a.MineName, &cEnd, &a.DaysLeft); err == nil {
			a.ContractEnd = string(cEnd)
			alerts = append(alerts, a)

			// Generate notifications
			severity := "WARNING"
			if a.DaysLeft <= 7 {
				severity = "CRITICAL"
			}

			title := fmt.Sprintf("Contract Expiry Warning: %s", a.CompanyName)
			message := fmt.Sprintf("Contract for %s at %s expires in %d day(s) (End Date: %s). Please review renewal/safety certifications.",
				a.CompanyName, a.MineName, a.DaysLeft, a.ContractEnd)

			notifRows, _ := database.DB.Query(`
				SELECT u.id FROM users u
				JOIN roles r ON r.id = u.role_id
				WHERE u.status = 'ACTIVE' AND r.role_key IN ('MINE_MANAGER', 'SAFETY_OFFICER', 'SUPER_ADMIN')`)
			if notifRows != nil {
				for notifRows.Next() {
					var uid int
					if err := notifRows.Scan(&uid); err == nil {
						_, _ = database.DB.Exec(`
							INSERT INTO notifications (recipient_id, title, message, severity, type, is_read)
							VALUES (?, ?, ?, ?, 'CONTRACT_EXPIRY', FALSE)`,
							uid, title, message, severity)
					}
				}
				notifRows.Close()
			}
		}
	}

	utils.Success(c, http.StatusOK, "Contract expiry check completed", gin.H{
		"checked_at":    time.Now(),
		"alerts_count":  len(alerts),
		"expiry_alerts": alerts,
	})
}