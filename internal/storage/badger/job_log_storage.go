package badger

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/timshannon/badgerhold/v4"
)

// logSequence is a global counter to ensure unique log keys even within the same nanosecond
var logSequence uint64

// JobLogStorage implements the JobLogStorage interface for Badger
type JobLogStorage struct {
	db     *BadgerDB
	logger arbor.ILogger
}

// NewJobLogStorage creates a new JobLogStorage instance
func NewJobLogStorage(db *BadgerDB, logger arbor.ILogger) interfaces.JobLogStorage {
	return &JobLogStorage{
		db:     db,
		logger: logger,
	}
}

func (s *JobLogStorage) AppendLog(ctx context.Context, jobID string, entry models.JobLogEntry) error {
	entry.AssociatedJobID = jobID
	// We don't have a unique ID for logs in the struct, so we might need to generate one or rely on BadgerHold's auto-increment if we used it.
	// But BadgerHold auto-increment is for uint64 keys.
	// We can use a composite key or just a timestamp-based key if we assume low concurrency per job.
	// Or we can use a sequence.
	// Let's use a composite key: jobID + timestamp + random/sequence?
	// Or just let BadgerHold generate a key if we don't need to query by ID directly (we query by JobID).
	// But we need to query by JobID efficiently.
	// BadgerHold allows querying by field.

	// We need a struct that includes the ID for BadgerHold if we want to use it.
	// But models.JobLogEntry doesn't have an ID field suitable for DB primary key.
	// We can wrap it or use a separate model.
	// For simplicity, let's assume we can just insert it and let BadgerHold manage the key if we use uint64?
	// But we want to query by JobID.

	// Let's define a wrapper struct for storage if needed, or just add an ID field to the model?
	// The model is shared, so maybe not.
	// Let's use a composite key string: "log:<jobID>:<timestamp>:<nanos>"
	// This allows range scans if needed, but BadgerHold queries by field index.

	// Generate unique key using timestamp + atomic sequence counter
	// This ensures uniqueness even when multiple logs are written within the same nanosecond
	seq := atomic.AddUint64(&logSequence, 1)
	key := fmt.Sprintf("%s_%d_%d", jobID, time.Now().UnixNano(), seq)

	if err := s.db.Store().Insert(key, &entry); err != nil {
		return fmt.Errorf("failed to append log: %w", err)
	}
	return nil
}

func (s *JobLogStorage) AppendLogs(ctx context.Context, jobID string, entries []models.JobLogEntry) error {
	for _, entry := range entries {
		if err := s.AppendLog(ctx, jobID, entry); err != nil {
			return err
		}
	}
	return nil
}

func (s *JobLogStorage) GetLogs(ctx context.Context, jobID string, limit int) ([]models.JobLogEntry, error) {
	var logs []models.JobLogEntry
	// Order by timestamp descending
	query := badgerhold.Where("AssociatedJobID").Eq(jobID).SortBy("FullTimestamp").Reverse()
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := s.db.Store().Find(&logs, query); err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}
	return logs, nil
}

func (s *JobLogStorage) GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.JobLogEntry, error) {
	var logs []models.JobLogEntry
	query := badgerhold.Where("AssociatedJobID").Eq(jobID).And("Level").Eq(level).SortBy("FullTimestamp").Reverse()
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := s.db.Store().Find(&logs, query); err != nil {
		return nil, fmt.Errorf("failed to get logs by level: %w", err)
	}
	return logs, nil
}

func (s *JobLogStorage) DeleteLogs(ctx context.Context, jobID string) error {
	if err := s.db.Store().DeleteMatching(&models.JobLogEntry{}, badgerhold.Where("AssociatedJobID").Eq(jobID)); err != nil {
		return fmt.Errorf("failed to delete logs: %w", err)
	}
	return nil
}

func (s *JobLogStorage) CountLogs(ctx context.Context, jobID string) (int, error) {
	count, err := s.db.Store().Count(&models.JobLogEntry{}, badgerhold.Where("AssociatedJobID").Eq(jobID))
	if err != nil {
		return 0, fmt.Errorf("failed to count logs: %w", err)
	}
	return int(count), nil
}

func (s *JobLogStorage) GetLogsWithOffset(ctx context.Context, jobID string, limit int, offset int) ([]models.JobLogEntry, error) {
	var logs []models.JobLogEntry
	query := badgerhold.Where("AssociatedJobID").Eq(jobID).SortBy("FullTimestamp").Reverse()
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Skip(offset)
	}

	if err := s.db.Store().Find(&logs, query); err != nil {
		return nil, fmt.Errorf("failed to get logs with offset: %w", err)
	}
	return logs, nil
}

func (s *JobLogStorage) GetLogsByLevelWithOffset(ctx context.Context, jobID string, level string, limit int, offset int) ([]models.JobLogEntry, error) {
	var logs []models.JobLogEntry
	query := badgerhold.Where("AssociatedJobID").Eq(jobID).And("Level").Eq(level).SortBy("FullTimestamp").Reverse()
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Skip(offset)
	}

	if err := s.db.Store().Find(&logs, query); err != nil {
		return nil, fmt.Errorf("failed to get logs by level with offset: %w", err)
	}
	return logs, nil
}
