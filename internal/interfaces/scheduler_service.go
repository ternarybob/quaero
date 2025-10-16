package interfaces

import "time"

// JobStatus represents the current status of a scheduled job
type JobStatus struct {
	Name        string
	Enabled     bool
	Schedule    string
	Description string
	LastRun     *time.Time
	NextRun     *time.Time
	IsRunning   bool
	LastError   string
}

// SchedulerService manages cron-based scheduling
type SchedulerService interface {
	// Start the scheduler with a cron expression
	Start(cronExpr string) error

	// Stop the scheduler
	Stop() error

	// TriggerCollectionNow manually triggers collection
	TriggerCollectionNow() error

	// IsRunning returns true if scheduler is active
	IsRunning() bool

	// RegisterJob registers a new job with the scheduler
	RegisterJob(name string, schedule string, handler func() error) error

	// EnableJob enables a disabled job
	EnableJob(name string) error

	// DisableJob disables an enabled job
	DisableJob(name string) error

	// GetJobStatus returns the status of a specific job
	GetJobStatus(name string) (*JobStatus, error)

	// GetAllJobStatuses returns all job statuses
	GetAllJobStatuses() map[string]*JobStatus
}
