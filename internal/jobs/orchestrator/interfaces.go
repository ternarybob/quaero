package orchestrator

import (
	"context"
)

// ParentJobOrchestrator monitors parent job progress and aggregates child job statistics.
// It runs in background goroutines (not via queue) and publishes real-time progress events.
type ParentJobOrchestrator interface {
	// StartMonitoring begins monitoring a parent job in a background goroutine.
	// Polls child job statistics periodically and publishes progress events.
	// Automatically stops when all child jobs complete or parent job is cancelled.
	StartMonitoring(ctx context.Context, parentJobID string) error

	// StopMonitoring stops monitoring a specific parent job.
	// Used for cleanup or when parent job is cancelled.
	StopMonitoring(parentJobID string) error

	// GetMonitoringStatus returns whether a parent job is currently being monitored.
	GetMonitoringStatus(parentJobID string) bool
}
