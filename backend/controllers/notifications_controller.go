package controllers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/utils"
)

type NotificationsController struct{}

func NewNotificationsController() *NotificationsController {
	return &NotificationsController{}
}

// ListNotifications returns in-app notifications for the logged-in user.
func (nc *NotificationsController) ListNotifications(c *gin.Context) {
	userID, _ := c.Get(middleware.CtxUserID)

	query := `
		SELECT id, title, message, severity, type, is_read, created_at
		FROM notifications
		WHERE recipient_id = ?`
	args := []interface{}{userID}

	if unreadOnly := c.Query("unread_only"); unreadOnly == "true" {
		query += " AND is_read = FALSE"
	}

	query += " ORDER BY created_at DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query notifications", err.Error())
		return
	}
	defer rows.Close()

	type notificationItem struct {
		ID        int       `json:"id"`
		Title     string    `json:"title"`
		Message   string    `json:"message"`
		Severity  string    `json:"severity"`
		Type      string    `json:"type"`
		IsRead    bool      `json:"is_read"`
		CreatedAt time.Time `json:"created_at"`
	}

	list := []notificationItem{}
	for rows.Next() {
		var n notificationItem
		err := rows.Scan(&n.ID, &n.Title, &n.Message, &n.Severity, &n.Type, &n.IsRead, &n.CreatedAt)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse notification", err.Error())
			return
		}
		list = append(list, n)
	}

	utils.Success(c, http.StatusOK, "Notifications fetched", list)
}

// MarkAsRead marks a specific notification as read.
func (nc *NotificationsController) MarkAsRead(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get(middleware.CtxUserID)

	// Verify ownership before updating
	var recipientID int
	err := database.DB.QueryRow(`SELECT recipient_id FROM notifications WHERE id = ?`, id).Scan(&recipientID)
	if err == sql.ErrNoRows {
		utils.Fail(c, http.StatusNotFound, "Notification not found", "not found")
		return
	} else if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Database query failed", err.Error())
		return
	}

	if recipientID != userID.(int) {
		utils.Fail(c, http.StatusForbidden, "You do not own this notification", "forbidden")
		return
	}

	_, err = database.DB.Exec(`UPDATE notifications SET is_read = TRUE WHERE id = ?`, id)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to mark read", err.Error())
		return
	}

	utils.Success(c, http.StatusOK, "Notification marked as read", nil)
}
