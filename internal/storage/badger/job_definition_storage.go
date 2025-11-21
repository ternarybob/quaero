package badger

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/timshannon/badgerhold/v4"
)

// JobDefinitionStorage implements the JobDefinitionStorage interface for Badger
type JobDefinitionStorage struct {
	db     *BadgerDB
	logger arbor.ILogger
}

// NewJobDefinitionStorage creates a new JobDefinitionStorage instance
func NewJobDefinitionStorage(db *BadgerDB, logger arbor.ILogger) interfaces.JobDefinitionStorage {
	return &JobDefinitionStorage{
		db:     db,
		logger: logger,
	}
}

func (s *JobDefinitionStorage) SaveJobDefinition(ctx context.Context, jobDef *models.JobDefinition) error {
	if jobDef.ID == "" {
		return fmt.Errorf("job definition ID is required")
	}

	now := time.Now()
	if jobDef.CreatedAt.IsZero() {
		jobDef.CreatedAt = now
	}
	jobDef.UpdatedAt = now

	if err := s.db.Store().Upsert(jobDef.ID, jobDef); err != nil {
		return fmt.Errorf("failed to save job definition: %w", err)
	}
	return nil
}

func (s *JobDefinitionStorage) UpdateJobDefinition(ctx context.Context, jobDef *models.JobDefinition) error {
	return s.SaveJobDefinition(ctx, jobDef)
}

func (s *JobDefinitionStorage) GetJobDefinition(ctx context.Context, id string) (*models.JobDefinition, error) {
	var jobDef models.JobDefinition
	if err := s.db.Store().Get(id, &jobDef); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil, fmt.Errorf("job definition not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get job definition: %w", err)
	}
	return &jobDef, nil
}

func (s *JobDefinitionStorage) ListJobDefinitions(ctx context.Context, opts *interfaces.JobDefinitionListOptions) ([]*models.JobDefinition, error) {
	query := badgerhold.Where("ID").Ne("")

	if opts != nil {
		if opts.Type != "" {
			query = query.And("Type").Eq(opts.Type)
		}
		if opts.Enabled != nil {
			query = query.And("Enabled").Eq(*opts.Enabled)
		}
		if opts.Limit > 0 {
			query = query.Limit(opts.Limit)
		}
		if opts.Offset > 0 {
			query = query.Skip(opts.Offset)
		}
		// Sorting
		if opts.OrderBy != "" {
			if opts.OrderDir == "DESC" {
				query = query.SortBy(opts.OrderBy).Reverse()
			} else {
				query = query.SortBy(opts.OrderBy)
			}
		} else {
			// Default sort
			query = query.SortBy("CreatedAt").Reverse()
		}
	}

	var jobDefs []models.JobDefinition
	if err := s.db.Store().Find(&jobDefs, query); err != nil {
		return nil, fmt.Errorf("failed to list job definitions: %w", err)
	}

	result := make([]*models.JobDefinition, len(jobDefs))
	for i := range jobDefs {
		result[i] = &jobDefs[i]
	}
	return result, nil
}

func (s *JobDefinitionStorage) GetJobDefinitionsByType(ctx context.Context, jobType string) ([]*models.JobDefinition, error) {
	var jobDefs []models.JobDefinition
	if err := s.db.Store().Find(&jobDefs, badgerhold.Where("Type").Eq(jobType)); err != nil {
		return nil, fmt.Errorf("failed to get job definitions by type: %w", err)
	}

	result := make([]*models.JobDefinition, len(jobDefs))
	for i := range jobDefs {
		result[i] = &jobDefs[i]
	}
	return result, nil
}

func (s *JobDefinitionStorage) GetEnabledJobDefinitions(ctx context.Context) ([]*models.JobDefinition, error) {
	var jobDefs []models.JobDefinition
	if err := s.db.Store().Find(&jobDefs, badgerhold.Where("Enabled").Eq(true)); err != nil {
		return nil, fmt.Errorf("failed to get enabled job definitions: %w", err)
	}

	result := make([]*models.JobDefinition, len(jobDefs))
	for i := range jobDefs {
		result[i] = &jobDefs[i]
	}
	return result, nil
}

func (s *JobDefinitionStorage) DeleteJobDefinition(ctx context.Context, id string) error {
	if err := s.db.Store().Delete(id, &models.JobDefinition{}); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil
		}
		return fmt.Errorf("failed to delete job definition: %w", err)
	}
	return nil
}

func (s *JobDefinitionStorage) CountJobDefinitions(ctx context.Context) (int, error) {
	count, err := s.db.Store().Count(&models.JobDefinition{}, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count job definitions: %w", err)
	}
	return int(count), nil
}