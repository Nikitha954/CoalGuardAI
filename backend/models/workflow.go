package models

import "time"

// Inspection represents a mine site inspection.
type Inspection struct {
	ID             int              `json:"id"`
	MineID         int              `json:"mine_id"`
	MineName       string           `json:"mine_name,omitempty"`
	InspectionType string           `json:"inspection_type"`
	InspectorID    int              `json:"inspector_id"`
	InspectorName  string           `json:"inspector_name,omitempty"`
	InspectionDate string           `json:"inspection_date"` // YYYY-MM-DD
	InspectionTime string           `json:"inspection_time"` // HH:MM:SS
	GPSLatitude    float64          `json:"gps_latitude"`
	GPSLongitude   float64          `json:"gps_longitude"`
	Remarks        string           `json:"remarks"`
	Status         string           `json:"status"` // DRAFT, SUBMITTED, REVIEWED, APPROVED
	Items          []InspectionItem `json:"items,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// InspectionItem represents check item result.
type InspectionItem struct {
	ID            int    `json:"id"`
	InspectionID  int    `json:"inspection_id"`
	ChecklistItem string `json:"checklist_item"`
	Result        string `json:"result"` // PASS, FAIL, NA
	Remarks       string `json:"remarks"`
}

// Observation represents qualitative findings during an inspection.
type Observation struct {
	ID           int       `json:"id"`
	InspectionID int       `json:"inspection_id"`
	CategoryID   *int      `json:"category_id"`
	CategoryName string    `json:"category_name,omitempty"`
	Observation  string    `json:"observation"`
	Severity     string    `json:"severity"` // LOW, MEDIUM, HIGH, CRITICAL
	EvidencePath string    `json:"evidence_path"`
	CreatedAt    time.Time `json:"created_at"`
}

// Violation represents an identified compliance breach.
type Violation struct {
	ID                int       `json:"id"`
	ViolationCode     string    `json:"violation_code"`
	MineID            int       `json:"mine_id"`
	MineName          string    `json:"mine_name,omitempty"`
	InspectionID      *int      `json:"inspection_id"`
	CategoryID        int       `json:"category_id"`
	CategoryName      string    `json:"category_name,omitempty"`
	Description       string    `json:"description"`
	Severity          string    `json:"severity"` // LOW, MEDIUM, HIGH, CRITICAL
	EvidencePath      string    `json:"evidence_path"`
	ReportedBy        int       `json:"reported_by"`
	ReportedByName    string    `json:"reported_by_name,omitempty"`
	ResponsiblePerson *int      `json:"responsible_person"`
	ResponsibleName   string    `json:"responsible_name,omitempty"`
	Deadline          string    `json:"deadline"` // YYYY-MM-DD
	Status            string    `json:"status"`   // OPEN, IN_PROGRESS, RESOLVED, VERIFIED, CLOSED, OVERDUE
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CorrectiveAction represents a plan to remedy a violation.
type CorrectiveAction struct {
	ID                int        `json:"id"`
	ViolationID       int        `json:"violation_id"`
	ViolationCode     string     `json:"violation_code,omitempty"`
	ViolationDesc     string     `json:"violation_desc,omitempty"`
	MineName          string     `json:"mine_name,omitempty"`
	Severity          string     `json:"severity,omitempty"`
	AssignedTo        int        `json:"assigned_to"`
	AssignedToName    string     `json:"assigned_to_name,omitempty"`
	ActionDescription string     `json:"action_description"`
	Deadline          string     `json:"deadline"` // YYYY-MM-DD
	SubmittedAt       *time.Time `json:"submitted_at,omitempty"`
	VerifiedBy        *int       `json:"verified_by,omitempty"`
	VerifiedByName    string     `json:"verified_by_name,omitempty"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	EscalationLevel   int        `json:"escalation_level"`
	Status            string     `json:"status"` // ASSIGNED, SUBMITTED, VERIFIED, CLOSED, OVERDUE
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// AIInspectionAnalysis represents a Gemini AI analysis of an inspection.
type AIInspectionAnalysis struct {
	ID                int       `json:"id"`
	InspectionID      int       `json:"inspection_id"`
	Category          string    `json:"category"`
	Severity          string    `json:"severity"`
	RiskLevel         string    `json:"risk_level"`
	RiskScore         int       `json:"risk_score"`
	Summary           string    `json:"summary"`
	Reasoning         string    `json:"reasoning"`
	RecommendedAction string    `json:"recommended_action"`
	RecurringIssue    bool      `json:"recurring_issue"`
	Urgency           string    `json:"urgency"`
	Confidence        float64   `json:"confidence"`
	ModelName         string    `json:"model_name"`
	CreatedAt         time.Time `json:"created_at"`
}

// Worker represents a mine worker or contract labour.
type Worker struct {
	ID             int       `json:"id"`
	MineID         int       `json:"mine_id"`
	MineName       string    `json:"mine_name,omitempty"`
	WorkerCode     string    `json:"worker_code"`
	FullName       string    `json:"full_name"`
	Designation    string    `json:"designation"`
	DepartmentID   *int      `json:"department_id"`
	DepartmentName string    `json:"department_name,omitempty"`
	ContractorID   *int      `json:"contractor_id"`
	ContractorName string    `json:"contractor_name,omitempty"`
	Status         string    `json:"status"` // ACTIVE, INACTIVE
	CreatedAt      time.Time `json:"created_at"`
}

// Attendance represents daily attendance for workers or mine totals.
type Attendance struct {
	ID            int64     `json:"id"`
	MineID        int       `json:"mine_id"`
	MineName      string    `json:"mine_name,omitempty"`
	WorkerID      *int      `json:"worker_id"`
	WorkerCode    string    `json:"worker_code,omitempty"`
	WorkerName    string    `json:"worker_name,omitempty"`
	Designation   string    `json:"designation,omitempty"`
	ContractorID  *int      `json:"contractor_id,omitempty"`
	ContractorName string   `json:"contractor_name,omitempty"`
	RecordDate    string    `json:"record_date"` // YYYY-MM-DD
	Status        string    `json:"status"`      // PRESENT, ABSENT, LEAVE, HALF_DAY
	Shift         string    `json:"shift"`       // GENERAL, SHIFT_1, SHIFT_2, SHIFT_3
	OvertimeHours float64   `json:"overtime_hours"`
	PresentCount  *int      `json:"present_count,omitempty"`
	TotalCount    *int      `json:"total_count,omitempty"`
	MarkedBy      *int      `json:"marked_by,omitempty"`
	MarkedByName  string    `json:"marked_by_name,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Grievance represents a worker grievance or complaint.
type Grievance struct {
	ID              int       `json:"id"`
	WorkerID        *int      `json:"worker_id"`
	WorkerName      string    `json:"worker_name,omitempty"`
	WorkerCode      string    `json:"worker_code,omitempty"`
	MineID          int       `json:"mine_id"`
	MineName        string    `json:"mine_name,omitempty"`
	Category        string    `json:"category"` // Safety, Compensation, Working Conditions, Harassment, Equipment, Other
	Description     string    `json:"description"`
	Status          string    `json:"status"` // SUBMITTED, IN_REVIEW, RESOLVED, ESCALATED, CLOSED
	AssignedTo      *int      `json:"assigned_to"`
	AssignedToName  string    `json:"assigned_to_name,omitempty"`
	ResolutionNotes string    `json:"resolution_notes,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Contractor represents a contracted service vendor.
type Contractor struct {
	ID              int       `json:"id"`
	MineID          int       `json:"mine_id"`
	MineName        string    `json:"mine_name,omitempty"`
	CompanyName     string    `json:"company_name"`
	ContactPerson   string    `json:"contact_person"`
	Phone           string    `json:"phone"`
	Email           string    `json:"email"`
	ContractType    string    `json:"contract_type"`
	ContractStart   string    `json:"contract_start"` // YYYY-MM-DD
	ContractEnd     string    `json:"contract_end"`   // YYYY-MM-DD
	Status          string    `json:"status"`         // ACTIVE, INACTIVE, BLACKLISTED
	BlacklistReason string    `json:"blacklist_reason,omitempty"`
	WorkerCount     int       `json:"worker_count,omitempty"`
	DocumentCount   int       `json:"document_count,omitempty"`
	DaysUntilExpiry int       `json:"days_until_expiry,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// EnvironmentalData represents mine environmental sensor data.
type EnvironmentalData struct {
	ID                int64     `json:"id"`
	MineID            int       `json:"mine_id"`
	MineName          string    `json:"mine_name,omitempty"`
	RecordDate        string    `json:"record_date"`
	AQI               float64   `json:"aqi"`
	WaterQualityIndex float64   `json:"water_quality_index"`
	NoiseLevelDB      float64   `json:"noise_level_db"`
	DustLevel         float64   `json:"dust_level"`
	CreatedAt         time.Time `json:"created_at"`
}

// OperationalData represents mine production and operational metrics.
type OperationalData struct {
	ID                 int64     `json:"id"`
	MineID             int       `json:"mine_id"`
	MineName           string    `json:"mine_name,omitempty"`
	RecordDate         string    `json:"record_date"`
	ProductionTonnes   float64   `json:"production_tonnes"`
	ExpectedProduction float64   `json:"expected_production"`
	VarianceTonnes     float64   `json:"variance_tonnes,omitempty"`
	VariancePct        float64   `json:"variance_pct,omitempty"`
	EquipmentHealthPct float64   `json:"equipment_health_pct"`
	AttendancePct      float64   `json:"attendance_pct"`
	CreatedAt          time.Time `json:"created_at"`
}

// AuditLog represents a tamper-evident audit record with cryptographic hash chain.
type AuditLog struct {
	ID         int64                  `json:"id"`
	UserID     *int                   `json:"user_id"`
	UserName   string                 `json:"user_name,omitempty"`
	Action     string                 `json:"action"`
	Module     string                 `json:"module"`
	RecordID   string                 `json:"record_id"`
	Details    map[string]interface{} `json:"details"`
	IPAddress  string                 `json:"ip_address"`
	PrevHash   string                 `json:"prev_hash"`
	Hash       string                 `json:"hash"`
	CreatedAt  time.Time              `json:"created_at"`
	IsTampered bool                   `json:"is_tampered,omitempty"`
}

