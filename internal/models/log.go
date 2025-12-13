package models

// LogEntry represents a single log entry with extensible context.
// Used for all persistent logging: job logs, step logs, worker logs, system logs.
//
// All metadata is stored in the Context map for consistency and flexibility.
// Badgerhold indexes on Context keys enable efficient queries.
//
// Common Context Keys (indexed via badgerhold):
//   - job_id: Job that generated this log
//   - manager_id: Root manager job ID
//   - step_id: Step job ID
//   - parent_id: Direct parent job ID
//   - originator: "manager", "step", "worker", "system"
//   - phase: "init", "run", "orchestrator"
//   - source_type: Worker type
//   - step_name: Step name
//
// Timestamp Format:
//   - Timestamp: "15:04:05.000" (HH:MM:SS.mmm) for display
//   - FullTimestamp: RFC3339Nano for accurate sorting
//
// Log Levels: "debug", "info", "warn", "error"
type LogEntry struct {
	// Core fields
	Timestamp     string `json:"timestamp"`                // HH:MM:SS.mmm format for display
	FullTimestamp string `json:"full_timestamp"`           // RFC3339Nano for sorting
	Level         string `json:"level" badgerhold:"index"` // Log level (indexed)
	Message       string `json:"message"`                  // Log message

	// LineNumber is a per-job monotonically increasing counter (1-based)
	// This provides stable, contiguous line numbers for each job's logs
	// Used for UI display and pagination
	LineNumber int `json:"line_number" badgerhold:"index"`

	// Sequence is a global counter for stable ordering when timestamps are identical
	// Format: UnixNano timestamp + sequence counter (e.g., "1702393191123456789_0000000001")
	// This ensures logs are ordered correctly even when written in rapid succession
	// Note: LineNumber is preferred for per-job ordering; Sequence is for cross-job aggregation
	Sequence string `json:"sequence" badgerhold:"index"` // Composite sort key for stable ordering

	// JobIDField is the primary query field - stored separately for efficient badgerhold indexing
	// (badgerhold cannot query into map fields with dot notation)
	// Access via JobID() method for consistency with other getters
	JobIDField string `json:"job_id" badgerhold:"index"`

	// Context stores additional metadata as key-value pairs
	// Standard keys: manager_id, step_id, parent_id, originator, phase, source_type, step_name
	Context map[string]string `json:"context,omitempty"`
}

// Context key constants for consistent access
const (
	LogCtxJobID      = "job_id"
	LogCtxManagerID  = "manager_id"
	LogCtxStepID     = "step_id"
	LogCtxParentID   = "parent_id"
	LogCtxOriginator = "originator"
	LogCtxPhase      = "phase"
	LogCtxSourceType = "source_type"
	LogCtxStepName   = "step_name"
)

// GetContext safely retrieves a context value
func (e *LogEntry) GetContext(key string) string {
	if e.Context == nil {
		return ""
	}
	return e.Context[key]
}

// SetContext safely sets a context value (initializes map if needed)
func (e *LogEntry) SetContext(key, value string) {
	if e.Context == nil {
		e.Context = make(map[string]string)
	}
	if value != "" {
		e.Context[key] = value
	}
}

// Convenience getters for common fields
// JobID returns the job ID from the dedicated indexed field
func (e *LogEntry) JobID() string      { return e.JobIDField }
func (e *LogEntry) ManagerID() string  { return e.GetContext(LogCtxManagerID) }
func (e *LogEntry) StepID() string     { return e.GetContext(LogCtxStepID) }
func (e *LogEntry) ParentID() string   { return e.GetContext(LogCtxParentID) }
func (e *LogEntry) Originator() string { return e.GetContext(LogCtxOriginator) }
func (e *LogEntry) Phase() string      { return e.GetContext(LogCtxPhase) }
func (e *LogEntry) SourceType() string { return e.GetContext(LogCtxSourceType) }
func (e *LogEntry) StepName() string   { return e.GetContext(LogCtxStepName) }
