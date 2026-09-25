package controllers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/config"
	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/utils"
)

type DocumentsController struct {
	Cfg *config.Config
}

func NewDocumentsController(cfg *config.Config) *DocumentsController {
	return &DocumentsController{Cfg: cfg}
}

type documentItem struct {
	ID                  int         `json:"id"`
	MineID              *int        `json:"mine_id"`
	MineName            string      `json:"mine_name"`
	MineCode            string      `json:"mine_code"`
	ContractorID        *int        `json:"contractor_id"`
	ContractorName      string      `json:"contractor_name"`
	DocumentType        string      `json:"document_type"`
	FilePath            string      `json:"file_path"`
	CertificateNumber   string      `json:"certificate_number"`
	IssueDate           string      `json:"issue_date"`
	ExpiryDate          string      `json:"expiry_date"`
	OCRRawText          string      `json:"ocr_raw_text"`
	InspectorName       string      `json:"inspector_name"`
	InspectionDate      string      `json:"inspection_date"`
	ComplianceStatus    string      `json:"compliance_status"`
	ViolationDetails    string      `json:"violation_details"`
	RiskLevel           string      `json:"risk_level"`
	CorrectiveAction    string      `json:"corrective_action"`
	DueDate             string      `json:"due_date"`
	RegulatoryReference string      `json:"regulatory_reference"`
	OCRDataJSON         interface{} `json:"ocr_data_json,omitempty"`
	WorkflowStatus      string      `json:"workflow_status"`
	UploadedBy          int         `json:"uploaded_by"`
	UploadedByName      string      `json:"uploaded_by_name"`
	ReviewedBy          *int        `json:"reviewed_by,omitempty"`
	ReviewedByName      string      `json:"reviewed_by_name,omitempty"`
	ReviewedAt          *string     `json:"reviewed_at,omitempty"`
	ApprovedBy          *int        `json:"approved_by,omitempty"`
	ApprovedByName      string      `json:"approved_by_name,omitempty"`
	ApprovedAt          *string     `json:"approved_at,omitempty"`
	VerifiedBy          *int        `json:"verified_by,omitempty"`
	VerifiedByName      string      `json:"verified_by_name,omitempty"`
	VerifiedAt          *string     `json:"verified_at,omitempty"`
	Status              string      `json:"status"`
	CreatedAt           time.Time   `json:"created_at"`
}

