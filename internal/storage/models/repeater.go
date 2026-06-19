package models

import "time"

// RepeaterTab is a saved, editable request in the Repeater tool, persisted
// across sessions.
type RepeaterTab struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Scheme          string    `json:"scheme"`
	Host            string    `json:"host"`
	Raw             []byte    `json:"-"`
	FollowRedirects bool      `json:"followRedirects"`
	HTTPVersion     string    `json:"httpVersion"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// RepeaterHistoryEntry records one send of a Repeater tab.
type RepeaterHistoryEntry struct {
	ID          string    `json:"id"`
	TabID       string    `json:"tabId"`
	RequestRaw  []byte    `json:"-"`
	ResponseRaw []byte    `json:"-"`
	StatusCode  int       `json:"statusCode"`
	DurationMs  int64     `json:"durationMs"`
	CreatedAt   time.Time `json:"createdAt"`
}
