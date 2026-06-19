package models

import "time"

// Severity classifies the impact of a finding.
type Severity string

const (
	SeverityCritical      Severity = "critical"
	SeverityHigh          Severity = "high"
	SeverityMedium        Severity = "medium"
	SeverityLow           Severity = "low"
	SeverityInformational Severity = "informational"
)

// FindingStatus tracks the triage workflow state of a finding.
type FindingStatus string

const (
	FindingNew           FindingStatus = "new"
	FindingTriaged       FindingStatus = "triaged"
	FindingConfirmed     FindingStatus = "confirmed"
	FindingFalsePositive FindingStatus = "false_positive"
	FindingFixed         FindingStatus = "fixed"
)

// Finding is a vulnerability or issue surfaced by the scanner (Phase 4). It is
// defined now so the storage schema is stable from the start.
type Finding struct {
	ID          string        `json:"id"`
	RequestID   string        `json:"requestId,omitempty"`
	Name        string        `json:"name"`
	Severity    Severity      `json:"severity"`
	Confidence  string        `json:"confidence,omitempty"`
	Description string        `json:"description,omitempty"`
	Evidence    string        `json:"evidence,omitempty"`
	Remediation string        `json:"remediation,omitempty"`
	Status      FindingStatus `json:"status"`
	CreatedAt   time.Time     `json:"createdAt"`
}