// ListDocuments fetches uploaded certificates and checks for expiry status.
func (dc *DocumentsController) ListDocuments(c *gin.Context) {
	// Update expiry statuses based on current date
	_, _ = database.DB.Exec(`
		UPDATE documents 
		SET status = CASE 
			WHEN expiry_date < CURRENT_DATE THEN 'EXPIRED'
			WHEN expiry_date <= CURRENT_DATE + INTERVAL '30 day' THEN 'EXPIRING_SOON'
			ELSE 'VALID'
		END
		WHERE expiry_date IS NOT NULL`)

	query := `
		SELECT d.id, d.mine_id, COALESCE(m.mine_name, ''), COALESCE(d.mine_code, COALESCE(m.mine_code, '')),
		       d.contractor_id, COALESCE(ct.company_name, ''),
		       COALESCE(d.document_type, 'Statutory Certificate'), d.file_path, 
		       COALESCE(d.certificate_number, ''), d.issue_date, d.expiry_date, COALESCE(d.ocr_raw_text, ''),
		       COALESCE(d.inspector_name, ''), d.inspection_date, COALESCE(d.compliance_status, 'COMPLIANT'),
		       COALESCE(d.violation_details, ''), COALESCE(d.risk_level, 'LOW'), COALESCE(d.corrective_action, ''),
		       d.due_date, COALESCE(d.regulatory_reference, ''), d.ocr_data_json,
		       COALESCE(d.workflow_status, 'PENDING_REVIEW'),
		       d.uploaded_by, u.full_name AS uploaded_by_name,
		       d.reviewed_by, COALESCE(ur.full_name, ''), d.reviewed_at,
		       d.approved_by, COALESCE(ua.full_name, ''), d.approved_at,
		       d.verified_by, COALESCE(uv.full_name, ''), d.verified_at,
		       COALESCE(d.status, 'VALID'), d.created_at
		FROM documents d
		LEFT JOIN mines m ON m.id = d.mine_id
		LEFT JOIN contractors ct ON ct.id = d.contractor_id
		LEFT JOIN users u ON u.id = d.uploaded_by
		LEFT JOIN users ur ON ur.id = d.reviewed_by
		LEFT JOIN users ua ON ua.id = d.approved_by
		LEFT JOIN users uv ON uv.id = d.verified_by
		WHERE 1=1`
	args := []interface{}{}

	if mineID := c.Query("mine_id"); mineID != "" {
		query += " AND d.mine_id = ?"
		args = append(args, mineID)
	}
	if contractorID := c.Query("contractor_id"); contractorID != "" {
		query += " AND d.contractor_id = ?"
		args = append(args, contractorID)
	}
	if status := c.Query("status"); status != "" {
		query += " AND d.status = ?"
		args = append(args, status)
	}
	if workflow := c.Query("workflow_status"); workflow != "" {
		query += " AND d.workflow_status = ?"
		args = append(args, workflow)
	}
	if docType := c.Query("document_type"); docType != "" {
		query += " AND d.document_type = ?"
		args = append(args, docType)
	}

	query += " ORDER BY d.created_at DESC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query documents", err.Error())
		return
	}
	defer rows.Close()

	list := []documentItem{}
	for rows.Next() {
		var d documentItem
		var mineID, contractorID sql.NullInt64
		var reviewedBy, approvedBy, verifiedBy sql.NullInt64
		var reviewedAt, approvedAt, verifiedAt sql.NullTime
		var issueVal, expiryVal, inspectVal, dueVal []uint8
		var ocrJSONVal sql.NullString

		err := rows.Scan(
			&d.ID, &mineID, &d.MineName, &d.MineCode,
			&contractorID, &d.ContractorName,
			&d.DocumentType, &d.FilePath,
			&d.CertificateNumber, &issueVal, &expiryVal, &d.OCRRawText,
			&d.InspectorName, &inspectVal, &d.ComplianceStatus,
			&d.ViolationDetails, &d.RiskLevel, &d.CorrectiveAction,
			&dueVal, &d.RegulatoryReference, &ocrJSONVal,
			&d.WorkflowStatus,
			&d.UploadedBy, &d.UploadedByName,
			&reviewedBy, &d.ReviewedByName, &reviewedAt,
			&approvedBy, &d.ApprovedByName, &approvedAt,
			&verifiedBy, &d.VerifiedByName, &verifiedAt,
			&d.Status, &d.CreatedAt,
		)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse document record", err.Error())
			return
		}

		if mineID.Valid {
			val := int(mineID.Int64)
			d.MineID = &val
		}
		if contractorID.Valid {
			cid := int(contractorID.Int64)
			d.ContractorID = &cid
		}
		if reviewedBy.Valid {
			rb := int(reviewedBy.Int64)
			d.ReviewedBy = &rb
		}
		if approvedBy.Valid {
			ab := int(approvedBy.Int64)
			d.ApprovedBy = &ab
		}
		if verifiedBy.Valid {
			vb := int(verifiedBy.Int64)
			d.VerifiedBy = &vb
		}
		if reviewedAt.Valid {
			t := reviewedAt.Time.Format("2006-01-02 15:04")
			d.ReviewedAt = &t
		}
		if approvedAt.Valid {
			t := approvedAt.Time.Format("2006-01-02 15:04")
			d.ApprovedAt = &t
		}
		if verifiedAt.Valid {
			t := verifiedAt.Time.Format("2006-01-02 15:04")
			d.VerifiedAt = &t
		}
		if issueVal != nil {
			d.IssueDate = string(issueVal)
		}
		if expiryVal != nil {
			d.ExpiryDate = string(expiryVal)
		}
		if inspectVal != nil {
			d.InspectionDate = string(inspectVal)
		}
		if dueVal != nil {
			d.DueDate = string(dueVal)
		}
		if ocrJSONVal.Valid && ocrJSONVal.String != "" {
			var parsed interface{}
			if err := json.Unmarshal([]byte(ocrJSONVal.String), &parsed); err == nil {
				d.OCRDataJSON = parsed
			}
		}

		list = append(list, d)
	}

	utils.Success(c, http.StatusOK, "Documents fetched successfully", list)
}

