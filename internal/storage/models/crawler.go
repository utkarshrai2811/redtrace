package models

import "time"

// CrawlTask is a scope-bounded spider of a target, seeded at one URL.
type CrawlTask struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Seed      string    `json:"seed"`
	Scheme    string    `json:"scheme"`
	Host      string    `json:"host"`
	MaxDepth  int       `json:"maxDepth"`
	MaxPages  int       `json:"maxPages"`
	Status    string    `json:"status"`
	Pages     int       `json:"pages"`
	Found     int       `json:"found"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CrawlURL is one URL discovered by a crawl (the full exchange is also stored in
// the request history / site map).
type CrawlURL struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"taskId"`
	URL         string    `json:"url"`
	Method      string    `json:"method"`
	StatusCode  int       `json:"statusCode"`
	Length      int       `json:"length"`
	ContentType string    `json:"contentType"`
	Depth       int       `json:"depth"`
	CreatedAt   time.Time `json:"createdAt"`
}
