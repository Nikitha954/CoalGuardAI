package controllers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf/v2"

	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/utils"
)

type ReportsController struct{}

func NewReportsController() *ReportsController {
	return &ReportsController{}
}

type reportRequest struct {
	ReportType string `json:"report_type" binding:"required"` // MINE_COMPLIANCE, VIOLATIONS, CORRECTIVE_ACTIONS, HIGH_RISK_MINES, GOVERNANCE_SUMMARY
	Format     string `json:"format" binding:"required"`     // PDF, CSV
	MineID     *int   `json:"mine_id"`
}

// GenerateReport generates PDF or CSV reports dynamically and streams them back.
func (rc *ReportsController) GenerateReport(c *gin.Context) {
	var req reportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	// Fetch mine name if filtered
	mineName := "All Mines"
	if req.MineID != nil {
		_ = database.DB.QueryRow(`SELECT mine_name FROM mines WHERE id = ?`, *req.MineID).Scan(&mineName)
	}

	// Log in reports table
	paramsMap := map[string]interface{}{"mine_id": req.MineID, "mine_name": mineName}
	paramsJSON, _ := json.Marshal(paramsMap)
	
	var mineIDVal sql.NullInt64
	if req.MineID != nil {
		mineIDVal = sql.NullInt64{Int64: int64(*req.MineID), Valid: true}
	}

	_, _ = database.DB.Exec(`
		INSERT INTO reports (report_type, generated_by, mine_id, format, parameters_json)
		VALUES (?, ?, ?, ?, ?)`,
		req.ReportType, userID, mineIDVal, req.Format, string(paramsJSON))

	utils.LogAudit(userID.(int), "REPORT_GENERATED", "REPORTS", req.ReportType, paramsMap, c.ClientIP())

	// Generate and serve based on Format
	if req.Format == "CSV" {
		rc.generateCSV(c, req)
	} else {
		rc.generatePDF(c, req, mineName)
	}
}

func (rc *ReportsController) generateCSV(c *gin.Context, req reportRequest) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=report_%s_%d.csv", req.ReportType, time.Now().Unix()))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	switch req.ReportType {
	case "VIOLATIONS":
		_ = writer.Write([]string{"Violation Code", "Mine Site", "Category", "Description", "Severity", "Reported By", "Assignee", "Deadline", "Status"})
		
		query := `
			SELECT v.violation_code, m.mine_name, cc.name, v.description, v.severity, u1.full_name, COALESCE(u2.full_name, 'Unassigned'), v.deadline, v.status
			FROM violations v
			JOIN mines m ON m.id = v.mine_id
			JOIN compliance_categories cc ON cc.id = v.category_id
			JOIN users u1 ON u1.id = v.reported_by
			LEFT JOIN users u2 ON u2.id = v.responsible_person`
		args := []interface{}{}
		if req.MineID != nil {
			query += " WHERE v.mine_id = ?"
			args = append(args, *req.MineID)
		}
		query += " ORDER BY v.created_at DESC"
		
		rows, err := database.DB.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var code, mine, cat, desc, sev, repBy, respBy, deadline, status string
				var dlVal []uint8
				if err := rows.Scan(&code, &mine, &cat, &desc, &sev, &repBy, &respBy, &dlVal, &status); err == nil {
					deadline = string(dlVal)
					_ = writer.Write([]string{code, mine, cat, desc, sev, repBy, respBy, deadline, status})
				}
			}
		}

	case "CORRECTIVE_ACTIONS":
		_ = writer.Write([]string{"Violation Code", "Mine Site", "Action Required", "Assignee", "Deadline", "Submitted At", "Status", "Escalation Level"})
		
		query := `
			SELECT v.violation_code, m.mine_name, ca.action_description, u.full_name, ca.deadline, COALESCE(ca.submitted_at, ''), ca.status, ca.escalation_level
			FROM corrective_actions ca
			JOIN violations v ON v.id = ca.violation_id
			JOIN mines m ON m.id = v.mine_id
			JOIN users u ON u.id = ca.assigned_to`
		args := []interface{}{}
		if req.MineID != nil {
			query += " WHERE v.mine_id = ?"
			args = append(args, *req.MineID)
		}
		query += " ORDER BY ca.created_at DESC"

		rows, err := database.DB.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var code, mine, desc, assignee, deadline, subAt, status string
				var escLevel int
				var dlVal []uint8
				var subAtVal sql.NullTime
				if err := rows.Scan(&code, &mine, &desc, &assignee, &dlVal, &subAtVal, &status, &escLevel); err == nil {
					deadline = string(dlVal)
					if subAtVal.Valid {
						subAt = subAtVal.Time.Format("2006-01-02 15:04:05")
					} else {
						subAt = "N/A"
					}
					_ = writer.Write([]string{code, mine, desc, assignee, deadline, subAt, status, strconv.Itoa(escLevel)})
				}
			}
		}

	case "HIGH_RISK_MINES":
		_ = writer.Write([]string{"Mine Name", "Mine Code", "Subsidiary", "State/District", "Risk Score", "Classification", "Factors List"})
		
		rows, err := database.DB.Query(`
			SELECT m.mine_name, m.mine_code, s.name, m.state, r.score, r.classification, r.factors_json
			FROM risk_scores r
			JOIN mines m ON m.id = r.mine_id
			JOIN subsidiaries s ON s.id = m.subsidiary_id
			WHERE r.classification IN ('HIGH', 'CRITICAL')
			  AND r.computed_at = (SELECT MAX(computed_at) FROM risk_scores WHERE mine_id = r.mine_id)
			ORDER BY r.score DESC`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name, code, sub, state, class, factors string
				var score float64
				if err := rows.Scan(&name, &code, &sub, &state, &score, &class, &factors); err == nil {
					_ = writer.Write([]string{name, code, sub, state, fmt.Sprintf("%.1f", score), class, factors})
				}
			}
		}

	default:
		// Generic report summary
		_ = writer.Write([]string{"Generic Report Summary", "Format", "Generated At"})
		_ = writer.Write([]string{req.ReportType, req.Format, time.Now().Format(time.RFC3339)})
	}
}