// UploadDocument receives certificate upload, triggers Flask OCR for all 13 fields, and saves to database.
func (dc *DocumentsController) UploadDocument(c *gin.Context) {
	userID, _ := c.Get(middleware.CtxUserID)

	mineIDStr := c.PostForm("mine_id")
	contractorIDStr := c.PostForm("contractor_id")

	// Save file upload
	file, header, err := c.Request.FormFile("document")
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "No document file uploaded", err.Error())
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("doc_%d_%d%s", userID, time.Now().Unix(), ext)
	uploadDir := "./uploads"
	_ = os.MkdirAll(uploadDir, os.ModePerm)

	localFilePath := filepath.Join(uploadDir, filename)
	webFilePath := "/uploads/" + filename // This ensures the browser can find the file

	out, err := os.Create(localFilePath)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to save document file", err.Error())
		return
	}
	defer out.Close()
	_, _ = io.Copy(out, file)

	// Call Flask OCR service using multipart upload so the file is sent directly to
	// the AI service instead of relying on a shared filesystem path between containers.
	ocrURL := fmt.Sprintf("%s/api/ocr/extract", dc.Cfg.AIServiceURL)
	var certNumber, docType, issueDate, expiryDate, rawText string
	var mineNameMatched, mineCode, inspectorName, inspectionDate string
	var complianceStatus, violationDetails, riskLevel, correctiveAction, dueDate, regulatoryRef string
	var fullOCRJSON []byte

	client := &http.Client{Timeout: 30 * time.Second}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fileBytes, _ := os.ReadFile(localFilePath)
	part, err := writer.CreateFormFile("document", filepath.Base(localFilePath))
	if err == nil {
		_, _ = part.Write(fileBytes)
	}
	_ = writer.Close()

	req, reqErr := http.NewRequest(http.MethodPost, ocrURL, body)
	if reqErr == nil {
		req.Header.Set("Content-Type", writer.FormDataContentType())
	}

	var resp *http.Response
	var postErr error
	if reqErr == nil {
		resp, postErr = client.Do(req)
	}
	if postErr != nil && strings.Contains(ocrURL, "localhost") {
		fallbackURL := strings.Replace(ocrURL, "localhost", "127.0.0.1", 1)
		body2 := &bytes.Buffer{}
		writer2 := multipart.NewWriter(body2)
		part2, _ := writer2.CreateFormFile("document", filepath.Base(localFilePath))
		_, _ = part2.Write(fileBytes)
		_ = writer2.Close()
		req2, reqErr2 := http.NewRequest(http.MethodPost, fallbackURL, body2)
		if reqErr2 == nil {
			req2.Header.Set("Content-Type", writer2.FormDataContentType())
			resp, postErr = client.Do(req2)
		}
	}

	if postErr != nil {
		log.Printf("[DocumentsController] OCR request to %s error: %v", ocrURL, postErr)
	} else if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)

			type ocrData struct {
				CertificateNumber   string      `json:"certificate_number"`
				MineName            string      `json:"mine_name"`
				MineCode            string      `json:"mine_code"`
				DocumentType        string      `json:"document_type"`
				InspectionDate      string      `json:"inspection_date"`
				InspectorName       string      `json:"inspector_name"`
				ComplianceStatus    string      `json:"compliance_status"`
				ViolationDetails    string      `json:"violation_details"`
				RiskLevel           string      `json:"risk_level"`
				CorrectiveAction    string      `json:"corrective_action"`
				DueDate             string      `json:"due_date"`
				IssueDate           string      `json:"issue_date"`
				ExpiryDate          string      `json:"expiry_date"`
				RegulatoryReference string      `json:"regulatory_reference"`
				OCRRawText          string      `json:"ocr_raw_text"`
				FieldCategories     interface{} `json:"field_categories,omitempty"`
			}
			type ocrResponse struct {
				Success bool    `json:"success"`
				Message string  `json:"message"`
				Data    ocrData `json:"data"`
			}

			var ocrRes ocrResponse
			if err := json.Unmarshal(bodyBytes, &ocrRes); err == nil && ocrRes.Success {
				d := ocrRes.Data
				certNumber = d.CertificateNumber
				mineNameMatched = d.MineName
				mineCode = d.MineCode
				docType = d.DocumentType
				inspectionDate = d.InspectionDate
				inspectorName = d.InspectorName
				complianceStatus = d.ComplianceStatus
				violationDetails = d.ViolationDetails
				riskLevel = d.RiskLevel
				correctiveAction = d.CorrectiveAction
				dueDate = d.DueDate
				issueDate = d.IssueDate
				expiryDate = d.ExpiryDate
				regulatoryRef = d.RegulatoryReference
				rawText = d.OCRRawText
				if d.FieldCategories != nil {
					fullOCRJSON, _ = json.Marshal(map[string]interface{}{
						"certificate_number":   d.CertificateNumber,
						"mine_name":            d.MineName,
						"mine_code":            d.MineCode,
						"document_type":        d.DocumentType,
						"inspection_date":      d.InspectionDate,
						"inspector_name":       d.InspectorName,
						"compliance_status":    d.ComplianceStatus,
						"violation_details":    d.ViolationDetails,
						"risk_level":           d.RiskLevel,
						"corrective_action":    d.CorrectiveAction,
						"due_date":             d.DueDate,
						"issue_date":           d.IssueDate,
						"expiry_date":          d.ExpiryDate,
						"regulatory_reference": d.RegulatoryReference,
						"ocr_raw_text":         d.OCRRawText,
						"field_categories":     d.FieldCategories,
					})
				} else {
					fullOCRJSON, _ = json.Marshal(d)
				}
			}
		}
	}

	// Fallback values if OCR service did not return useful mine/compliance data.
	// Never invent a valid mine record for blank or unrelated images.
	if certNumber == "" || strings.TrimSpace(mineNameMatched) == "" || strings.TrimSpace(docType) == "" || strings.TrimSpace(rawText) == "" {
		certNumber = "UNSPECIFIED"
		docType = "Unidentified Document"
		mineNameMatched = "Not detected"
		mineCode = "N/A"
		inspectorName = ""
		inspectionDate = ""
		complianceStatus = "UNKNOWN"
		riskLevel = "LOW"
		regulatoryRef = ""
		issueDate = ""
		expiryDate = ""
		dueDate = ""
		if rawText == "" {
			rawText = "No mining compliance details detected in the uploaded image."
		}
	}

	var mineIDVal interface{} = nil
	if mineIDStr != "" {
		if mid, err := strconv.Atoi(mineIDStr); err == nil {
			mineIDVal = mid
		}
	} else {
		var mid int
		err := database.DB.QueryRow(`SELECT id FROM mines WHERE mine_name LIKE ?`, "%"+mineNameMatched+"%").Scan(&mid)
		if err == nil {
			mineIDVal = mid
		}
	}

	var contractorIDVal interface{} = nil
	if contractorIDStr != "" {
		if cid, err := strconv.Atoi(contractorIDStr); err == nil {
			contractorIDVal = cid
		}
	}

	// Expiry calculation
	status := "VALID"
	if expiryDate != "" {
		if expiryTime, err := time.Parse("2006-01-02", expiryDate); err == nil {
			if expiryTime.Before(time.Now()) {
				status = "EXPIRED"
			} else if expiryTime.Before(time.Now().AddDate(0, 0, 30)) {
				status = "EXPIRING_SOON"
			}
		}
	}

	var issueDateDB, expiryDateDB, inspectionDateDB, dueDateDB interface{} = nil, nil, nil, nil
	if strings.TrimSpace(issueDate) != "" {
		issueDateDB = issueDate
	}
	if strings.TrimSpace(expiryDate) != "" {
		expiryDateDB = expiryDate
	}
	if strings.TrimSpace(inspectionDate) != "" {
		inspectionDateDB = inspectionDate
	}
	if strings.TrimSpace(dueDate) != "" {
		dueDateDB = dueDate
	}

	var newID int64
	err = database.DB.QueryRow(`
		INSERT INTO documents (
			mine_id, contractor_id, document_type, file_path, certificate_number,
			issue_date, expiry_date, ocr_raw_text, uploaded_by, status,
			mine_code, inspector_name, inspection_date, compliance_status,
			violation_details, risk_level, corrective_action, due_date,
			regulatory_reference, ocr_data_json, workflow_status
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING_REVIEW')
		RETURNING id`,
		mineIDVal, contractorIDVal, docType, webFilePath, certNumber,
		issueDateDB, expiryDateDB, rawText, userID, status,
		mineCode, inspectorName, inspectionDateDB, complianceStatus,
		violationDetails, riskLevel, correctiveAction, dueDateDB,
		regulatoryRef, string(fullOCRJSON)).Scan(&newID)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to save document to database", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "DOCUMENT_UPLOADED", "DOCUMENTS", strconv.FormatInt(newID, 10),
		map[string]interface{}{
			"document_type":      docType,
			"certificate_number": certNumber,
			"mine_name":          mineNameMatched,
			"compliance_status":  complianceStatus,
			"risk_level":         riskLevel,
		}, c.ClientIP())

	// Warning notifications if expired
	if status == "EXPIRED" || status == "EXPIRING_SOON" {
		createDocumentExpiryNotification(certNumber, docType, expiryDate, status)
	}

	utils.Success(c, http.StatusCreated, "Document uploaded and OCR parsed successfully", gin.H{
		"id":                   newID,
		"certificate_number":   certNumber,
		"document_type":        docType,
		"mine_name":            mineNameMatched,
		"mine_code":            mineCode,
		"inspector_name":       inspectorName,
		"compliance_status":    complianceStatus,
		"risk_level":           riskLevel,
		"violation_details":    violationDetails,
		"corrective_action":    correctiveAction,
		"due_date":             dueDate,
		"regulatory_reference": regulatoryRef,
		"issue_date":           issueDate,
		"expiry_date":          expiryDate,
		"status":               status,
		"workflow_status":      "PENDING_REVIEW",
	})
}

