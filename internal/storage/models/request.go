// Package models defines the persisted entities for RedTrace storage.
package models

import "time"

// Source identifies which RedTrace tool produced a captured request.
type Source string

const (
	SourceProxy    Source = "proxy"
	SourceRepeater Source = "repeater"
	SourceIntruder Source = "intruder"
	SourceCrawler  Source = "crawler"
	SourceScanner  Source = "scanner"
)

// Request is a captured HTTP request together with the metadata RedTrace indexes
// for filtering and display. Raw holds the exact bytes as observed on the wire
// and is excluded from default JSON encoding (the API exposes it explicitly).
type Request struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Source      Source    `json:"source"`
	Method      string    `json:"method"`
	Scheme      string    `json:"scheme"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Path        string    `json:"path"`
	Query       string    `json:"query,omitempty"`
	URL         string    `json:"url"`
	HTTPVersion string    `json:"httpVersion"`
	ContentType string    `json:"contentType,omitempty"`
	BodySize    int       `json:"bodySize"`
	InScope     bool      `json:"inScope"`
	Raw         []byte    `json:"-"`
}
