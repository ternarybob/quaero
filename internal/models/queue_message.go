package models

import (
	"encoding/json"
	"errors"
)

// ErrNoMessage is returned when the queue is empty
var ErrNoMessage = errors.New("no messages in queue")

// QueueMessage is the structure stored in the queue.
// Keep it simple - just enough to route the job.
type QueueMessage struct {
	JobID   string          `json:"job_id"`  // References jobs.id
	Type    string          `json:"type"`    // Job type for executor routing
	Payload json.RawMessage `json:"payload"` // Job-specific data (passed through)
}