// ReviewDocument — Safety Officer / Mine Manager / Super Admin reviews and confirms OCR findings.
func (dc *DocumentsController) ReviewDocument(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid document ID", err.Error())
		return
	}

	var req struct {
		ComplianceStatus string `json:"compliance_status"`
		ViolationDetails string `json:"violation_details"`
		RiskLevel        string `json:"risk_level"`
		CorrectiveAction string `json:"corrective_action"`
		DueDate          string `json:"due_date"`
		ReviewNotes      string `json:"review_notes"`
	}
	_ = c.ShouldBindJSON(&req)

	if req.ComplianceStatus == "" {
		req.ComplianceStatus = "COMPLIANT"
	}
	if req.RiskLevel == "" {
		req.RiskLevel = "LOW"
	}

	userID, _ := c.Get(middleware.CtxUserID)

	_, err = database.DB.Exec(`
		UPDATE documents
		SET workflow_status='REVIEWED',
		    compliance_status=?,
		    violation_details=?,
		    risk_level=?,
		    corrective_action=?,
		    due_date=?,
		    reviewed_by=?,
		    reviewed_at=NOW()
		WHERE id=?`,
		req.ComplianceStatus, req.ViolationDetails, req.RiskLevel, req.CorrectiveAction, req.DueDate, userID, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update review status", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "DOCUMENT_REVIEWED", "DOCUMENTS", strconv.Itoa(id),
		map[string]interface{}{"compliance_status": req.ComplianceStatus, "risk_level": req.RiskLevel, "notes": req.ReviewNotes}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Document findings reviewed successfully", gin.H{"workflow_status": "REVIEWED"})
}

