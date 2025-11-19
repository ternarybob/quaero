package systemlogs

import "time"

// LogEntry represents a parsed log line from the system log file
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Raw       string    `json:"raw"` // Keep raw line just in case parsing fails or for display
}

// LogFile represents a log file on disk
type LogFile struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}
