package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// TestJobStorage_DeleteJob_SingleJob tests deleting a single job without children
func TestJobStorage_DeleteJob_SingleJob(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewJobStorage(db, logger)
	ctx := context.Background()

	job := &models.CrawlJob{
		JobType:    models.JobTypeParent,
		SourceType: "test",
		EntityType: "test",
		Status:     models.JobStatusCompleted,
		Config:     models.CrawlConfig{},
		Progress:   models.CrawlProgress{},
		CreatedAt:  time.Now().UTC(),
	}

	if err := storage.SaveJob(ctx, job); err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	t.Logf("Created job: %s", job.ID)

	// Verify job exists
	existingJob, err := storage.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("Job should exist before deletion: %v", err)
	}
	crawlJob, ok := existingJob.(*models.CrawlJob)
	if !ok {
		t.Fatal("Expected CrawlJob type")
	}
	if crawlJob.ID != job.ID {
		t.Errorf("Expected job ID %s, got: %s", job.ID, crawlJob.ID)
	}

	// Delete the job
	if err := storage.DeleteJob(ctx, job.ID); err != nil {
		t.Fatalf("Failed to delete job: %v", err)
	}

	t.Log("✓ Job deleted successfully")

	// Verify job no longer exists
	_, err = storage.GetJob(ctx, job.ID)
	if err == nil {
		t.Error("Job should not exist after deletion")
	}

	t.Log("✓ Job deletion verified")
}

// TestJobStorage_DeleteJob_NonExistent tests that deleting a non-existent job is idempotent
func TestJobStorage_DeleteJob_NonExistent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewJobStorage(db, logger)
	ctx := context.Background()

	nonExistentID := "job-does-not-exist-12345"

	// Should not error (idempotent)
	err := storage.DeleteJob(ctx, nonExistentID)
	if err != nil {
		t.Errorf("Deleting non-existent job should not error (idempotent), got: %v", err)
	}

	t.Log("✓ Non-existent job deletion is idempotent")
}

// TestJobStorage_DeleteJob_CascadeChildren tests that child jobs are cascade deleted
func TestJobStorage_DeleteJob_CascadeChildren(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewJobStorage(db, logger)
	ctx := context.Background()

	// Create parent job
	parent := &models.CrawlJob{
		JobType:    models.JobTypeParent,
		SourceType: "test",
		EntityType: "test",
		Status:     models.JobStatusCompleted,
		Config:     models.CrawlConfig{},
		Progress:   models.CrawlProgress{},
		CreatedAt:  time.Now().UTC(),
	}
	if err := storage.SaveJob(ctx, parent); err != nil {
		t.Fatalf("Failed to create parent job: %v", err)
	}
	t.Logf("Created parent job: %s", parent.ID)

	// Create 3 child jobs
	children := make([]*models.CrawlJob, 3)
	for i := 0; i < 3; i++ {
		child := &models.CrawlJob{
			JobType:    models.JobTypeCrawlerURL,
			ParentID:   parent.ID,
			SourceType: "test",
			EntityType: "test",
			Status:     models.JobStatusCompleted,
			Config:     models.CrawlConfig{},
			Progress:   models.CrawlProgress{},
			CreatedAt:  time.Now().UTC(),
		}
		if err := storage.SaveJob(ctx, child); err != nil {
			t.Fatalf("Failed to create child job %d: %v", i, err)
		}
		children[i] = child
		t.Logf("Created child job %d: %s", i, child.ID)
	}

	// Verify all children exist
	for _, child := range children {
		_, err := storage.GetJob(ctx, child.ID)
		if err != nil {
			t.Errorf("Child job %s should exist before parent deletion", child.ID)
		}
	}

	// Delete parent
	if err := storage.DeleteJob(ctx, parent.ID); err != nil {
		t.Fatalf("Failed to delete parent job: %v", err)
	}
	t.Log("✓ Parent job deleted")

	// Verify parent no longer exists
	_, err := storage.GetJob(ctx, parent.ID)
	if err == nil {
		t.Error("Parent job should not exist after deletion")
	}

	// Verify all children are cascade deleted
	for _, child := range children {
		_, err := storage.GetJob(ctx, child.ID)
		if err == nil {
			t.Errorf("Child job %s should be cascade deleted", child.ID)
		}
	}

	t.Log("✓ Cascade deletion of 3 children verified")
}

// TestJobStorage_DeleteJob_IdempotentDelete tests that deleting the same job multiple times is safe
func TestJobStorage_DeleteJob_IdempotentDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewJobStorage(db, logger)
	ctx := context.Background()

	job := &models.CrawlJob{
		JobType:    models.JobTypeParent,
		SourceType: "test",
		EntityType: "test",
		Status:     models.JobStatusCompleted,
		Config:     models.CrawlConfig{},
		Progress:   models.CrawlProgress{},
		CreatedAt:  time.Now().UTC(),
	}
	if err := storage.SaveJob(ctx, job); err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}
	t.Logf("Created job: %s", job.ID)

	// First deletion
	if err := storage.DeleteJob(ctx, job.ID); err != nil {
		t.Fatalf("First deletion failed: %v", err)
	}
	t.Log("✓ First deletion successful")

	// Second deletion (idempotent)
	if err := storage.DeleteJob(ctx, job.ID); err != nil {
		t.Errorf("Second deletion should be idempotent (no error), got: %v", err)
	}
	t.Log("✓ Second deletion idempotent")

	// Third deletion (idempotent)
	if err := storage.DeleteJob(ctx, job.ID); err != nil {
		t.Errorf("Third deletion should be idempotent (no error), got: %v", err)
	}
	t.Log("✓ Idempotent deletion behavior verified")
}
