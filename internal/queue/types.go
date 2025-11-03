package queue

import "encoding/json"

// Message is the ONLY structure that goes into goqite.
// Keep it simple - just enough to route the job.
type Message struct {
	JobID   string          `json:"job_id"`  // References jobs.id
	Type    string          `json:"type"`    // Job type for executor routing
	Payload json.RawMessage `json:"payload"` // Job-specific data (passed through)
}
