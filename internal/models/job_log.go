package models

// JobLogEntry represents a single log entry for a crawler job
type JobLogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}
