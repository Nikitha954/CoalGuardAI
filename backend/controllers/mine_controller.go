package controllers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/models"
	"coal-governance-backend/utils"
)

type MineController struct{}

func NewMineController() *MineController {
	return &MineController{}
}

// ListMines returns all mines, with optional filters (subsidiary_id, state, status).
// Every role can view mines; data is not restricted here because visibility
// scoping (e.g. a manager's own mine) is handled at the UI/query-filter level
// for this phase, and will be tightened further as modules are added.
func (mc *MineController) ListMines(c *gin.Context) {
	query := `
		SELECT m.id, m.mine_name, m.mine_code, m.subsidiary_id, s.name,
		       m.state, m.district, m.latitude, m.longitude, m.mine_type,
		       m.production_capacity, m.manager_id, COALESCE(u.full_name, ''),
		       m.status, m.created_at, m.updated_at
		FROM mines m
		JOIN subsidiaries s ON s.id = m.subsidiary_id
		LEFT JOIN users u ON u.id = m.manager_id
		WHERE 1=1`
	args := []interface{}{}

	if subID := c.Query("subsidiary_id"); subID != "" {
		query += " AND m.subsidiary_id = ?"
		args = append(args, subID)
	}
	if state := c.Query("state"); state != "" {
		query += " AND m.state = ?"
		args = append(args, state)
	}
	if status := c.Query("status"); status != "" {
		query += " AND m.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY m.mine_name ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch mines", err.Error())
		return
	}
	defer rows.Close()

	mines := []models.Mine{}
	for rows.Next() {
		var m models.Mine
		var managerID sql.NullInt64
		if err := rows.Scan(&m.ID, &m.MineName, &m.MineCode, &m.SubsidiaryID, &m.SubsidiaryName,
			&m.State, &m.District, &m.Latitude, &m.Longitude, &m.MineType,
			&m.ProductionCapacity, &managerID, &m.ManagerName,
			&m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse mine data", err.Error())
			return
		}
		if managerID.Valid {
			mid := int(managerID.Int64)
			m.ManagerID = &mid
		}

		// Attach open violation count for at-a-glance visibility.
		_ = database.DB.QueryRow(
			`SELECT COUNT(*) FROM violations WHERE mine_id = ? AND status IN ('OPEN','IN_PROGRESS','OVERDUE')`,
			m.ID,
		).Scan(&m.OpenViolationsCount)

		// Attach overdue corrective actions count
		_ = database.DB.QueryRow(
			`SELECT COUNT(*) FROM corrective_actions ca 
			 JOIN violations v ON v.id = ca.violation_id 
			 WHERE v.mine_id = ? AND ca.status = 'OVERDUE'`,
			m.ID,
		).Scan(&m.OverdueActionsCount)

		// Attach latest risk score & classification
		var riskScore sql.NullFloat64
		var riskClass sql.NullString
		_ = database.DB.QueryRow(
			`SELECT score, classification FROM risk_scores WHERE mine_id = ? ORDER BY computed_at DESC LIMIT 1`,
			m.ID,
		).Scan(&riskScore, &riskClass)
		if riskScore.Valid {
			val := riskScore.Float64
			m.RiskScore = &val
			m.RiskClassification = riskClass.String
		} else {
			// default to LOW risk if not computed yet
			m.RiskClassification = "LOW"
			val := 0.0
			m.RiskScore = &val
		}

		// Calculate compliance score as: (Passed checklist items / total graded items) * 100
		var complianceScore float64
		err := database.DB.QueryRow(`
			SELECT COALESCE(
				(SUM(CASE WHEN ii.result = 'PASS' THEN 1 ELSE 0 END) / 
				 SUM(CASE WHEN ii.result IN ('PASS', 'FAIL') THEN 1 ELSE 0 END)) * 100, 
				100.0
			)
			FROM inspection_items ii
			JOIN inspections i ON i.id = ii.inspection_id
			WHERE i.mine_id = ? AND i.status = 'APPROVED'`, m.ID).Scan(&complianceScore)
		if err == nil {
			m.ComplianceScore = &complianceScore
		}

		mines = append(mines, m)
	}

	utils.Success(c, http.StatusOK, "Mines fetched successfully", mines)
}

