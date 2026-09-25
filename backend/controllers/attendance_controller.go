package controllers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"coal-governance-backend/database"
	"coal-governance-backend/middleware"
	"coal-governance-backend/models"
	"coal-governance-backend/utils"
)

type AttendanceController struct{}

func NewAttendanceController() *AttendanceController {
	return &AttendanceController{}
}

type markAttendanceRequest struct {
	MineID        int      `json:"mine_id" binding:"required"`
	WorkerID      *int     `json:"worker_id"`
	RecordDate    string   `json:"record_date"` // YYYY-MM-DD
	Status        string   `json:"status"`      // PRESENT, ABSENT, LEAVE, HALF_DAY
	Shift         string   `json:"shift"`       // GENERAL, SHIFT_1, SHIFT_2, SHIFT_3
	OvertimeHours float64  `json:"overtime_hours"`
	PresentCount  *int     `json:"present_count"`
	TotalCount    *int     `json:"total_count"`
	WorkerIDs     []int    `json:"worker_ids"`  // for batch marking
}

// MarkAttendance handles individual or batch worker attendance marking.
func (ac *AttendanceController) MarkAttendance(c *gin.Context) {
	var req markAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	if req.RecordDate == "" {
		req.RecordDate = time.Now().Format("2006-01-02")
	}
	if req.Status == "" {
		req.Status = "PRESENT"
	}
	if req.Shift == "" {
		req.Shift = "GENERAL"
	}

	// 1. Batch marking for multiple worker IDs
	if len(req.WorkerIDs) > 0 {
		tx, err := database.DB.Begin()
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to begin transaction", err.Error())
			return
		}
		defer tx.Rollback()

		markedCount := 0
		for _, wID := range req.WorkerIDs {
			// Postgres has no ON DUPLICATE KEY UPDATE (MySQL-only) — the
			// equivalent is ON CONFLICT ... DO UPDATE, which targets a real
			// unique constraint. See the accompanying migration that adds
			// UNIQUE (mine_id, worker_id, record_date) on attendance.
			_, err = tx.Exec(`
				INSERT INTO attendance (mine_id, worker_id, record_date, status, shift, overtime_hours, marked_by)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (mine_id, worker_id, record_date) DO UPDATE SET
					status = EXCLUDED.status,
					shift = EXCLUDED.shift,
					overtime_hours = EXCLUDED.overtime_hours,
					marked_by = EXCLUDED.marked_by`,
				req.MineID, wID, req.RecordDate, req.Status, req.Shift, req.OvertimeHours, userID)
			if err != nil {
				utils.Fail(c, http.StatusInternalServerError, "Failed to mark batch attendance", err.Error())
				return
			}
			markedCount++
		}

		if err := tx.Commit(); err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to commit batch attendance", err.Error())
			return
		}

		utils.LogAudit(userID, "ATTENDANCE_BATCH_MARKED", "ATTENDANCE", strconv.Itoa(req.MineID),
			map[string]interface{}{"mine_id": req.MineID, "date": req.RecordDate, "count": markedCount, "status": req.Status}, c.ClientIP())

		utils.Success(c, http.StatusOK, "Batch attendance marked successfully", gin.H{"count": markedCount})
		return
	}

	// 2. Individual worker or aggregate mine total
	var newID int64
	err := database.DB.QueryRow(`
		INSERT INTO attendance (mine_id, worker_id, record_date, status, shift, overtime_hours, present_count, total_count, marked_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		req.MineID, req.WorkerID, req.RecordDate, req.Status, req.Shift, req.OvertimeHours, req.PresentCount, req.TotalCount, userID).Scan(&newID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to mark attendance", err.Error())
		return
	}

	utils.LogAudit(userID, "ATTENDANCE_MARKED", "ATTENDANCE", strconv.FormatInt(newID, 10),
		map[string]interface{}{"mine_id": req.MineID, "worker_id": req.WorkerID, "date": req.RecordDate, "status": req.Status}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Attendance recorded successfully", gin.H{"id": newID})
}

// ListAttendance lists attendance records with flexible filtering.
func (ac *AttendanceController) ListAttendance(c *gin.Context) {
	query := `
		SELECT a.id, a.mine_id, m.mine_name, a.worker_id, 
			   COALESCE(w.worker_code, ''), COALESCE(w.full_name, ''), COALESCE(w.designation, ''),
			   w.contractor_id, COALESCE(ct.company_name, ''),
			   a.record_date, a.status, a.shift, a.overtime_hours,
			   a.present_count, a.total_count, a.marked_by, COALESCE(u.full_name, ''), a.created_at
		FROM attendance a
		JOIN mines m ON m.id = a.mine_id
		LEFT JOIN workers w ON w.id = a.worker_id
		LEFT JOIN contractors ct ON ct.id = w.contractor_id
		LEFT JOIN users u ON u.id = a.marked_by
		WHERE 1=1`
	args := []interface{}{}

	if mineID := c.Query("mine_id"); mineID != "" {
		query += " AND a.mine_id = ?"
		args = append(args, mineID)
	}
	if workerID := c.Query("worker_id"); workerID != "" {
		query += " AND a.worker_id = ?"
		args = append(args, workerID)
	}
	if contractorID := c.Query("contractor_id"); contractorID != "" {
		query += " AND w.contractor_id = ?"
		args = append(args, contractorID)
	}
	if status := c.Query("status"); status != "" {
		query += " AND a.status = ?"
		args = append(args, status)
	}
	if date := c.Query("date"); date != "" {
		query += " AND a.record_date = ?"
		args = append(args, date)
	}
	if startDate := c.Query("start_date"); startDate != "" {
		query += " AND a.record_date >= ?"
		args = append(args, startDate)
	}
	if endDate := c.Query("end_date"); endDate != "" {
		query += " AND a.record_date <= ?"
		args = append(args, endDate)
	}

	query += " ORDER BY a.record_date DESC, a.id DESC LIMIT 300"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to fetch attendance records", err.Error())
		return
	}
	defer rows.Close()

	records := []models.Attendance{}
	for rows.Next() {
		var a models.Attendance
		var workerID, contractorID, markedBy sql.NullInt64
		var presentCount, totalCount sql.NullInt64
		var recordDate []uint8

		err := rows.Scan(
			&a.ID, &a.MineID, &a.MineName, &workerID,
			&a.WorkerCode, &a.WorkerName, &a.Designation,
			&contractorID, &a.ContractorName,
			&recordDate, &a.Status, &a.Shift, &a.OvertimeHours,
			&presentCount, &totalCount, &markedBy, &a.MarkedByName, &a.CreatedAt,
		)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse attendance records", err.Error())
			return
		}

		a.RecordDate = string(recordDate)
		if workerID.Valid {
			wid := int(workerID.Int64)
			a.WorkerID = &wid
		}
		if contractorID.Valid {
			cid := int(contractorID.Int64)
			a.ContractorID = &cid
		}
		if markedBy.Valid {
			mid := int(markedBy.Int64)
			a.MarkedBy = &mid
		}
		if presentCount.Valid {
			pc := int(presentCount.Int64)
			a.PresentCount = &pc
		}
		if totalCount.Valid {
			tc := int(totalCount.Int64)
			a.TotalCount = &tc
		}

		records = append(records, a)
	}

	utils.Success(c, http.StatusOK, "Attendance records fetched successfully", records)
}

// GetAttendanceReport provides aggregated attendance metrics and compliance KPIs.
func (ac *AttendanceController) GetAttendanceReport(c *gin.Context) {
	mineID := c.Query("mine_id")
	contractorID := c.Query("contractor_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	// 1. Total Workers in scope
	var totalWorkers int
	workerQuery := `SELECT COUNT(*) FROM workers WHERE status = 'ACTIVE'`
	workerArgs := []interface{}{}
	if mineID != "" {
		workerQuery += " AND mine_id = ?"
		workerArgs = append(workerArgs, mineID)
	}
	if contractorID != "" {
		workerQuery += " AND contractor_id = ?"
		workerArgs = append(workerArgs, contractorID)
	}
	_ = database.DB.QueryRow(workerQuery, workerArgs...).Scan(&totalWorkers)

	// 2. Status counts (Present, Absent, Leave, Half Day)
	statusQuery := `
		SELECT a.status, COUNT(*)
		FROM attendance a
		LEFT JOIN workers w ON w.id = a.worker_id
		WHERE a.worker_id IS NOT NULL AND a.record_date BETWEEN ? AND ?`
	statusArgs := []interface{}{startDate, endDate}
	if mineID != "" {
		statusQuery += " AND a.mine_id = ?"
		statusArgs = append(statusArgs, mineID)
	}
	if contractorID != "" {
		statusQuery += " AND w.contractor_id = ?"
		statusArgs = append(statusArgs, contractorID)
	}
	statusQuery += " GROUP BY a.status"

	rows, err := database.DB.Query(statusQuery, statusArgs...)
	statusBreakdown := map[string]int{"PRESENT": 0, "ABSENT": 0, "LEAVE": 0, "HALF_DAY": 0}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var st string
			var cnt int
			if err := rows.Scan(&st, &cnt); err == nil {
				statusBreakdown[st] = cnt
			}
		}
	}

	// 3. Overall rate
	totalLogged := statusBreakdown["PRESENT"] + statusBreakdown["ABSENT"] + statusBreakdown["LEAVE"] + statusBreakdown["HALF_DAY"]
	attendanceRate := 100.0
	if totalLogged > 0 {
		attendanceRate = (float64(statusBreakdown["PRESENT"]) + float64(statusBreakdown["HALF_DAY"])*0.5) / float64(totalLogged) * 100
	}

	// 4. Overtime Hours Sum
	var totalOvertime float64
	otQuery := `
		SELECT COALESCE(SUM(a.overtime_hours), 0.0)
		FROM attendance a
		LEFT JOIN workers w ON w.id = a.worker_id
		WHERE a.record_date BETWEEN ? AND ?`
	otArgs := []interface{}{startDate, endDate}
	if mineID != "" {
		otQuery += " AND a.mine_id = ?"
		otArgs = append(otArgs, mineID)
	}
	if contractorID != "" {
		otQuery += " AND w.contractor_id = ?"
		otArgs = append(otArgs, contractorID)
	}
	_ = database.DB.QueryRow(otQuery, otArgs...).Scan(&totalOvertime)

	utils.Success(c, http.StatusOK, "Attendance report summary", gin.H{
		"total_workers":    totalWorkers,
		"total_records":    totalLogged,
		"status_breakdown": statusBreakdown,
		"attendance_rate":  attendanceRate,
		"total_overtime":   totalOvertime,
		"start_date":       startDate,
		"end_date":         endDate,
	})
}

// ListWorkers returns active workers for dropdowns and attendance tables.
func (ac *AttendanceController) ListWorkers(c *gin.Context) {
	query := `
		SELECT w.id, w.mine_id, m.mine_name, w.worker_code, w.full_name, w.designation,
			   w.department_id, COALESCE(d.dept_name, ''),
			   w.contractor_id, COALESCE(ct.company_name, ''),
			   w.status, w.created_at
		FROM workers w
		JOIN mines m ON m.id = w.mine_id
		LEFT JOIN departments d ON d.id = w.department_id
		LEFT JOIN contractors ct ON ct.id = w.contractor_id
		WHERE 1=1`
	args := []interface{}{}

	if mineID := c.Query("mine_id"); mineID != "" {
		query += " AND w.mine_id = ?"
		args = append(args, mineID)
	}
	if contractorID := c.Query("contractor_id"); contractorID != "" {
		query += " AND w.contractor_id = ?"
		args = append(args, contractorID)
	}
	if status := c.Query("status"); status != "" {
		query += " AND w.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY w.full_name ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to query workers", err.Error())
		return
	}
	defer rows.Close()

	workers := []models.Worker{}
	for rows.Next() {
		var w models.Worker
		var deptID, contID sql.NullInt64
		err := rows.Scan(
			&w.ID, &w.MineID, &w.MineName, &w.WorkerCode, &w.FullName, &w.Designation,
			&deptID, &w.DepartmentName, &contID, &w.ContractorName,
			&w.Status, &w.CreatedAt,
		)
		if err != nil {
			utils.Fail(c, http.StatusInternalServerError, "Failed to parse worker row", err.Error())
			return
		}
		if deptID.Valid {
			val := int(deptID.Int64)
			w.DepartmentID = &val
		}
		if contID.Valid {
			val := int(contID.Int64)
			w.ContractorID = &val
		}
		workers = append(workers, w)
	}

	utils.Success(c, http.StatusOK, "Workers fetched successfully", workers)
}

// CreateWorker registers a new worker or contractor personnel.
func (ac *AttendanceController) CreateWorker(c *gin.Context) {
	var req struct {
		MineID       int    `json:"mine_id" binding:"required"`
		WorkerCode   string `json:"worker_code" binding:"required"`
		FullName     string `json:"full_name" binding:"required"`
		Designation  string `json:"designation"`
		DepartmentID *int   `json:"department_id"`
		ContractorID *int   `json:"contractor_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	userIDVal, _ := c.Get(middleware.CtxUserID)
	userID := userIDVal.(int)

	var newID int64
	err := database.DB.QueryRow(`
		INSERT INTO workers (mine_id, worker_code, full_name, designation, department_id, contractor_id, status)
		VALUES (?, ?, ?, ?, ?, ?, 'ACTIVE') RETURNING id`,
		req.MineID, req.WorkerCode, req.FullName, req.Designation, req.DepartmentID, req.ContractorID).Scan(&newID)

	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "Failed to create worker (code may already exist)", err.Error())
		return
	}

	utils.LogAudit(userID, "WORKER_CREATED", "WORKERS", strconv.FormatInt(newID, 10),
		map[string]interface{}{"worker_code": req.WorkerCode, "full_name": req.FullName, "mine_id": req.MineID}, c.ClientIP())

	utils.Success(c, http.StatusCreated, "Worker created successfully", gin.H{"id": newID})
}