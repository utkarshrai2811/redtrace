package models

import "time"

// OOBPayload is a generated out-of-band payload: a unique token forming a
// hostname under the configured OOB domain that an operator plants in a target.
type OOBPayload struct {
	Token     string    `json:"token"`
	Host      string    `json:"host"`
	CreatedAt time.Time `json:"createdAt"`
}

// OOBInteraction is a captured out-of-band callback (a DNS query or HTTP request
// that reached the OOB listeners), correlated to a payload token when possible.
type OOBInteraction struct {
	ID        string    `json:"id"`
	Token     string    `json:"token,omitempty"`
	Protocol  string    `json:"protocol"` // "dns" | "http"
	SourceIP  string    `json:"sourceIp"`
	Query     string    `json:"query"` // queried name (DNS) or Host (HTTP)
	Detail    string    `json:"detail"`
	Raw       []byte    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}
