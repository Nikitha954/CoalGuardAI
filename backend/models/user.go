package models

import "time"

// User represents a system user (any of the 6 roles).
type User struct {
	ID           int        `json:"id"`
	FullName     string     `json:"full_name"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	RoleID       int        `json:"role_id"`
	RoleKey      string     `json:"role_key,omitempty"`
	RoleName     string     `json:"role_name,omitempty"`
	SubsidiaryID *int       `json:"subsidiary_id"`
	Phone        string     `json:"phone"`
	Status       string     `json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Role keys — must match roles.role_key in the database exactly.
const (
	RoleSuperAdmin       = "SUPER_ADMIN"
	RoleMineManager      = "MINE_MANAGER"
	RoleSafetyOfficer    = "SAFETY_OFFICER"
	RoleInspector        = "INSPECTOR"
	RoleCorporateManager = "CORPORATE_MANAGER"
	RoleRegulatoryOffice = "REGULATORY_OFFICER"
)
