package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/models"
	"coal-governance-backend/utils"
)

type AuditController struct{}

func NewAuditController() *AuditController {
	return &AuditController{}
}

// ListAuditLogs fetches audit trail records with filters.
func (ac *AuditController) ListAuditLogs(c *gin.Context) {
	query := `
		SELECT a.id, a.user_id, COALESCE(u.full_name, 'System / Anonymous'),
		       a.action, a.module, a.record_id, a.details, a.ip_address,
		       COALESCE(a.prev_hash, ''), COALESCE(a.hash, ''), a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.user_id
		WHERE 1=1`
	args := []interface{}{}

	if module := c.Query("module"); module != "" {
		query += " AND a.module = ?"
		args = append(args, module)
	}
	if action := c.Query("action"); action != "" {
		query += " AND a.action = ?"
		args = append(args, action)
	}
	if userID := c.Query("user_id"); userID != "" {
		query += " AND a.user_id = ?"
		args = append(args, userID)
	}
	if startDate := c.Query("start_date"); startDate != "" {
		query += " AND a.created_at >= ?"
		args = append(args, startDate)
	}
	if endDate := c.Query("end_date"); endDate != "" {
		query += " AND a.created_at <= ?"
		args = append(args, endDate)
	}

	query += " ORDER BY a.id DESC LIMIT 200"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch audit logs", err.Error())
		return
	}
	defer rows.Close()

	logs := []models.AuditLog{}
	for rows.Next() {
		var a models.AuditLog
		var uid sql.NullInt64
		var detailsJSON []byte

		err := rows.Scan(
			&a.ID, &uid, &a.UserName,
			&a.Action, &a.Module, &a.RecordID, &detailsJSON, &a.IPAddress,
			&a.PrevHash, &a.Hash, &a.CreatedAt,
		)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse audit log record", err.Error())
			return
		}

		if uid.Valid {
			val := int(uid.Int64)
			a.UserID = &val
		}

		if len(detailsJSON) > 0 {
			var d map[string]interface{}
			if err := json.Unmarshal(detailsJSON, &d); err == nil {
				a.Details = d
			}
		}
		if a.Details == nil {
			a.Details = map[string]interface{}{}
		}

		logs = append(logs, a)
	}

	utils.Success(c, http.StatusOK, "Audit logs fetched successfully", logs)
}

type chainVerificationResult struct {
	TotalLogs       int          `json:"total_logs"`
	VerifiedLogs    int          `json:"verified_logs"`
	TamperedLogs    int          `json:"tampered_logs"`
	IsChainIntact   bool         `json:"is_chain_intact"`
	GenesisHash     string       `json:"genesis_hash"`
	LatestHash      string       `json:"latest_hash"`
	VerifiedAt      time.Time    `json:"verified_at"`
	TamperedEntries []tamperedLog `json:"tampered_entries,omitempty"`
}

type tamperedLog struct {
	ID           int64  `json:"id"`
	Action       string `json:"action"`
	Module       string `json:"module"`
	StoredHash   string `json:"stored_hash"`
	ComputedHash string `json:"computed_hash"`
	Reason       string `json:"reason"`
}

// VerifyAuditChain validates the cryptographic hash chain sequentially.
// It detects any row modification, deletion, or tampering in the database.
func (ac *AuditController) VerifyAuditChain(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT id, user_id, action, module, record_id, details, ip_address,
		       COALESCE(prev_hash, ''), COALESCE(hash, ''), created_at
		FROM audit_logs
		ORDER BY id ASC`)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query audit trail for verification", err.Error())
		return
	}
	defer rows.Close()

	var total, verified, tampered int
	var tamperedList []tamperedLog
	expectedPrevHash := utils.GenesisHash
	latestHash := utils.GenesisHash

	for rows.Next() {
		total++
		var id int64
		var uid sql.NullInt64
		var action, module, recordID, ip, prevHash, hash string
		var detailsJSON []byte
		var createdAt time.Time

		err := rows.Scan(&id, &uid, &action, &module, &recordID, &detailsJSON, &ip, &prevHash, &hash, &createdAt)
		if err != nil {
			continue
		}

		userID := 0
		if uid.Valid {
			userID = int(uid.Int64)
		}

		// Handle legacy rows without hash
		if hash == "" {
			continue
		}

		latestHash = hash

		// 1. Verify prev_hash continuity
		if prevHash != expectedPrevHash {
			tampered++
			tamperedList = append(tamperedList, tamperedLog{
				ID:           id,
				Action:       action,
				Module:       module,
				StoredHash:   hash,
				ComputedHash: "",
				Reason:       "Broken hash pointer: prev_hash does not match previous record's hash",
			})
			expectedPrevHash = hash
			continue
		}

		// 2. Recompute row hash
		timestampStr := createdAt.UTC().Format("2006-01-02 15:04:05")
		computed := utils.ComputeAuditHash(prevHash, userID, action, module, recordID, string(detailsJSON), ip, timestampStr)

		if computed != hash {
			tampered++
			tamperedList = append(tamperedList, tamperedLog{
				ID:           id,
				Action:       action,
				Module:       module,
				StoredHash:   hash,
				ComputedHash: computed,
				Reason:       "Payload integrity violation: data content or metadata has been altered",
			})
		} else {
			verified++
		}

		expectedPrevHash = hash
	}

	isChainIntact := (tampered == 0 && total > 0)
	result := chainVerificationResult{
		TotalLogs:       total,
		VerifiedLogs:    verified,
		TamperedLogs:    tampered,
		IsChainIntact:   isChainIntact,
		GenesisHash:     utils.GenesisHash,
		LatestHash:      latestHash,
		VerifiedAt:      time.Now(),
		TamperedEntries: tamperedList,
	}

	utils.Success(c, http.StatusOK, "Audit log chain verification completed", result)
}