// ApproveDocument — Mine Manager / Super Admin approves document for compliance tracking.
func (dc *DocumentsController) ApproveDocument(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid document ID", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	_, err = database.DB.Exec(`
		UPDATE documents
		SET workflow_status='APPROVED',
		    approved_by=?,
		    approved_at=NOW()
		WHERE id=?`, userID, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to approve document", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "DOCUMENT_APPROVED", "DOCUMENTS", strconv.Itoa(id),
		map[string]interface{}{"workflow_status": "APPROVED"}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Document approved successfully", gin.H{"workflow_status": "APPROVED"})
}

// RegulatoryVerifyDocument — Regulatory Officer / Super Admin performs statutory verification or flags violation.
func (dc *DocumentsController) RegulatoryVerifyDocument(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid document ID", err.Error())
		return
	}

	var req struct {
		Action            string `json:"action"` // VERIFY or FLAG_VIOLATION
		VerificationNotes string `json:"verification_notes"`
	}
	_ = c.ShouldBindJSON(&req)

	userID, _ := c.Get(middleware.CtxUserID)

	targetStatus := "REGULATORY_VERIFIED"
	auditEvent := "DOCUMENT_REGULATORY_VERIFIED"
	if req.Action == "FLAG_VIOLATION" {
		targetStatus = "VIOLATION_FLAGGED"
		auditEvent = "DOCUMENT_VIOLATION_FLAGGED"
	}

	_, err = database.DB.Exec(`
		UPDATE documents
		SET workflow_status=?,
		    verified_by=?,
		    verified_at=NOW()
		WHERE id=?`, targetStatus, userID, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update regulatory verification", err.Error())
		return
	}

	utils.LogAudit(userID.(int), auditEvent, "DOCUMENTS", strconv.Itoa(id),
		map[string]interface{}{"workflow_status": targetStatus, "notes": req.VerificationNotes}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Regulatory verification recorded", gin.H{"workflow_status": targetStatus})
}

// FlagViolationFromDocument creates a live violation & corrective action directly from OCR findings.
func (dc *DocumentsController) FlagViolationFromDocument(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid document ID", err.Error())
		return
	}

	var req struct {
		Title            string `json:"title" binding:"required"`
		Description      string `json:"description" binding:"required"`
		Severity         string `json:"severity"`
		CorrectiveAction string `json:"corrective_action"`
		DueDate          string `json:"due_date"`
		ResponsibleDept  string `json:"responsible_dept"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request", err.Error())
		return
	}

	if req.Severity == "" {
		req.Severity = "HIGH"
	}
	if req.DueDate == "" {
		req.DueDate = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	}

	userID, _ := c.Get(middleware.CtxUserID)

	// Fetch document details
	var mineID sql.NullInt64
	var certNumber, docType string
	_ = database.DB.QueryRow(`SELECT mine_id, certificate_number, document_type FROM documents WHERE id=?`, id).Scan(&mineID, &certNumber, &docType)

	var midVal interface{} = 1
	if mineID.Valid {
		midVal = int(mineID.Int64)
	}

	// Insert into violations. violation_code is NOT NULL/UNIQUE in the schema,
	// so it must be generated — same pattern used in inspections_controller.go's
	// auto-violation logic. category_id defaults to 1 (Safety) since the OCR
	// flow doesn't currently collect a category from the reviewer.
	year := time.Now().Format("2006")
	var seq int
	_ = database.DB.QueryRow(`SELECT COUNT(*) + 1 FROM violations`).Scan(&seq)
	violationCode := fmt.Sprintf("VIO-%s-%04d", year, seq)

	var vID int64
	err = database.DB.QueryRow(`
		INSERT INTO violations (violation_code, mine_id, category_id, description, severity, reported_by, deadline, status)
		VALUES (?, ?, 1, ?, ?, ?, ?, 'OPEN')
		RETURNING id`,
		violationCode, midVal, fmt.Sprintf("%s: %s (Ref: %s / %s)", req.Title, req.Description, certNumber, docType),
		req.Severity, userID, req.DueDate).Scan(&vID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to create violation", err.Error())
		return
	}

	// Insert corrective action. corrective_actions has no action_plan/due_date
	// columns — the schema uses action_description/deadline — and its status
	// CHECK constraint doesn't allow 'PENDING', only ASSIGNED/SUBMITTED/
	// VERIFIED/CLOSED/OVERDUE.
	if req.CorrectiveAction != "" {
		_, _ = database.DB.Exec(`
			INSERT INTO corrective_actions (violation_id, assigned_to, action_description, deadline, status)
			VALUES (?, ?, ?, ?, 'ASSIGNED')`,
			vID, userID, req.CorrectiveAction, req.DueDate)
	}

	// Update document status
	_, _ = database.DB.Exec(`UPDATE documents SET workflow_status='VIOLATION_FLAGGED' WHERE id=?`, id)

	utils.LogAudit(userID.(int), "VIOLATION_CREATED_FROM_DOCUMENT", "VIOLATIONS", strconv.FormatInt(vID, 10),
		map[string]interface{}{"document_id": id, "certificate_number": certNumber, "severity": req.Severity}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Violation and corrective action created successfully", gin.H{
		"violation_id": vID,
		"document_id":  id,
	})
}