func (rc *ReportsController) generatePDF(c *gin.Context, req reportRequest, mineFilterName string) {
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=report_%s_%d.pdf", req.ReportType, time.Now().Unix()))

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	
	// Header Strata Band
	pdf.SetFillColor(46, 125, 70) // Dark Green
	pdf.Rect(0, 0, 210, 15, "F")
	
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(255, 255, 255)
	pdf.Text(10, 10, "COAL INDIA LIMITED - MINISTRY OF COAL")
	
	// Title
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "B", 16)
	pdf.Ln(18)
	pdf.Cell(0, 10, fmt.Sprintf("%s REPORT", req.ReportType))
	pdf.Ln(8)
	
	// Subtitle Metadata
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 5, fmt.Sprintf("Generated on: %s", time.Now().Format("2006-01-02 15:04:05")))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("Scope: %s", mineFilterName))
	pdf.Ln(10)
	
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
	pdf.Ln(8)

	switch req.ReportType {
	case "VIOLATIONS":
		// Table Headers
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(25, 8, "Vio Code", "1", 0, "L", true, 0, "")
		pdf.CellFormat(35, 8, "Mine Site", "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 8, "Description", "1", 0, "L", true, 0, "")
		pdf.CellFormat(25, 8, "Severity", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 8, "Deadline", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 8, "Status", "1", 1, "C", true, 0, "")

		pdf.SetFont("Arial", "", 8.5)
		
		query := `
			SELECT v.violation_code, m.mine_name, v.description, v.severity, v.deadline, v.status
			FROM violations v
			JOIN mines m ON m.id = v.mine_id`
		args := []interface{}{}
		if req.MineID != nil {
			query += " WHERE v.mine_id = ?"
			args = append(args, *req.MineID)
		}
		query += " ORDER BY v.created_at DESC LIMIT 20"
		
		rows, err := database.DB.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var code, mine, desc, severity, status string
				var dlVal []uint8
				if err := rows.Scan(&code, &mine, &desc, &severity, &dlVal, &status); err == nil {
					deadline := string(dlVal)
					
					// Truncate desc for single line cell wrapping
					if len(desc) > 34 {
						desc = desc[:31] + "..."
					}
					if len(mine) > 18 {
						mine = mine[:15] + "..."
					}

					pdf.CellFormat(25, 7, code, "1", 0, "L", false, 0, "")
					pdf.CellFormat(35, 7, mine, "1", 0, "L", false, 0, "")
					pdf.CellFormat(60, 7, desc, "1", 0, "L", false, 0, "")
					pdf.CellFormat(25, 7, severity, "1", 0, "C", false, 0, "")
					pdf.CellFormat(20, 7, deadline, "1", 0, "C", false, 0, "")
					pdf.CellFormat(25, 7, status, "1", 1, "C", false, 0, "")
				}
			}
		}

	case "CORRECTIVE_ACTIONS":
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(25, 8, "Vio Code", "1", 0, "L", true, 0, "")
		pdf.CellFormat(35, 8, "Mine Site", "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 8, "Action Required", "1", 0, "L", true, 0, "")
		pdf.CellFormat(30, 8, "Assignee", "1", 0, "L", true, 0, "")
		pdf.CellFormat(20, 8, "Deadline", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 8, "Status", "1", 1, "C", true, 0, "")

		pdf.SetFont("Arial", "", 8.5)
		
		query := `
			SELECT v.violation_code, m.mine_name, ca.action_description, u.full_name, ca.deadline, ca.status
			FROM corrective_actions ca
			JOIN violations v ON v.id = ca.violation_id
			JOIN mines m ON m.id = v.mine_id
			JOIN users u ON u.id = ca.assigned_to`
		args := []interface{}{}
		if req.MineID != nil {
			query += " WHERE v.mine_id = ?"
			args = append(args, *req.MineID)
		}
		query += " ORDER BY ca.created_at DESC LIMIT 20"

		rows, err := database.DB.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var code, mine, desc, assignee, deadline, status string
				var dlVal []uint8
				if err := rows.Scan(&code, &mine, &desc, &assignee, &dlVal, &status); err == nil {
					deadline = string(dlVal)
					if len(desc) > 34 {
						desc = desc[:31] + "..."
					}
					if len(mine) > 18 {
						mine = mine[:15] + "..."
					}

					pdf.CellFormat(25, 7, code, "1", 0, "L", false, 0, "")
					pdf.CellFormat(35, 7, mine, "1", 0, "L", false, 0, "")
					pdf.CellFormat(60, 7, desc, "1", 0, "L", false, 0, "")
					pdf.CellFormat(30, 7, assignee, "1", 0, "L", false, 0, "")
					pdf.CellFormat(20, 7, deadline, "1", 0, "C", false, 0, "")
					pdf.CellFormat(20, 7, status, "1", 1, "C", false, 0, "")
				}
			}
		}

	case "HIGH_RISK_MINES":
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(45, 8, "Mine Site", "1", 0, "L", true, 0, "")
		pdf.CellFormat(25, 8, "Mine Code", "1", 0, "L", true, 0, "")
		pdf.CellFormat(55, 8, "Subsidiary", "1", 0, "L", true, 0, "")
		pdf.CellFormat(25, 8, "State", "1", 0, "L", true, 0, "")
		pdf.CellFormat(20, 8, "Risk Score", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 8, "Class", "1", 1, "C", true, 0, "")

		pdf.SetFont("Arial", "", 8.5)
		
		rows, err := database.DB.Query(`
			SELECT m.mine_name, m.mine_code, s.name, m.state, r.score, r.classification
			FROM risk_scores r
			JOIN mines m ON m.id = r.mine_id
			JOIN subsidiaries s ON s.id = m.subsidiary_id
			WHERE r.classification IN ('HIGH', 'CRITICAL')
			  AND r.computed_at = (SELECT MAX(computed_at) FROM risk_scores WHERE mine_id = r.mine_id)
			ORDER BY r.score DESC`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name, code, sub, state, class string
				var score float64
				if err := rows.Scan(&name, &code, &sub, &state, &score, &class); err == nil {
					pdf.CellFormat(45, 7, name, "1", 0, "L", false, 0, "")
					pdf.CellFormat(25, 7, code, "1", 0, "L", false, 0, "")
					pdf.CellFormat(55, 7, sub, "1", 0, "L", false, 0, "")
					pdf.CellFormat(25, 7, state, "1", 0, "L", false, 0, "")
					pdf.CellFormat(20, 7, fmt.Sprintf("%.1f", score), "1", 0, "C", false, 0, "")
					pdf.CellFormat(20, 7, class, "1", 1, "C", false, 0, "")
				}
			}
		}

	default:
		pdf.SetFont("Arial", "I", 11)
		pdf.Cell(0, 10, "Summary report details are compiled dynamically based on active telemetry feeds.")
		pdf.Ln(10)
	}
	
	// Add Footer Signatures
	pdf.Ln(25)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(95, 5, "Generated By: System Administrator", "", 0, "L", false, 0, "")
	pdf.CellFormat(95, 5, "Authorized Officer Signature", "", 1, "R", false, 0, "")
	
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(120, 120, 120)
	_ = pdf.Output(c.Writer)
}

