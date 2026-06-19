package models

import "time"

// Response is a captured HTTP response linked to a Request. Raw holds the exact
// bytes as observed and is excluded from default JSON encoding.
type Response struct {
	ID          string    `json:"id"`
	RequestID   string    `json:"requestId"`
	Timestamp   time.Time `json:"timestamp"`
	StatusCode  int       `json:"statusCode"`
	Reason      string    `json:"reason,omitempty"`
	HTTPVersion string    `json:"httpVersion"`
	MimeType    string    `json:"mimeType,omitempty"`
	BodySize    int       `json:"bodySize"`
	DurationMs  int64     `json:"durationMs"`
	Raw         []byte    `json:"-"`
}

// Exchange pairs a request with its response (which may be nil if the upstream
// never replied).
type Exchange struct {
	Request  *Request  `json:"request"`
	Response *Response `json:"response,omitempty"`
}
