// -----------------------------------------------------------------------
// Last Modified: Monday, 20th October 2025 5:30:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package models

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

func init() {
	// Register types for gob encoding (required for BadgerHold storage of interface{} fields)
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
	gob.Register([]map[string]interface{}{}) // Used for step_definitions in job metadata
	gob.Register(map[string]float64{})       // Used by keyword extractor agent for keyword scores
	gob.Register(map[string]string{})        // Used in document metadata (open_graph, meta, etc.)
	gob.Register([]string{})                 // Used by agents for keyword lists
	gob.Register(time.Time{})                // Used in document timestamps and job metadata
	gob.Register(CrawlConfig{})              // Used in crawler job configs
	gob.Register(DevOpsMetadata{})           // Used for C/C++ DevOps enrichment
}

// JobDefinitionType represents the type of job definition
type JobDefinitionType string

// JobDefinitionType constants
const (
	JobDefinitionTypeCrawler     JobDefinitionType = "crawler"
	JobDefinitionTypeSummarizer  JobDefinitionType = "summarizer"
	JobDefinitionTypeCustom      JobDefinitionType = "custom"
	JobDefinitionTypePlaces      JobDefinitionType = "places"
	JobDefinitionTypeAgent       JobDefinitionType = "agent"        // Agent-powered document processing jobs
	JobDefinitionTypeFetch       JobDefinitionType = "fetch"        // API-based data collection with authentication (GitHub, etc.)
	JobDefinitionTypeWebSearch   JobDefinitionType = "web_search"   // Gemini-powered web search with grounding
	JobDefinitionTypeLocalDir    JobDefinitionType = "local_dir"    // Local filesystem directory indexing
	JobDefinitionTypeCodeMap     JobDefinitionType = "code_map"     // Hierarchical code structure analysis
	JobDefinitionTypeJobTemplate JobDefinitionType = "job_template" // Template orchestration - executes job templates with variable substitution
)

// JobOwnerType represents whether a job is system-managed or user-created
type JobOwnerType string

// JobOwnerType constants
const (
	JobOwnerTypeSystem JobOwnerType = "system" // System-managed jobs (readonly, cannot be edited/deleted)
	JobOwnerTypeUser   JobOwnerType = "user"   // User-created jobs (can be edited/deleted)
)

