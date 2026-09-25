package controllers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/utils"
)

type ComplianceController struct{}

func NewComplianceController() *ComplianceController {
	return &ComplianceController{}
}

// ListRules returns all compliance rules with their category name joined in.
func (cc *ComplianceController) ListRules(c *gin.Context) {
	query := `
		SELECT cr.id, cr.rule_code, cr.title, COALESCE(cr.description, ''), cc.id AS category_id, cc.name AS category,
		       cr.applicable_mine_id, cr.frequency, cr.severity, COALESCE(cr.responsible_dept, ''), cr.due_period_days, cr.status
		FROM compliance_rules cr
		JOIN compliance_categories cc ON cc.id = cr.category_id
		WHERE 1=1`
	args := []interface{}{}

	if category := c.Query("category"); category != "" {
		query += " AND cc.name = ?"
		args = append(args, category)
	}
	if severity := c.Query("severity"); severity != "" {
		query += " AND cr.severity = ?"
		args = append(args, severity)
	}
	if status := c.Query("status"); status != "" {
		query += " AND cr.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY cr.rule_code ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch compliance rules", err.Error())
		return
	}
	defer rows.Close()

	type rule struct {
		ID               int    `json:"id"`
		RuleCode         string `json:"rule_code"`
		Title            string `json:"title"`
		Description      string `json:"description"`
		CategoryID       int    `json:"category_id"`
		Category         string `json:"category"`
		ApplicableMineID *int   `json:"applicable_mine_id"`
		Frequency        string `json:"frequency"`
		Severity         string `json:"severity"`
		ResponsibleDept  string `json:"responsible_dept"`
		DuePeriodDays    int    `json:"due_period_days"`
		Status           string `json:"status"`
	}

	rules := []rule{}
	for rows.Next() {
		var r rule
		var applicableMine sql.NullInt64
		if err := rows.Scan(&r.ID, &r.RuleCode, &r.Title, &r.Description, &r.CategoryID, &r.Category,
			&applicableMine, &r.Frequency, &r.Severity, &r.ResponsibleDept, &r.DuePeriodDays, &r.Status); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse compliance rule", err.Error())
			return
		}
		if applicableMine.Valid {
			v := int(applicableMine.Int64)
			r.ApplicableMineID = &v
		}
		rules = append(rules, r)
	}

	utils.Success(c, http.StatusOK, "Compliance rules fetched successfully", rules)
}

type ruleRequest struct {
	RuleCode        string `json:"rule_code" binding:"required"`
	Title           string `json:"title" binding:"required"`
	Description     string `json:"description"`
	CategoryID      int    `json:"category_id" binding:"required"`
	ApplicableMine  *int   `json:"applicable_mine_id"`
	Frequency       string `json:"frequency"`
	Severity        string `json:"severity"`
	ResponsibleDept string `json:"responsible_dept"`
	DuePeriodDays   int    `json:"due_period_days"`
	Status          string `json:"status"`
}

// CreateRule — restricted to SUPER_ADMIN via RBAC middleware on the route.
func (cc *ComplianceController) CreateRule(c *gin.Context) {
	var req ruleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request", err.Error())
		return
	}
	if req.Frequency == "" {
		req.Frequency = "MONTHLY"
	}
	if req.Severity == "" {
		req.Severity = "MEDIUM"
	}
	if req.DuePeriodDays == 0 {
		req.DuePeriodDays = 30
	}
	if req.Status == "" {
		req.Status = "ACTIVE"
	}

	userID, _ := c.Get(middleware.CtxUserID)

	var newID int64
	err := database.DB.QueryRow(`
		INSERT INTO compliance_rules (rule_code, title, description, category_id, applicable_mine_id,
		                               frequency, severity, responsible_dept, due_period_days, status, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		req.RuleCode, req.Title, req.Description, req.CategoryID, req.ApplicableMine,
		req.Frequency, req.Severity, req.ResponsibleDept, req.DuePeriodDays, req.Status, userID, userID).Scan(&newID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to create rule (check that rule_code is unique)", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "COMPLIANCE_RULE_CREATED", "COMPLIANCE", strconv.FormatInt(newID, 10),
		map[string]interface{}{"rule_code": req.RuleCode, "title": req.Title}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Compliance rule created successfully", gin.H{"id": newID})
}

// UpdateRule — restricted to SUPER_ADMIN via RBAC middleware on the route.
func (cc *ComplianceController) UpdateRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid compliance rule ID", err.Error())
		return
	}

	var req ruleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request", err.Error())
		return
	}
	if req.Frequency == "" {
		req.Frequency = "MONTHLY"
	}
	if req.Severity == "" {
		req.Severity = "MEDIUM"
	}
	if req.DuePeriodDays == 0 {
		req.DuePeriodDays = 30
	}
	if req.Status == "" {
		req.Status = "ACTIVE"
	}

	userID, _ := c.Get(middleware.CtxUserID)

	_, err = database.DB.Exec(`
		UPDATE compliance_rules SET rule_code=?, title=?, description=?, category_id=?,
		       applicable_mine_id=?, frequency=?, severity=?, responsible_dept=?,
		       due_period_days=?, status=?, updated_by=?
		WHERE id=?`,
		req.RuleCode, req.Title, req.Description, req.CategoryID,
		req.ApplicableMine, req.Frequency, req.Severity, req.ResponsibleDept,
		req.DuePeriodDays, req.Status, userID, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update compliance rule", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "COMPLIANCE_RULE_UPDATED", "COMPLIANCE", strconv.Itoa(id),
		map[string]interface{}{"rule_code": req.RuleCode, "title": req.Title}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Compliance rule updated successfully", nil)
}

// DeactivateRule performs a soft-delete/deactivation — restricted to SUPER_ADMIN via RBAC middleware.
func (cc *ComplianceController) DeactivateRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid compliance rule ID", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	_, err = database.DB.Exec(`UPDATE compliance_rules SET status='INACTIVE', updated_by=? WHERE id=?`, userID, id)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to deactivate compliance rule", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "COMPLIANCE_RULE_DEACTIVATED", "COMPLIANCE", strconv.Itoa(id),
		map[string]interface{}{"rule_id": id}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Compliance rule deactivated successfully", nil)
}

// ListCategories returns all compliance categories for dropdowns.
func (cc *ComplianceController) ListCategories(c *gin.Context) {
	rows, err := database.DB.Query(`SELECT id, name, description FROM compliance_categories ORDER BY name`)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch categories", err.Error())
		return
	}
	defer rows.Close()

	type cat struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	cats := []cat{}
	for rows.Next() {
		var ct cat
		if err := rows.Scan(&ct.ID, &ct.Name, &ct.Description); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse category", err.Error())
			return
		}
		cats = append(cats, ct)
	}

	utils.Success(c, http.StatusOK, "Categories fetched successfully", cats)
}