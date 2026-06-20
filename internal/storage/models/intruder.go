package models

import "time"

// IntruderAttack is a saved fuzzing attack: a request template with marked
// positions, its attack type, and the payload configuration (stored as JSON).
type IntruderAttack struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Scheme     string    `json:"scheme"`
	Host       string    `json:"host"`
	Template   []byte    `json:"-"`
	AttackType string    `json:"attackType"`
	Config     []byte    `json:"-"` // JSON: payload sets, processors, options
	Status     string    `json:"status"`
	Total      int       `json:"total"`
	Completed  int       `json:"completed"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// IntruderResult is the outcome of one generated request in an attack.
type IntruderResult struct {
	ID          string    `json:"id"`
	AttackID    string    `json:"attackId"`
	Index       int       `json:"index"`
	Payloads    []byte    `json:"-"` // JSON array of payload strings
	StatusCode  int       `json:"statusCode"`
	Length      int       `json:"length"`
	DurationMs  int64     `json:"durationMs"`
	RequestRaw  []byte    `json:"-"`
	ResponseRaw []byte    `json:"-"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}
