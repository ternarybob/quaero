package orchestrator

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// ParentJobOrchestrator monitors parent job progress and aggregates child job statistics.
// It runs in background goroutines (not via queue) and publishes real-time progress events.
// Orchestrators subscribe to child job status changes for real-time tracking.
type ParentJobOrchestrator interface {
	// StartMonitoring begins monitoring a parent job in a background goroutine.
	// Takes the full job model (not just ID) to access config fields like source_type and entity_type.
	// Returns immediately after starting the monitoring goroutine.
	StartMonitoring(ctx context.Context, job *models.JobModel)

	// SubscribeToChildStatusChanges sets up event subscriptions for real-time child job tracking.
	// This is called during orchestrator initialization.
	SubscribeToChildStatusChanges()
}
