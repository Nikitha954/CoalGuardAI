package utils

import "github.com/gin-gonic/gin"

// APIResponse is the consistent JSON envelope used by every endpoint.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Success sends a 2xx response with the standard success envelope.
func Success(c *gin.Context, status int, message string, data interface{}) {
	c.JSON(status, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Fail sends an error response with the standard error envelope.
func Fail(c *gin.Context, status int, message string, err string) {
	c.JSON(status, APIResponse{
		Success: false,
		Message: message,
		Error:   err,
	})
}
