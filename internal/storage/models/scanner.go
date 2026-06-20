package models

import "time"

// ScanTask is an active (request-fuzzing) scan: a request template that is
// probed for vulnerabilities at its insertion points.
type ScanTask struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Scheme          string    `json:"scheme"`
	Host            string    `json:"host"`
	Template        []byte    `json:"-"`
	HTTPVersion     string    `json:"httpVersion"`
	FollowRedirects bool      `json:"followRedirects"`
	Status          string    `json:"status"`
	Total           int       `json:"total"`
	Completed       int       `json:"completed"`
	Issues          int       `json:"issues"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// ScanIssue is a single finding from the passive or active scanner.
type ScanIssue struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"taskId,omitempty"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	Severity    string    `json:"severity"`
	Confidence  string    `json:"confidence"`
	Scheme      string    `json:"scheme"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	Param       string    `json:"param,omitempty"`
	Payload     string    `json:"payload,omitempty"`
	Detail      string    `json:"detail"`
	Evidence    string    `json:"evidence"`
	Remediation string    `json:"remediation"`
	Origin      string    `json:"origin"` // "passive" | "active"
	Fingerprint string    `json:"-"`
	RequestRaw  []byte    `json:"-"`
	ResponseRaw []byte    `json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
}
