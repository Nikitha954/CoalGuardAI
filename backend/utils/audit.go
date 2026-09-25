package utils

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"coal-governance-backend/database"
)

const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

var auditMutex sync.Mutex

// ComputeAuditHash calculates the SHA-256 hash of an audit log entry.
func ComputeAuditHash(prevHash string, userID int, action, module, recordID, detailsJSON, ip, timestamp string) string {
	payload := fmt.Sprintf("%s|%d|%s|%s|%s|%s|%s|%s",
		prevHash, userID, action, module, recordID, detailsJSON, ip, timestamp)
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

// LogAudit writes a tamper-evident row to audit_logs with cryptographic SHA-256 hash chaining.
// It never fails the calling request — problems are logged server-side without blocking actions.
func LogAudit(userID int, action, module, recordID string, details map[string]interface{}, ip string) {
	auditMutex.Lock()
	defer auditMutex.Unlock()

	var detailsJSON []byte
	var err error
	if details != nil {
		detailsJSON, err = json.Marshal(details)
		if err != nil {
			log.Println("audit: failed to marshal details:", err)
			detailsJSON = []byte("{}")
		}
	} else {
		detailsJSON = []byte("{}")
	}

	var userIDVal sql.NullInt64
	if userID > 0 {
		userIDVal = sql.NullInt64{Int64: int64(userID), Valid: true}
	}

	// 1. Fetch latest entry's hash to serve as prev_hash
	var prevHash string
	err = database.DB.QueryRow(`SELECT hash FROM audit_logs WHERE hash IS NOT NULL AND hash != '' ORDER BY id DESC LIMIT 1`).Scan(&prevHash)
	if err == sql.ErrNoRows || err != nil || prevHash == "" {
		prevHash = GenesisHash
	}

	now := time.Now().UTC()
	timestampStr := now.Format("2006-01-02 15:04:05")

	// 2. Compute SHA-256 hash for the current block
	currentHash := ComputeAuditHash(prevHash, userID, action, module, recordID, string(detailsJSON), ip, timestampStr)

	// 3. Insert record with hash chaining
	_, err = database.DB.Exec(
		`INSERT INTO audit_logs (user_id, action, module, record_id, details, ip_address, prev_hash, hash, created_at) 
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userIDVal, action, module, recordID, detailsJSON, ip, prevHash, currentHash, timestampStr,
	)
	if err != nil {
		log.Println("audit: failed to insert audit log:", err)
	}
}