// GetMine returns a single mine by ID with its latest risk score if available.
func (mc *MineController) GetMine(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid mine ID", err.Error())
		return
	}

	var m models.Mine
	var managerID sql.NullInt64
	row := database.DB.QueryRow(`
		SELECT m.id, m.mine_name, m.mine_code, m.subsidiary_id, s.name,
		       m.state, m.district, m.latitude, m.longitude, m.mine_type,
		       m.production_capacity, m.manager_id, COALESCE(u.full_name, ''),
		       m.status, m.created_at, m.updated_at
		FROM mines m
		JOIN subsidiaries s ON s.id = m.subsidiary_id
		LEFT JOIN users u ON u.id = m.manager_id
		WHERE m.id = ?`, id)

	err = row.Scan(&m.ID, &m.MineName, &m.MineCode, &m.SubsidiaryID, &m.SubsidiaryName,
		&m.State, &m.District, &m.Latitude, &m.Longitude, &m.MineType,
		&m.ProductionCapacity, &managerID, &m.ManagerName,
		&m.Status, &m.CreatedAt, &m.UpdatedAt)

	if err == sql.ErrNoRows {
		utils.Fail(c, http.StatusNotFound, "Mine not found", "no mine with that ID")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch mine", err.Error())
		return
	}

	if managerID.Valid {
		mid := int(managerID.Int64)
		m.ManagerID = &mid
	}

	var riskScore sql.NullFloat64
	var riskClass sql.NullString
	_ = database.DB.QueryRow(
		`SELECT score, classification FROM risk_scores WHERE mine_id = ? ORDER BY computed_at DESC LIMIT 1`,
		m.ID,
	).Scan(&riskScore, &riskClass)
	if riskScore.Valid {
		m.RiskScore = &riskScore.Float64
		m.RiskClassification = riskClass.String
	}

	utils.Success(c, http.StatusOK, "Mine fetched successfully", m)
}

type mineRequest struct {
	MineName           string  `json:"mine_name" binding:"required"`
	MineCode           string  `json:"mine_code" binding:"required"`
	SubsidiaryID       int     `json:"subsidiary_id" binding:"required"`
	State              string  `json:"state"`
	District           string  `json:"district"`
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	MineType           string  `json:"mine_type"`
	ProductionCapacity float64 `json:"production_capacity"`
	ManagerID          *int    `json:"manager_id"`
	Status             string  `json:"status"`
}

// CreateMine — restricted to SUPER_ADMIN via RBAC middleware on the route.
func (mc *MineController) CreateMine(c *gin.Context) {
	var req mineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request", err.Error())
		return
	}
	if req.MineType == "" {
		req.MineType = "OPENCAST"
	}
	if req.Status == "" {
		req.Status = "ACTIVE"
	}

	userID, _ := c.Get(middleware.CtxUserID)

	var newID int64
	err := database.DB.QueryRow(`
		INSERT INTO mines (mine_name, mine_code, subsidiary_id, state, district, latitude, longitude,
		                    mine_type, production_capacity, manager_id, status, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		req.MineName, req.MineCode, req.SubsidiaryID, req.State, req.District, req.Latitude, req.Longitude,
		req.MineType, req.ProductionCapacity, req.ManagerID, req.Status, userID, userID).Scan(&newID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to create mine (check that mine_code is unique)", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "MINE_CREATED", "MINES", strconv.FormatInt(newID, 10),
		map[string]interface{}{"mine_name": req.MineName, "mine_code": req.MineCode}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Mine created successfully", gin.H{"id": newID})
}

// UpdateMine — restricted to SUPER_ADMIN via RBAC middleware on the route.
func (mc *MineController) UpdateMine(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid mine ID", err.Error())
		return
	}

	var req mineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	_, err = database.DB.Exec(`
		UPDATE mines SET mine_name=?, mine_code=?, subsidiary_id=?, state=?, district=?,
		       latitude=?, longitude=?, mine_type=?, production_capacity=?, manager_id=?,
		       status=?, updated_by=?
		WHERE id=?`,
		req.MineName, req.MineCode, req.SubsidiaryID, req.State, req.District,
		req.Latitude, req.Longitude, req.MineType, req.ProductionCapacity, req.ManagerID,
		req.Status, userID, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update mine", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "MINE_UPDATED", "MINES", strconv.Itoa(id),
		map[string]interface{}{"mine_name": req.MineName}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Mine updated successfully", nil)
}

// DeactivateMine performs a soft-delete (status change) rather than a hard
// delete, preserving historical inspection/violation records for audit integrity.
func (mc *MineController) DeactivateMine(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid mine ID", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	_, err = database.DB.Exec(`UPDATE mines SET status='INACTIVE', updated_by=? WHERE id=?`, userID, id)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to deactivate mine", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "MINE_DEACTIVATED", "MINES", strconv.Itoa(id), nil, c.ClientIP())
	utils.Success(c, http.StatusOK, "Mine deactivated successfully", nil)
}

// ListSubsidiaries is a helper endpoint used to populate dropdowns.
func (mc *MineController) ListSubsidiaries(c *gin.Context) {
	rows, err := database.DB.Query(`SELECT id, name, code, headquarters, status, created_at FROM subsidiaries ORDER BY name`)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch subsidiaries", err.Error())
		return
	}
	defer rows.Close()

	subs := []models.Subsidiary{}
	for rows.Next() {
		var s models.Subsidiary
		if err := rows.Scan(&s.ID, &s.Name, &s.Code, &s.Headquarters, &s.Status, &s.CreatedAt); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse subsidiary data", err.Error())
			return
		}
		subs = append(subs, s)
	}

	utils.Success(c, http.StatusOK, "Subsidiaries fetched successfully", subs)
}