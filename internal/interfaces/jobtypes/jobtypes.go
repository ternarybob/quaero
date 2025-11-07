package jobtypes

// JobStatusReport provides a standardized view of job status across all job types.
// This struct encapsulates status calculation logic and provides consistent reporting
// for both parent and child jobs. It can be embedded in API responses or used standalone.
type JobStatusReport struct {
	Status            string   `json:"status"`             // Current job status (pending, running, completed, failed, cancelled)
	ChildCount        int      `json:"child_count"`        // Total number of child jobs spawned
	CompletedChildren int      `json:"completed_children"` // Number of completed child jobs
	FailedChildren    int      `json:"failed_children"`    // Number of failed child jobs
	CancelledChildren int      `json:"cancelled_children"` // Number of cancelled child jobs
	RunningChildren   int      `json:"running_children"`   // Number of running child jobs
	PendingChildren   int      `json:"pending_children"`   // Number of pending child jobs
	ProgressText      string   `json:"progress_text"`      // Human-readable progress description (e.g., "105 pending, 2 running, 2 completed (Total: 109)")
	Errors            []string `json:"errors"`             // List of error messages from the job (extracted from job.Error field if present)
	Warnings          []string `json:"warnings"`           // List of warning messages (reserved for future use, initially empty)
}

// JobChildStats holds aggregate statistics for a parent job's children
type JobChildStats struct {
	ChildCount        int
	CompletedChildren int
	FailedChildren    int
	CancelledChildren int
	PendingChildren   int
	RunningChildren   int
}