// UpdateDocument updates an existing document record (Admin & Safety Officer & Mine Manager).
func (dc *DocumentsController) UpdateDocument(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid document ID", err.Error())
		return
	}

	var req struct {
		MineID              *int   `json:"mine_id"`
		MineCode            string `json:"mine_code"`
		ContractorID        *int   `json:"contractor_id"`
		DocumentType        string `json:"document_type" binding:"required"`
		CertificateNumber   string `json:"certificate_number" binding:"required"`
		InspectorName       string `json:"inspector_name"`
		InspectionDate      string `json:"inspection_date"`
		ComplianceStatus    string `json:"compliance_status"`
		ViolationDetails    string `json:"violation_details"`
		RiskLevel           string `json:"risk_level"`
		CorrectiveAction    string `json:"corrective_action"`
		DueDate             string `json:"due_date"`
		RegulatoryReference string `json:"regulatory_reference"`
		IssueDate           string `json:"issue_date"`
		ExpiryDate          string `json:"expiry_date"`
		Status              string `json:"status"`
		OCRRawText          string `json:"ocr_raw_text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request", err.Error())
		return
	}

	if req.Status == "" {
		req.Status = "VALID"
	}

	userID, _ := c.Get(middleware.CtxUserID)

	var issueVal, expiryVal, inspectVal, dueVal interface{} = nil, nil, nil, nil
	if req.IssueDate != "" {
		issueVal = req.IssueDate
	}
	if req.ExpiryDate != "" {
		expiryVal = req.ExpiryDate
	}
	if req.InspectionDate != "" {
		inspectVal = req.InspectionDate
	}
	if req.DueDate != "" {
		dueVal = req.DueDate
	}

	_, err = database.DB.Exec(`
		UPDATE documents
		SET mine_id=?, mine_code=?, contractor_id=?, document_type=?, certificate_number=?,
		    inspector_name=?, inspection_date=?, compliance_status=?, violation_details=?,
		    risk_level=?, corrective_action=?, due_date=?, regulatory_reference=?,
		    issue_date=?, expiry_date=?, status=?, ocr_raw_text=?
		WHERE id=?`,
		req.MineID, req.MineCode, req.ContractorID, req.DocumentType, req.CertificateNumber,
		req.InspectorName, inspectVal, req.ComplianceStatus, req.ViolationDetails,
		req.RiskLevel, req.CorrectiveAction, dueVal, req.RegulatoryReference,
		issueVal, expiryVal, req.Status, req.OCRRawText, id)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to update document", err.Error())
		return
	}

	utils.LogAudit(userID.(int), "DOCUMENT_UPDATED", "DOCUMENTS", strconv.Itoa(id),
		map[string]interface{}{"certificate_number": req.CertificateNumber, "document_type": req.DocumentType}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Document updated successfully", nil)
}

// DeleteDocument permanently deletes a document record and removes the file (Admin only).
func (dc *DocumentsController) DeleteDocument(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid document ID", err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)

	var filePath, certNumber string
	_ = database.DB.QueryRow(`SELECT file_path, certificate_number FROM documents WHERE id=?`, id).Scan(&filePath, &certNumber)

	_, err = database.DB.Exec(`DELETE FROM documents WHERE id=?`, id)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to delete document", err.Error())
		return
	}

	if filePath != "" {
		_ = os.Remove(filePath)
	}

	utils.LogAudit(userID.(int), "DOCUMENT_DELETED", "DOCUMENTS", strconv.Itoa(id),
		map[string]interface{}{"certificate_number": certNumber, "file_path": filePath}, c.ClientIP())

	utils.Success(c, http.StatusOK, "Document deleted successfully", nil)
}

// createDocumentExpiryNotification notifies administrators and safety officers.
func createDocumentExpiryNotification(certNumber, docType, expiryDate string, status string) {
	rows, err := database.DB.Query(`
		SELECT u.id FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE r.role_key IN ('SAFETY_OFFICER', 'SUPER_ADMIN') AND u.status = 'ACTIVE'`)
	if err != nil {
		return
	}
	defer rows.Close()

	title := fmt.Sprintf("ALERT: Document %s %s", certNumber, status)
	message := fmt.Sprintf("The %s with certificate number %s expires on %s. Current status: %s. Action required.", docType, certNumber, expiryDate, status)

	for rows.Next() {
		var uid int
		if err := rows.Scan(&uid); err == nil {
			_, _ = database.DB.Exec(`
				INSERT INTO notifications (recipient_id, title, message, severity, type, is_read)
				VALUES (?, ?, ?, 'WARNING', 'DOCUMENT_EXPIRY', FALSE)`,
				uid, title, message)
		}
	}
}
