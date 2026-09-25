package controllers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/config"
	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/models"
	"coal-governance-backend/utils"
)

type AuthController struct {
	Cfg *config.Config
}

func NewAuthController(cfg *config.Config) *AuthController {
	return &AuthController{Cfg: cfg}
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login validates credentials and issues a JWT. Every attempt (success or
// failure) is written to the audit trail for governance transparency.
func (ac *AuthController) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request", err.Error())
		return
	}

	var u models.User
	var subsidiaryID sql.NullInt64

	row := database.DB.QueryRow(`
		SELECT u.id, u.full_name, u.email, u.password_hash, u.role_id, r.role_key, r.role_name,
			   u.subsidiary_id, u.phone, u.status
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.email = ?`, req.Email)

	err := row.Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.RoleID, &u.RoleKey, &u.RoleName,
		&subsidiaryID, &u.Phone, &u.Status)

	if err == sql.ErrNoRows {
		utils.LogAudit(0, "LOGIN_FAILED", "AUTH", req.Email, map[string]interface{}{"reason": "user not found"}, c.ClientIP())
		utils.Fail(c, http.StatusUnauthorized, "Invalid email or password", "authentication failed")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Login failed", err.Error())
		return
	}

	if u.Status != "ACTIVE" {
		utils.LogAudit(u.ID, "LOGIN_BLOCKED", "AUTH", req.Email, map[string]interface{}{"status": u.Status}, c.ClientIP())
		utils.Fail(c, http.StatusForbidden, "Account is not active. Contact your administrator.", "account inactive")
		return
	}

	if !utils.CheckPassword(u.PasswordHash, req.Password) {
		utils.LogAudit(u.ID, "LOGIN_FAILED", "AUTH", req.Email, map[string]interface{}{"reason": "bad password"}, c.ClientIP())
		utils.Fail(c, http.StatusUnauthorized, "Invalid email or password", "authentication failed")
		return
	}

	token, err := utils.GenerateJWT(ac.Cfg.JWTSecret, ac.Cfg.JWTExpiryHrs, u.ID, u.Email, u.RoleKey)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to generate session", err.Error())
		return
	}

	_, _ = database.DB.Exec(`UPDATE users SET last_login_at = NOW() WHERE id = ?`, u.ID)
	utils.LogAudit(u.ID, "LOGIN_SUCCESS", "AUTH", u.Email, nil, c.ClientIP())

	if subsidiaryID.Valid {
		sid := int(subsidiaryID.Int64)
		u.SubsidiaryID = &sid
	}

	utils.Success(c, http.StatusOK, "Login successful", gin.H{
		"token": token,
		"user": gin.H{
			"id":            u.ID,
			"full_name":     u.FullName,
			"email":         u.Email,
			"role_key":      u.RoleKey,
			"role_name":     u.RoleName,
			"subsidiary_id": u.SubsidiaryID,
		},
	})
}

// Logout is stateless (JWT-based); the client discards its token. We still
// record the event in the audit trail so governance history stays complete.
func (ac *AuthController) Logout(c *gin.Context) {
	userID, _ := c.Get(middleware.CtxUserID)
	email, _ := c.Get(middleware.CtxEmail)

	if uid, ok := userID.(int); ok {
		utils.LogAudit(uid, "LOGOUT", "AUTH", email.(string), nil, c.ClientIP())
	}

	utils.Success(c, http.StatusOK, "Logged out successfully", nil)
}

// Me returns the profile of the currently authenticated user.
func (ac *AuthController) Me(c *gin.Context) {
	userID, _ := c.Get(middleware.CtxUserID)

	var u models.User
	var subsidiaryID sql.NullInt64

	row := database.DB.QueryRow(`
		SELECT u.id, u.full_name, u.email, r.role_key, r.role_name, u.subsidiary_id, u.phone, u.status, u.created_at
		FROM users u JOIN roles r ON r.id = u.role_id
		WHERE u.id = ?`, userID)

	err := row.Scan(&u.ID, &u.FullName, &u.Email, &u.RoleKey, &u.RoleName, &subsidiaryID, &u.Phone, &u.Status, &u.CreatedAt)
	if err != nil {
		utils.Fail(c, http.StatusNotFound, "User not found", err.Error())
		return
	}

	if subsidiaryID.Valid {
		sid := int(subsidiaryID.Int64)
		u.SubsidiaryID = &sid
	}

	utils.Success(c, http.StatusOK, "User profile", u)
}

