package models

import "time"

// TimingRecord stores timing and performance data for a completed worker job.
// Used for monitoring worker performance and debugging slow jobs.
type TimingRecord struct {
	ID          string           `json:"id"`
	JobID       string           `json:"job_id"`
	WorkerType  string           `json:"worker_type"`
	Ticker      string           `json:"ticker,omitempty"`
	StartedAt   time.Time        `json:"started_at"`
	CompletedAt time.Time        `json:"completed_at"`
	TotalMs     int64            `json:"total_ms"`
	Phases      map[string]int64 `json:"phases,omitempty"`
	Status      string           `json:"status"` // "success" or "failed"
	Error       string           `json:"error,omitempty"`
	APICallsMs  int64            `json:"api_calls_ms,omitempty"`
	AITokensIn  int              `json:"ai_tokens_in,omitempty"`
	AITokensOut int              `json:"ai_tokens_out,omitempty"`
}
