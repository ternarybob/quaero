// -----------------------------------------------------------------------
// Last Modified: Monday, 20th October 2025 5:30:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// JobType represents the type of job
type JobType string

// JobType constants
const (
	JobTypeCrawler    JobType = "crawler"
	JobTypeSummarizer JobType = "summarizer"
	JobTypeCustom     JobType = "custom"
)

// IsValidJobType checks if a given JobType is one of the valid constants
func IsValidJobType(jobType JobType) bool {
	switch jobType {
	case JobTypeCrawler, JobTypeSummarizer, JobTypeCustom:
		return true
	default:
		return false
	}
}

// ErrorStrategy represents how to handle errors during job execution
type ErrorStrategy string

// ErrorStrategy constants
const (
	ErrorStrategyContinue ErrorStrategy = "continue" // Log error and continue to next step
	ErrorStrategyFail     ErrorStrategy = "fail"     // Stop job execution immediately
	ErrorStrategyRetry    ErrorStrategy = "retry"    // Retry step with exponential backoff
)

// JobStep represents a single execution step in a job definition
type JobStep struct {
	Name      string                 `json:"name"`                // Step identifier/name
	Action    string                 `json:"action"`              // Action type (e.g., "crawl", "transform", "embed", "scan", "summarize")
	Config    map[string]interface{} `json:"config"`              // Step-specific configuration parameters
	OnError   ErrorStrategy          `json:"on_error"`            // Error handling strategy
	Condition string                 `json:"condition,omitempty"` // Optional conditional execution expression (for future use)
}

// Config keys for "crawl" action:
//   - start_urls ([]string): Initial URLs to begin crawling. Required if source doesn't provide seed URLs.
//   - include_patterns ([]string): Regex patterns for URLs to include. If empty, all discovered links are included (subject to exclude patterns).
//   - exclude_patterns ([]string): Regex patterns for URLs to exclude. Applied before include patterns.
//   - max_depth (int): Maximum crawl depth. Overrides source default if provided.
//   - max_pages (int): Maximum pages to crawl. Overrides source default if provided.
//   - concurrency (int): Number of concurrent workers. Overrides source default if provided.
//   - follow_links (bool): Whether to follow discovered links. Overrides source default if provided.
//   - refresh_source (bool): Whether to refresh source config and auth before crawling. Default: true.
//   - wait_for_completion (bool): Whether to block until crawl completes. Default: true.
//
// Example:
//   {
//     "name": "crawl",
//     "action": "crawl",
//     "config": {
//       "start_urls": ["https://company.atlassian.net/browse"],
//       "include_patterns": ["/browse/[A-Z]+-[0-9]+", "/projects/"],
//       "exclude_patterns": ["/admin", "/logout"],
//       "max_depth": 3,
//       "follow_links": true
//     },
//     "on_error": "continue"
//   }

// JobDefinition represents a configurable job definition
type JobDefinition struct {
	ID          string                 `json:"id"`          // Unique identifier for the job definition
	Name        string                 `json:"name"`        // Human-readable job name
	Type        JobType                `json:"type"`        // Type of job (crawler, summarizer, custom)
	Description string                 `json:"description"` // Job description
	Sources     []string               `json:"sources"`     // Array of source IDs this job operates on
	Steps       []JobStep              `json:"steps"`       // Ordered array of execution steps
	Schedule    string                 `json:"schedule"`    // Cron expression for scheduling
	Timeout     string                 `json:"timeout"`     // Optional: duration string like "10m", "1h", "30s". Empty means no timeout.
	Enabled     bool                   `json:"enabled"`     // Whether the job is enabled
	AutoStart   bool                   `json:"auto_start"`  // Whether to auto-start on scheduler initialization
	Config      map[string]interface{} `json:"config"`      // Job-specific configuration
	CreatedAt   time.Time              `json:"created_at"`  // Creation timestamp
	UpdatedAt   time.Time              `json:"updated_at"`  // Last update timestamp
}

// Validate validates the job definition
// Note: Schedule is optional. When empty, the job can only be triggered manually.
func (j *JobDefinition) Validate() error {
	// Validate required fields
	if j.ID == "" {
		return errors.New("job definition ID is required")
	}
	if j.Name == "" {
		return errors.New("job definition name is required")
	}
	if j.Type == "" {
		return errors.New("job definition type is required")
	}

	// Validate JobType is one of the allowed constants
	if !IsValidJobType(j.Type) {
		return fmt.Errorf("invalid job type: %s (must be one of: crawler, summarizer, custom)", j.Type)
	}

	// Validate Steps array is not empty
	if len(j.Steps) == 0 {
		return errors.New("job definition must have at least one step")
	}

	// Validate each step
	for i, step := range j.Steps {
		if err := j.ValidateStep(&step); err != nil {
			return fmt.Errorf("step %d validation failed: %w", i, err)
		}
	}

	// Validate cron schedule format (only if provided)
	if j.Schedule != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(j.Schedule); err != nil {
			return fmt.Errorf("invalid cron schedule '%s': %w", j.Schedule, err)
		}
	}

	// Validate timeout duration format (only if provided)
	if j.Timeout != "" {
		if _, err := time.ParseDuration(j.Timeout); err != nil {
			return fmt.Errorf("invalid timeout duration '%s': %w", j.Timeout, err)
		}
	}

	return nil
}

// ValidateStep validates a single job step
func (j *JobDefinition) ValidateStep(step *JobStep) error {
	if step.Name == "" {
		return errors.New("step name is required")
	}
	if step.Action == "" {
		return errors.New("step action is required")
	}

	// Validate error strategy if provided
	if step.OnError != "" {
		switch step.OnError {
		case ErrorStrategyContinue, ErrorStrategyFail, ErrorStrategyRetry:
			// Valid strategy
		default:
			return fmt.Errorf("invalid error strategy: %s (must be one of: continue, fail, retry)", step.OnError)
		}
	}

	return nil
}

// MarshalSteps serializes the steps array to JSON string for database storage
func (j *JobDefinition) MarshalSteps() (string, error) {
	data, err := json.Marshal(j.Steps)
	if err != nil {
		return "", fmt.Errorf("failed to marshal steps: %w", err)
	}
	return string(data), nil
}

// UnmarshalSteps deserializes the steps JSON string from database
func (j *JobDefinition) UnmarshalSteps(data string) error {
	if err := json.Unmarshal([]byte(data), &j.Steps); err != nil {
		return fmt.Errorf("failed to unmarshal steps: %w", err)
	}
	return nil
}

// MarshalSources serializes the sources array to JSON string for database storage
func (j *JobDefinition) MarshalSources() (string, error) {
	data, err := json.Marshal(j.Sources)
	if err != nil {
		return "", fmt.Errorf("failed to marshal sources: %w", err)
	}
	return string(data), nil
}

// UnmarshalSources deserializes the sources JSON string from database
func (j *JobDefinition) UnmarshalSources(data string) error {
	if err := json.Unmarshal([]byte(data), &j.Sources); err != nil {
		return fmt.Errorf("failed to unmarshal sources: %w", err)
	}
	return nil
}

// MarshalConfig serializes the config map to JSON string for database storage
func (j *JobDefinition) MarshalConfig() (string, error) {
	if j.Config == nil {
		return "{}", nil
	}
	data, err := json.Marshal(j.Config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}
	return string(data), nil
}

// UnmarshalConfig deserializes the config JSON string from database
func (j *JobDefinition) UnmarshalConfig(data string) error {
	if data == "" || data == "{}" {
		j.Config = make(map[string]interface{})
		return nil
	}
	if err := json.Unmarshal([]byte(data), &j.Config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return nil
}