// ListUsers returns all users in the system, with optional role/status filtering.
func (ac *AuthController) ListUsers(c *gin.Context) {
	query := `
		SELECT u.id, u.full_name, u.email, u.role_id, r.role_key, r.role_name, u.subsidiary_id, u.phone, u.status, u.created_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE 1=1`
	args := []interface{}{}

	if status := c.Query("status"); status != "" {
		query += " AND u.status = ?"
		args = append(args, status)
	}
	if role := c.Query("role"); role != "" {
		query += " AND r.role_key = ?"
		args = append(args, role)
	}

	query += " ORDER BY u.full_name ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch users", err.Error())
		return
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		var u models.User
		var subsidiaryID sql.NullInt64

		err := rows.Scan(&u.ID, &u.FullName, &u.Email, &u.RoleID, &u.RoleKey, &u.RoleName, &subsidiaryID, &u.Phone, &u.Status, &u.CreatedAt)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse user", err.Error())
			return
		}

		if subsidiaryID.Valid {
			sid := int(subsidiaryID.Int64)
			u.SubsidiaryID = &sid
		}
		users = append(users, u)
	}

	utils.Success(c, http.StatusOK, "Users fetched successfully", users)
}

type userManageRequest struct {
	FullName     string `json:"full_name" binding:"required"`
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password"`
	RoleID       int    `json:"role_id" binding:"required"`
	SubsidiaryID *int   `json:"subsidiary_id"`
	Phone        string `json:"phone"`
	Status       string `json:"status"`
}

// CreateUser handles administrative creation of new accounts.
func (ac *AuthController) CreateUser(c *gin.Context) {
	var req userManageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request", err.Error())
		return
	}

	if req.Password == "" {
		req.Password = "Coal@2026"
	}
	if req.Status == "" {
		req.Status = "ACTIVE"
	}

	pwdHash, err := utils.HashPassword(req.Password)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to hash password", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	var newID int64
	err = database.DB.QueryRow(`
		INSERT INTO users (full_name, email, password_hash, role_id, subsidiary_id, phone, status, created_by) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`, 
		req.FullName, req.Email, pwdHash, req.RoleID, req.SubsidiaryID, req.Phone, req.Status, userID).Scan(&newID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to create user (check if email already exists)", err.Error())
		return
	}

	utils.LogAudit(userID, "USER_CREATED", "USERS", strconv.FormatInt(newID, 10),
		map[string]interface{}{"email": req.Email, "full_name": req.FullName, "role_id": req.RoleID}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "User created successfully", gin.H{"id": newID})
}

// UpdateUser handles updating profile, role, subsidiary or status.
func (ac *AuthController) UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid user ID", err.Error())
		return
	}

	var req userManageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	if req.Password != "" {
		pwdHash, err := utils.HashPassword(req.Password)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to hash password", err.Error())
			return
		}
		_, err = database.DB.Exec(`
			UPDATE users 
			SET full_name = ?, email = ?, password_hash = ?, role_id = ?, subsidiary_id = ?, phone = ?, status = ?, updated_by = ?
			WHERE id = ?`,
			req.FullName, req.Email, pwdHash, req.RoleID, req.SubsidiaryID, req.Phone, req.Status, userID, id)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to update user", err.Error())
			return
		}
	} else {
		_, err = database.DB.Exec(`
			UPDATE users 
			SET full_name = ?, email = ?, role_id = ?, subsidiary_id = ?, phone = ?, status = ?, updated_by = ?
			WHERE id = ?`,
			req.FullName, req.Email, req.RoleID, req.SubsidiaryID, req.Phone, req.Status, userID, id)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to update user", err.Error())
			return
		}
	}

	utils.LogAudit(userID, "USER_UPDATED", "USERS", strconv.Itoa(id),
		map[string]interface{}{"email": req.Email, "status": req.Status}, c.ClientIP())

	utils.Success(c, http.StatusOK, "User updated successfully", nil)
}

// DeactivateUser disables an account.
func (ac *AuthController) DeactivateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid user ID", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	_, err = database.DB.Exec(`UPDATE users SET status = 'INACTIVE', updated_by = ? WHERE id = ?`, userID, id)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to deactivate user", err.Error())
		return
	}

	utils.LogAudit(userID, "USER_DEACTIVATED", "USERS", strconv.Itoa(id), nil, c.ClientIP())
	utils.Success(c, http.StatusOK, "User account deactivated successfully", nil)
}

// ListRoles returns all defined system roles.
func (ac *AuthController) ListRoles(c *gin.Context) {
	rows, err := database.DB.Query(`SELECT id, role_key, role_name, COALESCE(description, '') FROM roles ORDER BY id ASC`)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query roles", err.Error())
		return
	}
	defer rows.Close()

	type roleItem struct {
		ID          int    `json:"id"`
		RoleKey     string `json:"role_key"`
		RoleName    string `json:"role_name"`
		Description string `json:"description"`
	}

	roles := []roleItem{}
	for rows.Next() {
		var r roleItem
		if err := rows.Scan(&r.ID, &r.RoleKey, &r.RoleName, &r.Description); err == nil {
			roles = append(roles, r)
		}
	}

	utils.Success(c, http.StatusOK, "Roles fetched successfully", roles)
}