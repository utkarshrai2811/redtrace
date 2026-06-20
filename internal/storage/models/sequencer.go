package models

import "time"

// SequencerTask collects a sample of tokens (by replaying a request) and
// analyzes their randomness.
type SequencerTask struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Scheme      string    `json:"scheme"`
	Host        string    `json:"host"`
	Template    []byte    `json:"-"`
	HTTPVersion string    `json:"httpVersion"`
	Source      string    `json:"source"`   // "cookie" | "regex"
	Selector    string    `json:"selector"` // cookie name or regex pattern
	Target      int       `json:"target"`
	Status      string    `json:"status"`
	Collected   int       `json:"collected"`
	Tokens      []byte    `json:"-"` // JSON array of sampled token strings (capped)
	Report      []byte    `json:"-"` // JSON sequencer.Report
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
