package badger

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/timshannon/badgerhold/v4"
)

func TestJobStatusPersistence(t *testing.T) {
	// Setup temporary directory for BadgerDB
	tmpDir, err := ioutil.TempDir("", "badger-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize BadgerDB
	options := badgerhold.DefaultOptions
	options.Dir = tmpDir
	options.ValueDir = tmpDir

	store, err := badgerhold.Open(options)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Create BadgerDB wrapper
	db := &BadgerDB{store: store}
	logger := arbor.NewLogger()
	storage := NewQueueStorage(db, logger)

	ctx := context.Background()

	// 1. Create a parent job
	parentJob := &models.QueueJobState{
		ID:        "parent-1",
		Type:      "parent",
		Status:    models.JobStatusRunning,
		CreatedAt: time.Now(),
	}
	if err := storage.SaveJob(ctx, parentJob); err != nil {
		t.Fatalf("Failed to save parent job: %v", err)
	}

	// 2. Create child jobs
	child1 := &models.QueueJobState{
		ID:        "child-1",
		ParentID:  &parentJob.ID,
		Type:      "child",
		Status:    models.JobStatusPending,
		CreatedAt: time.Now(),
	}
	if err := storage.SaveJob(ctx, child1); err != nil {
		t.Fatalf("Failed to save child 1: %v", err)
	}

	child2 := &models.QueueJobState{
		ID:        "child-2",
		ParentID:  &parentJob.ID,
		Type:      "child",
		Status:    models.JobStatusPending,
		CreatedAt: time.Now(),
	}
	if err := storage.SaveJob(ctx, child2); err != nil {
		t.Fatalf("Failed to save child 2: %v", err)
	}

	// 3. Verify initial stats (should be 2 pending)
	t.Log("Verifying initial stats...")
	if storage == nil {
		t.Fatal("Storage is nil")
	}

	// Sanity check: can we find the parent job directly?
	var sanityCheck models.QueueJob
	if err := store.Get(parentJob.ID, &sanityCheck); err != nil {
		t.Fatalf("Sanity check failed: %v", err)
	}
	t.Logf("Sanity check passed, found job: %s", sanityCheck.ID)

	stats, err := storage.GetJobChildStats(ctx, []string{parentJob.ID})
	if err != nil {
		t.Fatalf("Failed to get child stats: %v", err)
	}
	if stats[parentJob.ID].PendingChildren != 2 {
		t.Errorf("Expected 2 pending children, got %d", stats[parentJob.ID].PendingChildren)
	}

	// 4. Update child 1 status to Running
	if err := storage.UpdateJobStatus(ctx, child1.ID, "running", ""); err != nil {
		t.Fatalf("Failed to update child 1 status: %v", err)
	}

	// 5. Verify stats (1 pending, 1 running)
	stats, err = storage.GetJobChildStats(ctx, []string{parentJob.ID})
	if err != nil {
		t.Fatalf("Failed to get child stats: %v", err)
	}
	if stats[parentJob.ID].PendingChildren != 1 {
		t.Errorf("Expected 1 pending child, got %d", stats[parentJob.ID].PendingChildren)
	}
	if stats[parentJob.ID].RunningChildren != 1 {
		t.Errorf("Expected 1 running child, got %d", stats[parentJob.ID].RunningChildren)
	}

	// 6. Update child 2 status to Completed
	if err := storage.UpdateJobStatus(ctx, child2.ID, "completed", ""); err != nil {
		t.Fatalf("Failed to update child 2 status: %v", err)
	}

	// 7. Verify stats (0 pending, 1 running, 1 completed)
	stats, err = storage.GetJobChildStats(ctx, []string{parentJob.ID})
	if err != nil {
		t.Fatalf("Failed to get child stats: %v", err)
	}
	if stats[parentJob.ID].PendingChildren != 0 {
		t.Errorf("Expected 0 pending children, got %d", stats[parentJob.ID].PendingChildren)
	}
	if stats[parentJob.ID].RunningChildren != 1 {
		t.Errorf("Expected 1 running child, got %d", stats[parentJob.ID].RunningChildren)
	}
	if stats[parentJob.ID].CompletedChildren != 1 {
		t.Errorf("Expected 1 completed child, got %d", stats[parentJob.ID].CompletedChildren)
	}

	// 8. Verify GetJob returns correct status
	job, err := storage.GetJob(ctx, child1.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	jobState := job.(*models.QueueJobState)
	if jobState.Status != models.JobStatusRunning {
		t.Errorf("Expected child 1 status Running, got %s", jobState.Status)
	}
}
