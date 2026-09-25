package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/config"
	"coal-governance-backend/utils"
)

// Context keys used to pass authenticated user info down the request chain.
const (
	CtxUserID  = "userID"
	CtxEmail   = "email"
	CtxRoleKey = "roleKey"
)

// AuthRequired validates the Bearer JWT on every protected route and injects
// the authenticated user's identity into the Gin context.
func AuthRequired(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			utils.Fail(c, http.StatusUnauthorized, "Authentication required", "missing or malformed Authorization header")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		claims, err := utils.ParseJWT(cfg.JWTSecret, tokenString)
		if err != nil {
			utils.Fail(c, http.StatusUnauthorized, "Session expired or invalid, please login again", err.Error())
			c.Abort()
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxEmail, claims.Email)
		c.Set(CtxRoleKey, claims.RoleKey)
		c.Next()
	}
}

// RequireRoles restricts a route to a specific set of role keys (RBAC).
// Usage: router.POST("/mines", middleware.RequireRoles("SUPER_ADMIN"), controller)
func RequireRoles(allowedRoles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedRoles))
	for _, r := range allowedRoles {
		allowed[r] = true
	}

	return func(c *gin.Context) {
		roleKey, exists := c.Get(CtxRoleKey)
		if !exists {
			utils.Fail(c, http.StatusUnauthorized, "Authentication required", "no role found in context")
			c.Abort()
			return
		}

		if !allowed[roleKey.(string)] {
			utils.Fail(c, http.StatusForbidden, "You do not have permission to perform this action", "insufficient role privileges")
			c.Abort()
			return
		}

		c.Next()
	}
}