// IsValidJobDefinitionType checks if a given JobDefinitionType is one of the valid constants
func IsValidJobDefinitionType(jobType JobDefinitionType) bool {
	switch jobType {
	case JobDefinitionTypeCrawler, JobDefinitionTypeSummarizer, JobDefinitionTypeCustom, JobDefinitionTypePlaces,
		JobDefinitionTypeAgent, JobDefinitionTypeFetch, JobDefinitionTypeWebSearch,
		JobDefinitionTypeLocalDir, JobDefinitionTypeCodeMap, JobDefinitionTypeJobTemplate:
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
//
// Step Config Keys (common across all step types):
//   - filter_tags ([]string): Document filter by tags. Only process documents that have ALL specified tags.
//     If not provided, the step is executed against all documents. This enables multi-step pipelines
//     where each step processes different document subsets.
//   - filter_created_after (string): RFC3339 timestamp. Only process documents created after this time.
//   - filter_updated_after (string): RFC3339 timestamp. Only process documents updated after this time.
//   - filter_limit (int): Maximum number of documents to process. Useful for testing or batching.
//
// AI Step Config Keys (for agent steps only):
//   - model_selection (string): LLM model selection strategy. Options:
//   - "auto": Automatically select based on task complexity (default)
//   - "fast": Use fast model for simple tasks
//   - "thinking": Use thinking model for complex reasoning
//   - "default": Use the standard agent_model
//   - validation (bool): Whether to validate AI output by re-prompting the LLM. Default: true
//   - validation_iteration_count (int): Number of validation iterations to perform. Default: 1
//     Higher values provide more thorough validation at the cost of additional API calls.
//
// Example TOML:
//
//	[step.extract_keywords]
//	type = "agent"
//	agent_type = "keyword_extractor"
//	filter_tags = ["technical", "needs-processing"]
//	filter_limit = 100
//	model_selection = "auto"
//	validation = true
//	validation_iteration_count = 1
type JobStep struct {
	Name        string                 `json:"name"`                  // Step identifier/name
	Type        WorkerType             `json:"type"`                  // Worker type for routing to appropriate worker (required)
	Description string                 `json:"description,omitempty"` // Human-readable description of what this step does
	Config      map[string]interface{} `json:"config"`                // Step-specific configuration parameters (flat structure)
	OnError     ErrorStrategy          `json:"on_error"`              // Error handling strategy
	Depends     string                 `json:"depends,omitempty"`     // Comma-separated list of step names this step depends on
	Condition   string                 `json:"condition,omitempty"`   // Optional conditional execution expression (for future use)
}

// ErrorTolerance defines failure threshold management for parent jobs
// Used to automatically stop jobs when child failure rate exceeds acceptable limits
type ErrorTolerance struct {
	MaxChildFailures int    `json:"max_child_failures"` // Maximum number of child job failures before stopping parent job (0 = unlimited)
	FailureAction    string `json:"failure_action"`     // Action to take when threshold exceeded: "stop_all", "continue", "mark_warning"
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
//     "type": "crawler",
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
	ID               string                 `json:"id"`                     // Unique identifier for the job definition
	Name             string                 `json:"name"`                   // Human-readable job name
	Type             JobDefinitionType      `json:"type"`                   // Type of job definition (crawler, summarizer, custom) - derived from steps if not specified
	JobType          JobOwnerType           `json:"job_type"`               // Job owner type (system or user)
	Description      string                 `json:"description"`            // Job description
	TOML             string                 `json:"toml" db:"toml"`         // Raw TOML content from which this job definition was loaded (optional)
	SourceType       string                 `json:"source_type"`            // Source type: "jira", "confluence", "github"
	BaseURL          string                 `json:"base_url"`               // Base URL for the source (e.g., "https://company.atlassian.net")
	AuthID           string                 `json:"auth_id"`                // Reference to auth_credentials.id for authentication
	Steps            []JobStep              `json:"steps"`                  // Ordered array of execution steps
	Schedule         string                 `json:"schedule"`               // Cron expression for scheduling
	Timeout          string                 `json:"timeout"`                // Optional: duration string like "10m", "1h", "30s". Empty means no timeout.
	Enabled          bool                   `json:"enabled"`                // Whether the job is enabled
	AutoStart        bool                   `json:"auto_start"`             // Whether to auto-start on scheduler initialization
	Extension        bool                   `json:"extension"`              // When true, this job can be matched by Chrome extension via url_patterns
	Config           map[string]interface{} `json:"config"`                 // Job-specific configuration
	PreJobs          []string               `json:"pre_jobs"`               // Array of job definition IDs to execute before main steps (validation, pre-checks)
	PostJobs         []string               `json:"post_jobs"`              // Array of job IDs to execute after this job completes
	ErrorTolerance   *ErrorTolerance        `json:"error_tolerance"`        // Optional error tolerance configuration for child job failure management
	Tags             []string               `json:"tags"`                   // Tags to apply to all documents created by this job
	UrlPatterns      []string               `json:"url_patterns"`           // URL patterns for automatic job matching (wildcards: *.domain.com/*)
	ValidationStatus string                 `json:"validation_status"`      // TOML validation status: "valid", "invalid", "unknown"
	ValidationError  string                 `json:"validation_error"`       // TOML validation error message (if invalid)
	ValidatedAt      *time.Time             `json:"validated_at"`           // Timestamp of last validation (nil if never validated)
	ContentHash      string                 `json:"content_hash,omitempty"` // MD5 hash (8-char hex) of TOML content for change detection
	Updated          bool                   `json:"updated"`                // True if TOML content changed since last load (computed, not persisted)
	RuntimeStatus    string                 `json:"runtime_status"`         // Runtime validation status: "ready", "disabled", "unknown" (not persisted to DB)
	RuntimeError     string                 `json:"runtime_error"`          // Runtime validation error message (e.g., missing API key) (not persisted to DB)
	CreatedAt        time.Time              `json:"created_at"`             // Creation timestamp
	UpdatedAt        time.Time              `json:"updated_at"`             // Last update timestamp
}

// isPlaceholder checks if a string value contains placeholder syntax {key-name}
// Used to skip validation for values that will be replaced at runtime
func isPlaceholder(value string) bool {
	return len(value) > 2 && value[0] == '{' && value[len(value)-1] == '}'
}

// Validate validates the job definition
// Note: Schedule is optional. When empty, the job can only be triggered manually.
// Note: Type is optional. When empty, it can be derived from the first step's worker type.
func (j *JobDefinition) Validate() error {
	// Validate required fields
	if j.ID == "" {
		return errors.New("job definition ID is required")
	}
	if j.Name == "" {
		return errors.New("job definition name is required")
	}

	// Type can be derived from steps, so only validate if explicitly set
	if j.Type != "" && !IsValidJobDefinitionType(j.Type) {
		return fmt.Errorf("invalid job definition type: %s (must be one of: crawler, summarizer, custom, places, agent, fetch, web_search, local_dir, code_map, job_template)", j.Type)
	}

	// Validate JobOwnerType (default to 'user' if empty)
	if j.JobType == "" {
		j.JobType = JobOwnerTypeUser
	}
	if j.JobType != JobOwnerTypeSystem && j.JobType != JobOwnerTypeUser {
		return fmt.Errorf("invalid job_type: %s (must be one of: system, user)", j.JobType)
	}

	// Validate source fields for crawler jobs
	// Note: source_type is optional for file-based crawler definitions that don't require source integration
	if j.Type == JobDefinitionTypeCrawler {
		// Validate source type only if provided and not a placeholder
		if j.SourceType != "" && !isPlaceholder(j.SourceType) {
			validTypes := map[string]bool{
				"jira":       true,
				"confluence": true,
				"github":     true,
				"web":        true, // Generic web crawler for arbitrary websites
			}
			if !validTypes[j.SourceType] {
				return fmt.Errorf("invalid source_type: %s (must be one of: jira, confluence, github, web)", j.SourceType)
			}
		}
		// Base URL is optional (only required if source_type is set)
		// Note: Removed strict base_url requirement to support generic crawler jobs
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

	// Validate PreJobs array
	for _, preJobID := range j.PreJobs {
		if preJobID == "" {
			return errors.New("pre_job ID cannot be empty string")
		}
		if preJobID == j.ID {
			return errors.New("pre_jobs cannot contain the job's own ID (circular dependency)")
		}
	}

	// Validate ErrorTolerance if provided
	if j.ErrorTolerance != nil {
		if j.ErrorTolerance.MaxChildFailures < 0 {
			return errors.New("error_tolerance.max_child_failures must be >= 0")
		}
		// Skip validation for placeholder values
		if !isPlaceholder(j.ErrorTolerance.FailureAction) {
			switch j.ErrorTolerance.FailureAction {
			case "stop_all", "continue", "mark_warning":
				// Valid failure action
			default:
				return fmt.Errorf("invalid error_tolerance.failure_action: %s (must be one of: stop_all, continue, mark_warning)", j.ErrorTolerance.FailureAction)
			}
		}
	}

	return nil
}

// ValidateStep validates a single job step
func (j *JobDefinition) ValidateStep(step *JobStep) error {
	if step.Name == "" {
		return errors.New("step name is required")
	}

	// Type field is required - this is the primary routing mechanism
	if step.Type == "" {
		return errors.New("worker type is required")
	}

	// Validate that Type is a known WorkerType
	if !step.Type.IsValid() {
		return fmt.Errorf("invalid worker type: %s (must be one of: agent, crawler, places_search, web_search, github_repo, github_actions, github_git, transform, reindex, local_dir, code_map, summary)", step.Type)
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

	// Validate depends field - check referenced steps exist
	if step.Depends != "" {
		dependsList := splitAndTrim(step.Depends)
		stepNames := make(map[string]bool)
		for _, s := range j.Steps {
			stepNames[s.Name] = true
		}
		for _, dep := range dependsList {
			if dep == step.Name {
				return fmt.Errorf("step '%s' cannot depend on itself", step.Name)
			}
			if !stepNames[dep] {
				return fmt.Errorf("step '%s' depends on unknown step '%s'", step.Name, dep)
			}
		}
	}

	// Agent-specific validation for agent job types
	if j.Type == JobDefinitionTypeAgent {
		if step.Config == nil {
			return errors.New("agent job steps must have config")
		}

		// Validate operation_type if provided (optional but recommended)
		if operationType, ok := step.Config["operation_type"].(string); ok && operationType != "" {
			// Skip validation for placeholder values
			if !isPlaceholder(operationType) {
				validOperations := map[string]bool{
					"scan":     true, // Add metadata/tags/keywords to existing documents
					"enrich":   true, // Add web-sourced information to documents
					"generate": true, // Create new documents from existing ones
				}
				if !validOperations[operationType] {
					return fmt.Errorf("invalid operation_type for agent job: %s (must be one of: scan, enrich, generate)", operationType)
				}
			}
		}

		// Validate agent_type is provided (required for agent jobs)
		if agentType, ok := step.Config["agent_type"].(string); ok {
			if agentType == "" && !isPlaceholder(agentType) {
				return errors.New("agent_type cannot be empty for agent job steps")
			}
		} else {
			return errors.New("agent_type is required in step config for agent job steps")
		}

		// Validate flat filter fields (new format: filter_* instead of document_filter.*)
		// Validate filter_tags if provided
		if tags, ok := step.Config["filter_tags"].([]interface{}); ok {
			for i, tag := range tags {
				if _, ok := tag.(string); !ok {
					return fmt.Errorf("filter_tags[%d] must be a string", i)
				}
			}
		}

		// Validate date filters format if provided
		if createdAfter, ok := step.Config["filter_created_after"].(string); ok && createdAfter != "" {
			if _, err := time.Parse(time.RFC3339, createdAfter); err != nil {
				return fmt.Errorf("filter_created_after must be in RFC3339 format: %w", err)
			}
		}
		if updatedAfter, ok := step.Config["filter_updated_after"].(string); ok && updatedAfter != "" {
			if _, err := time.Parse(time.RFC3339, updatedAfter); err != nil {
				return fmt.Errorf("filter_updated_after must be in RFC3339 format: %w", err)
			}
		}

		// Validate filter_limit if provided
		if limit, ok := step.Config["filter_limit"].(float64); ok {
			if limit < 1 {
				return errors.New("filter_limit must be >= 1")
			}
		} else if limit, ok := step.Config["filter_limit"].(int); ok {
			if limit < 1 {
				return errors.New("filter_limit must be >= 1")
			}
		} else if limit, ok := step.Config["filter_limit"].(int64); ok {
			if limit < 1 {
				return errors.New("filter_limit must be >= 1")
			}
		}
	}

	return nil
}

// splitAndTrim splits a comma-separated string and trims whitespace from each element
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
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

// MarshalPostJobs serializes the post_jobs array to JSON string for database storage
func (j *JobDefinition) MarshalPostJobs() (string, error) {
	data, err := json.Marshal(j.PostJobs)
	if err != nil {
		return "", fmt.Errorf("failed to marshal post_jobs: %w", err)
	}
	return string(data), nil
}

// UnmarshalPostJobs deserializes the post_jobs JSON string from database
func (j *JobDefinition) UnmarshalPostJobs(data string) error {
	if err := json.Unmarshal([]byte(data), &j.PostJobs); err != nil {
		return fmt.Errorf("failed to unmarshal post_jobs: %w", err)
	}
	return nil
}

// MarshalPreJobs serializes the pre_jobs array to JSON string for database storage
func (j *JobDefinition) MarshalPreJobs() (string, error) {
	data, err := json.Marshal(j.PreJobs)
	if err != nil {
		return "", fmt.Errorf("failed to marshal pre_jobs: %w", err)
	}
	return string(data), nil
}

// UnmarshalPreJobs deserializes the pre_jobs JSON string from database
func (j *JobDefinition) UnmarshalPreJobs(data string) error {
	if err := json.Unmarshal([]byte(data), &j.PreJobs); err != nil {
		return fmt.Errorf("failed to unmarshal pre_jobs: %w", err)
	}
	return nil
}

// MarshalErrorTolerance serializes the error_tolerance to JSON string for database storage
func (j *JobDefinition) MarshalErrorTolerance() (string, error) {
	if j.ErrorTolerance == nil {
		return "{}", nil
	}
	data, err := json.Marshal(j.ErrorTolerance)
	if err != nil {
		return "", fmt.Errorf("failed to marshal error_tolerance: %w", err)
	}
	return string(data), nil
}

// UnmarshalErrorTolerance deserializes the error_tolerance JSON string from database
func (j *JobDefinition) UnmarshalErrorTolerance(data string) error {
	if data == "" || data == "{}" {
		j.ErrorTolerance = nil
		return nil
	}
	var et ErrorTolerance
	if err := json.Unmarshal([]byte(data), &et); err != nil {
		return fmt.Errorf("failed to unmarshal error_tolerance: %w", err)
	}
	j.ErrorTolerance = &et
	return nil
}

// MarshalTags serializes the tags array to JSON string for database storage
func (j *JobDefinition) MarshalTags() (string, error) {
	if j.Tags == nil || len(j.Tags) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(j.Tags)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tags: %w", err)
	}
	return string(data), nil
}

// UnmarshalTags deserializes the tags JSON string from database
func (j *JobDefinition) UnmarshalTags(data string) error {
	if data == "" || data == "[]" {
		j.Tags = []string{}
		return nil
	}
	if err := json.Unmarshal([]byte(data), &j.Tags); err != nil {
		return fmt.Errorf("failed to unmarshal tags: %w", err)
	}
	return nil
}

// IsSystemJob returns true if the job is a system-managed job (readonly)
func (j *JobDefinition) IsSystemJob() bool {
	return j.JobType == JobOwnerTypeSystem
}

// IsUserJob returns true if the job is a user-created job (editable)
func (j *JobDefinition) IsUserJob() bool {
	return j.JobType == JobOwnerTypeUser || j.JobType == ""
}
