package models

import "time"

// Mine represents a coal mine site under a subsidiary.
type Mine struct {
	ID                  int       `json:"id"`
	MineName            string    `json:"mine_name"`
	MineCode            string    `json:"mine_code"`
	SubsidiaryID        int       `json:"subsidiary_id"`
	SubsidiaryName      string    `json:"subsidiary_name,omitempty"`
	State               string    `json:"state"`
	District            string    `json:"district"`
	Latitude            float64   `json:"latitude"`
	Longitude           float64   `json:"longitude"`
	MineType            string    `json:"mine_type"`
	ProductionCapacity  float64   `json:"production_capacity"`
	ManagerID           *int      `json:"manager_id"`
	ManagerName         string    `json:"manager_name,omitempty"`
	Status              string    `json:"status"`
	ComplianceScore     *float64  `json:"compliance_score,omitempty"`
	RiskScore           *float64  `json:"risk_score,omitempty"`
	RiskClassification  string    `json:"risk_classification,omitempty"`
	OpenViolationsCount int       `json:"open_violations_count,omitempty"`
	OverdueActionsCount int       `json:"overdue_actions_count,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Subsidiary represents a Coal India subsidiary company.
type Subsidiary struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Code         string    `json:"code"`
	Headquarters string    `json:"headquarters"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
