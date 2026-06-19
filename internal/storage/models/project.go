package models

import "time"

// Project groups captured traffic and findings for a single engagement.
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Target    string    `json:"target,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Session stores an exported/shareable snapshot of project state (Phase 8).
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	Data      []byte    `json:"-"`
}
