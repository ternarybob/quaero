package interfaces

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
}