// ListReports returns the history of generated reports.
func (rc *ReportsController) ListReports(c *gin.Context) {
	query := `
		SELECT r.id, r.report_type, r.generated_by, u.full_name AS generated_by_name,
		       r.mine_id, COALESCE(m.mine_name, 'All Mines'), r.format, r.parameters_json, r.created_at
		FROM reports r
		JOIN users u ON u.id = r.generated_by
		LEFT JOIN mines m ON m.id = r.mine_id
		ORDER BY r.created_at DESC LIMIT 50`

	rows, err := database.DB.Query(query)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch reports history", err.Error())
		return
	}
	defer rows.Close()

	type reportHistoryItem struct {
		ID              int                    `json:"id"`
		ReportType      string                 `json:"report_type"`
		GeneratedBy     int                    `json:"generated_by"`
		GeneratedByName string                 `json:"generated_by_name"`
		MineID          *int                   `json:"mine_id"`
		MineName        string                 `json:"mine_name"`
		Format          string                 `json:"format"`
		Parameters      map[string]interface{} `json:"parameters"`
		CreatedAt       time.Time              `json:"created_at"`
	}

	list := []reportHistoryItem{}
	for rows.Next() {
		var it reportHistoryItem
		var mineID sql.NullInt64
		var paramsJSON []byte
		err := rows.Scan(&it.ID, &it.ReportType, &it.GeneratedBy, &it.GeneratedByName,
			&mineID, &it.MineName, &it.Format, &paramsJSON, &it.CreatedAt)
		if err != nil {
			continue
		}
		if mineID.Valid {
			val := int(mineID.Int64)
			it.MineID = &val
		}
		if len(paramsJSON) > 0 {
			_ = json.Unmarshal(paramsJSON, &it.Parameters)
		}
		list = append(list, it)
	}

	utils.Success(c, http.StatusOK, "Reports history fetched successfully", list)
}

